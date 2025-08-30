/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

// Basic MCP tools: add, list, get, update, delete, mark-done

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addTaskHandler creates a new task
func addTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.AddTaskParams, types.TaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.AddTaskParams]) (*mcp.CallToolResultFor[types.TaskResponse], error) {
		args := params.Arguments
		logToolCall("add-task", args)

		// Validate required fields
		if strings.TrimSpace(args.Title) == "" {
			return nil, types.NewMCPError("MISSING_TITLE", "Task title is required", map[string]interface{}{
				"field": "title",
			})
		}

		// Normalize and validate priority input
		normalizedPriority := strings.TrimSpace(args.Priority)
		if normalizedPriority != "" {
			if canon, nerr := normalizePriorityString(normalizedPriority); nerr == nil {
				normalizedPriority = canon
			} else {
				return nil, types.NewMCPError("INVALID_PRIORITY", nerr.Error(), map[string]interface{}{
					"value":        args.Priority,
					"valid_values": []string{"low", "medium", "high", "urgent"},
				})
			}
		}

		// Validate input (use normalized priority)
		if err := ValidateTaskInput(args.Title, normalizedPriority, ""); err != nil {
			return nil, err
		}

		// Set priority with default
		priority := models.PriorityMedium
		if normalizedPriority != "" {
			switch normalizedPriority {
			case "low":
				priority = models.PriorityLow
			case "medium":
				priority = models.PriorityMedium
			case "high":
				priority = models.PriorityHigh
			case "urgent":
				priority = models.PriorityUrgent
			}
		}

		// Set parent ID if provided
		var parentID *string
		if strings.TrimSpace(args.ParentID) != "" {
			// Validate that parent task exists
			_, err := taskStore.GetTask(args.ParentID)
			if err != nil {
				return nil, types.NewMCPError("PARENT_NOT_FOUND", fmt.Sprintf("Parent task %s not found", args.ParentID), map[string]interface{}{
					"parent_id": args.ParentID,
				})
			}
			parentID = &args.ParentID
		}

		// Create new task
		task := models.Task{
			ID:                 uuid.New().String(),
			Title:              strings.TrimSpace(args.Title),
			Description:        strings.TrimSpace(args.Description),
			AcceptanceCriteria: strings.TrimSpace(args.AcceptanceCriteria),
			Status:             models.StatusTodo,
			Priority:           priority,
			ParentID:           parentID,
			SubtaskIDs:         []string{},
			Dependencies:       args.Dependencies,
			Dependents:         []string{},
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		// Validate task
		if err := models.ValidateStruct(task); err != nil {
			return nil, types.NewMCPError("VALIDATION_FAILED", fmt.Sprintf("Task validation failed: %s", err.Error()), nil)
		}

		// Create task in store
		createdTask, err := taskStore.CreateTask(task)
		if err != nil {
			return nil, WrapStoreError(err, "create", task.ID)
		}

		// If this is a subtask, update parent's SubtaskIDs
		if parentID != nil {
			parentTask, err := taskStore.GetTask(*parentID)
			if err != nil {
				// Log error but don't fail the creation since task was already created
				logError(fmt.Errorf("failed to get parent task %s for subtask update: %w", *parentID, err))
			} else {
				// Add this task to parent's subtasks
				updatedSubtasks := append(parentTask.SubtaskIDs, createdTask.ID)
				_, err = taskStore.UpdateTask(*parentID, map[string]interface{}{
					"subtaskIds": updatedSubtasks,
				})
				if err != nil {
					logError(fmt.Errorf("failed to update parent task %s with new subtask: %w", *parentID, err))
				}
			}
		}

		logInfo(fmt.Sprintf("Created task: %s", createdTask.ID))

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := fmt.Sprintf("Created task '%s' with ID: %s", createdTask.Title, createdTask.ID)
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcp.CallToolResultFor[types.TaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: taskToResponse(createdTask),
			IsError:           false,
		}, nil
	}
}

// listTasksHandler lists tasks with optional filtering
func listTasksHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.ListTasksParams, types.TaskListResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ListTasksParams]) (*mcp.CallToolResultFor[types.TaskListResponse], error) {
		args := params.Arguments

		// Create filter function
		filterFn := func(task models.Task) bool {
			// Filter by status
			if args.Status != "" {
				if string(task.Status) != args.Status {
					return false
				}
			}

			// Filter by priority
			if args.Priority != "" {
				if string(task.Priority) != args.Priority {
					return false
				}
			}

			// Filter by parent ID
			if args.ParentID != "" {
				if task.ParentID == nil || *task.ParentID != args.ParentID {
					return false
				}
			}

			// Search in title and description
			if args.Search != "" {
				search := strings.ToLower(args.Search)
				title := strings.ToLower(task.Title)
				description := strings.ToLower(task.Description)
				if !strings.Contains(title, search) && !strings.Contains(description, search) {
					return false
				}
			}

			return true
		}

		// Create sort function
		var sortFn func([]models.Task) []models.Task
		if args.SortBy != "" {
			sortFn = createSortFunction(args.SortBy, args.SortOrder)
		}

		// List tasks
		tasks, err := taskStore.ListTasks(filterFn, sortFn)
		if err != nil {
			return nil, WrapStoreError(err, "list", "multiple")
		}

		// Convert to response format
		taskResponses := make([]types.TaskResponse, len(tasks))
		for i, task := range tasks {
			taskResponses[i] = taskToResponse(task)
		}

		response := types.TaskListResponse{
			Tasks: taskResponses,
			Count: len(taskResponses),
		}

		logInfo(fmt.Sprintf("Listed %d tasks", len(tasks)))

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := fmt.Sprintf("Found %d tasks", len(tasks))
		// Append a compact list of top tasks with short IDs for quick selection
		maxShow := 5
		if len(tasks) > 0 {
			if len(tasks) < maxShow {
				maxShow = len(tasks)
			}
			responseText += "\nTop:"
			for i := 0; i < maxShow; i++ {
				responseText += fmt.Sprintf("\n - %s [%s]", tasks[i].Title, shortID(tasks[i].ID))
			}
		}
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcp.CallToolResultFor[types.TaskListResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: response,
		}, nil
	}
}

// updateTaskHandler updates an existing task
func updateTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.UpdateTaskParams, types.TaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.UpdateTaskParams]) (*mcp.CallToolResultFor[types.TaskResponse], error) {
		args := params.Arguments

		// Validate required fields (allow reference)
		if strings.TrimSpace(args.ID) == "" && strings.TrimSpace(args.Reference) == "" {
			return nil, types.NewMCPError("MISSING_ID", "Task ID or reference is required for update", nil)
		}

		// Build updates map
		updates := make(map[string]interface{})

		if strings.TrimSpace(args.Title) != "" {
			updates["title"] = strings.TrimSpace(args.Title)
		}

		if args.Description != "" {
			updates["description"] = strings.TrimSpace(args.Description)
		}

		if args.AcceptanceCriteria != "" {
			updates["acceptanceCriteria"] = strings.TrimSpace(args.AcceptanceCriteria)
		}

		if args.Status != "" {
			if err := ValidateTaskInput("", "", args.Status); err != nil {
				return nil, err
			}
			updates["status"] = models.TaskStatus(args.Status)
		}

		if args.Priority != "" {
			// Normalize and validate priority
			canon := args.Priority
			if np, nerr := normalizePriorityString(canon); nerr == nil {
				canon = np
			} else {
				return nil, types.NewMCPError("INVALID_PRIORITY", nerr.Error(), map[string]interface{}{
					"value":        args.Priority,
					"valid_values": []string{"low", "medium", "high", "urgent"},
				})
			}

			if err := ValidateTaskInput("", canon, ""); err != nil {
				return nil, err
			}
			updates["priority"] = models.TaskPriority(canon)
		}

		if args.Dependencies != nil {
			updates["dependencies"] = args.Dependencies
		}

		if args.ParentID != "" {
			// Validate that parent task exists
			_, err := taskStore.GetTask(args.ParentID)
			if err != nil {
				return nil, types.NewMCPError("PARENT_NOT_FOUND", fmt.Sprintf("Parent task %s not found", args.ParentID), map[string]interface{}{
					"parent_id": args.ParentID,
				})
			}
			updates["parentId"] = &args.ParentID
		}

		// Update task (try direct; then resolve reference if needed)
		id := strings.TrimSpace(args.ID)
		if id == "" {
			id = strings.TrimSpace(args.Reference)
		}

		updatedTask, err := taskStore.UpdateTask(id, updates)
		if err != nil {
			tasks, lerr := taskStore.ListTasks(nil, nil)
			if lerr == nil {
				if resolvedID, candidates, ok := resolveReference(id, tasks); ok {
					if retryTask, retryErr := taskStore.UpdateTask(resolvedID, updates); retryErr == nil {
						err = nil // Retry succeeded, clear the original error
						updatedTask = retryTask
					}
					// If retry also failed, keep the original error for reporting
				} else {
					details := map[string]interface{}{"reference": id}
					if len(candidates) > 0 {
						max := len(candidates)
						if max > 5 {
							max = 5
						}
						details["candidates"] = candidates[:max]
					}
					details["next_step"] = "Use 'resolve-task-reference' to obtain a concrete ID."
					return nil, types.NewMCPError("TASK_NOT_FOUND", "Task reference could not be resolved for update", details)
				}
			}
		}

		// Check if error persists after retry attempt
		if err != nil {
			return nil, WrapStoreError(err, "update", id)
		}

		logInfo(fmt.Sprintf("Updated task: %s", updatedTask.ID))

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := fmt.Sprintf("Updated task '%s' (ID: %s)", updatedTask.Title, updatedTask.ID)
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcp.CallToolResultFor[types.TaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: taskToResponse(updatedTask),
		}, nil
	}
}

// deleteTaskHandler deletes a task
func deleteTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.DeleteTaskParams, types.DeleteTaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.DeleteTaskParams]) (*mcp.CallToolResultFor[types.DeleteTaskResponse], error) {
		args := params.Arguments

		// Validate required fields (allow reference)
		if strings.TrimSpace(args.ID) == "" && strings.TrimSpace(args.Reference) == "" {
			return nil, types.NewMCPError("MISSING_ID", "Task ID or reference is required for deletion", nil)
		}

		// Resolve to concrete ID if necessary
		id := strings.TrimSpace(args.ID)
		if id == "" {
			id = strings.TrimSpace(args.Reference)
		}
		task, err := taskStore.GetTask(id)
		if err != nil {
			tasks, lerr := taskStore.ListTasks(nil, nil)
			if lerr == nil {
				if resolvedID, candidates, ok := resolveReference(id, tasks); ok {
					if retryTask, retryErr := taskStore.GetTask(resolvedID); retryErr == nil {
						err = nil // Retry succeeded, clear the original error
						task = retryTask
					}
					// If retry also failed, keep the original error for reporting
				} else {
					details := map[string]interface{}{"reference": id}
					if len(candidates) > 0 {
						max := len(candidates)
						if max > 5 {
							max = 5
						}
						details["candidates"] = candidates[:max]
					}
					details["next_step"] = "Use 'resolve-task-reference' to obtain a concrete ID."
					return nil, types.NewMCPError("TASK_NOT_FOUND", "Task reference could not be resolved for deletion", details)
				}
			}
		}

		// Check if error persists after retry attempt
		if err != nil {
			return nil, WrapStoreError(err, "get", id)
		}

		// Check if task has dependents
		if len(task.Dependents) > 0 {
			return nil, types.NewMCPError("HAS_DEPENDENTS", "Cannot delete task with dependent tasks", map[string]interface{}{
				"task_id":    args.ID,
				"dependents": task.Dependents,
			})
		}

		// Delete task
		if err := taskStore.DeleteTask(id); err != nil {
			return nil, WrapStoreError(err, "delete", id)
		}

		logInfo(fmt.Sprintf("Deleted task: %s", id))

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := fmt.Sprintf("Deleted task '%s' (ID: %s)", task.Title, task.ID)
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		response := types.DeleteTaskResponse{
			Success: true,
			TaskID:  id,
			Message: fmt.Sprintf("Task '%s' deleted successfully", task.Title),
		}

		return &mcp.CallToolResultFor[types.DeleteTaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: response,
		}, nil
	}
}

// markDoneHandler marks a task as completed
func markDoneHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.MarkDoneParams, types.TaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.MarkDoneParams]) (*mcp.CallToolResultFor[types.TaskResponse], error) {
		args := params.Arguments

		// Validate required fields (allow reference fallback)
		if strings.TrimSpace(args.ID) == "" {
			return nil, types.NewMCPError("MISSING_ID", "Task ID (or reference) is required to mark as done", map[string]interface{}{
				"tip": "Pass a full/partial ID or title; the server will resolve references.",
			})
		}

		// Try direct mark; if not found, attempt reference resolution
		completedTask, err := taskStore.MarkTaskDone(args.ID)
		if err != nil {
			// Load tasks and try to resolve reference
			tasks, lerr := taskStore.ListTasks(nil, nil)
			if lerr == nil {
				if resolvedID, candidates, ok := resolveReference(args.ID, tasks); ok {
					completedTask, err = taskStore.MarkTaskDone(resolvedID)
				} else {
					// Provide helpful candidates
					details := map[string]interface{}{
						"reference": args.ID,
					}
					if len(candidates) > 0 {
						// Include up to 5 candidates
						max := len(candidates)
						if max > 5 {
							max = 5
						}
						details["candidates"] = candidates[:max]
					}
					details["next_step"] = "Use 'find-task' or 'resolve-task-reference' to obtain a concrete ID."
					return nil, types.NewMCPError("TASK_NOT_FOUND", "Task reference could not be resolved", details)
				}
			}
		}
		if err != nil {
			return nil, WrapStoreError(err, "mark_done", args.ID)
		}

		logInfo(fmt.Sprintf("Marked task as done: %s", completedTask.ID))

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := fmt.Sprintf("Marked task '%s' as completed (ID: %s)", completedTask.Title, completedTask.ID)
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcp.CallToolResultFor[types.TaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: taskToResponse(completedTask),
		}, nil
	}
}

// getTaskHandler retrieves a specific task
func getTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.GetTaskParams, types.TaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.GetTaskParams]) (*mcp.CallToolResultFor[types.TaskResponse], error) {
		args := params.Arguments

		// Validate required fields (allow reference)
		if strings.TrimSpace(args.ID) == "" && strings.TrimSpace(args.Reference) == "" {
			return nil, types.NewMCPError("MISSING_ID", "Task ID or reference is required to get a task", nil)
		}

		id := strings.TrimSpace(args.ID)
		if id == "" {
			id = strings.TrimSpace(args.Reference)
		}

		// Get task (with resolution fallback)
		task, err := taskStore.GetTask(id)
		if err != nil {
			tasks, lerr := taskStore.ListTasks(nil, nil)
			if lerr == nil {
				if resolvedID, candidates, ok := resolveReference(id, tasks); ok {
					if retryTask, retryErr := taskStore.GetTask(resolvedID); retryErr == nil {
						err = nil // Retry succeeded, clear the original error
						task = retryTask
					}
					// If retry also failed, keep the original error for reporting
				} else {
					details := map[string]interface{}{"reference": id}
					if len(candidates) > 0 {
						max := len(candidates)
						if max > 5 {
							max = 5
						}
						details["candidates"] = candidates[:max]
					}
					details["next_step"] = "Use 'resolve-task-reference' to obtain a concrete ID."
					return nil, types.NewMCPError("TASK_NOT_FOUND", "Task reference could not be resolved", details)
				}
			}
		}

		// Check if error persists after retry attempt
		if err != nil {
			return nil, WrapStoreError(err, "get", id)
		}

		logInfo(fmt.Sprintf("Retrieved task: %s", task.ID))

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := fmt.Sprintf("Task '%s' (ID: %s)", task.Title, task.ID)
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcp.CallToolResultFor[types.TaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: taskToResponse(task),
		}, nil
	}
}

// Helper function to create sort function
func createSortFunction(sortBy, sortOrder string) func([]models.Task) []models.Task {
	return func(tasks []models.Task) []models.Task {
		sort.SliceStable(tasks, func(i, j int) bool {
			t1 := tasks[i]
			t2 := tasks[j]
			var less bool
			switch strings.ToLower(sortBy) {
			case "id":
				less = t1.ID < t2.ID
			case "title":
				less = strings.ToLower(t1.Title) < strings.ToLower(t2.Title)
			case "status":
				less = statusToInt(t1.Status) < statusToInt(t2.Status)
			case "priority":
				less = priorityToInt(t1.Priority) < priorityToInt(t2.Priority)
			case "createdat":
				less = t1.CreatedAt.Before(t2.CreatedAt)
			case "updatedat":
				less = t1.UpdatedAt.Before(t2.UpdatedAt)
			default:
				// Default to createdAt if sort field is unknown
				less = t1.CreatedAt.Before(t2.CreatedAt)
			}
			if strings.ToLower(sortOrder) == "desc" {
				return !less
			}
			return less
		})
		return tasks
	}
}

// setCurrentTaskHandler sets the current active task
func setCurrentTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.SetCurrentTaskParams, types.CurrentTaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.SetCurrentTaskParams]) (*mcp.CallToolResultFor[types.CurrentTaskResponse], error) {
		args := params.Arguments
		logToolCall("set-current-task", args)

		// Validate required fields
		if strings.TrimSpace(args.ID) == "" {
			return &mcp.CallToolResultFor[types.CurrentTaskResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: "Task ID is required",
					},
				},
				StructuredContent: types.CurrentTaskResponse{
					Success: false,
					Message: "Task ID is required",
				},
				IsError: true,
			}, nil
		}

		// Verify the task exists
		task, err := taskStore.GetTask(args.ID)
		if err != nil {
			return &mcp.CallToolResultFor[types.CurrentTaskResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Task '%s' not found", args.ID),
					},
				},
				StructuredContent: types.CurrentTaskResponse{
					Success: false,
					Message: fmt.Sprintf("Task '%s' not found", args.ID),
				},
				IsError: true,
			}, nil
		}

		// Set the current task
		if err := SetCurrentTask(args.ID); err != nil {
			return &mcp.CallToolResultFor[types.CurrentTaskResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Failed to set current task: %v", err),
					},
				},
				StructuredContent: types.CurrentTaskResponse{
					Success: false,
					Message: fmt.Sprintf("Failed to set current task: %v", err),
				},
				IsError: true,
			}, nil
		}

		logInfo(fmt.Sprintf("Set current task: %s - %s", task.ID, task.Title))

		response := types.CurrentTaskResponse{
			CurrentTask: taskToResponsePtr(task),
			Success:     true,
			Message:     fmt.Sprintf("Set current task: %s - %s", task.ID, task.Title),
		}

		return &mcp.CallToolResultFor[types.CurrentTaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: response.Message,
				},
			},
			StructuredContent: response,
		}, nil
	}
}

// getCurrentTaskHandler gets the current active task
func getCurrentTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.GetCurrentTaskParams, types.CurrentTaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.GetCurrentTaskParams]) (*mcp.CallToolResultFor[types.CurrentTaskResponse], error) {
		logToolCall("get-current-task", params.Arguments)

		currentTaskID := GetCurrentTask()

		if currentTaskID == "" {
			response := types.CurrentTaskResponse{
				Success: true,
				Message: "No current task set",
			}

			return &mcp.CallToolResultFor[types.CurrentTaskResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: "No current task set",
					},
				},
				StructuredContent: response,
			}, nil
		}

		// Get the current task
		task, err := taskStore.GetTask(currentTaskID)
		if err != nil {
			response := types.CurrentTaskResponse{
				Success: false,
				Message: fmt.Sprintf("Current task '%s' not found (may have been deleted)", currentTaskID),
			}

			return &mcp.CallToolResultFor[types.CurrentTaskResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: response.Message,
					},
				},
				StructuredContent: response,
				IsError:           true,
			}, nil
		}

		logInfo(fmt.Sprintf("Retrieved current task: %s - %s", task.ID, task.Title))

		response := types.CurrentTaskResponse{
			CurrentTask: taskToResponsePtr(task),
			Success:     true,
			Message:     fmt.Sprintf("Current task: %s - %s", task.ID, task.Title),
		}

		return &mcp.CallToolResultFor[types.CurrentTaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: response.Message,
				},
			},
			StructuredContent: response,
		}, nil
	}
}

// clearCurrentTaskHandler clears the current active task
func clearCurrentTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.ClearCurrentTaskParams, types.CurrentTaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ClearCurrentTaskParams]) (*mcp.CallToolResultFor[types.CurrentTaskResponse], error) {
		logToolCall("clear-current-task", params.Arguments)

		currentTaskID := GetCurrentTask()

		if currentTaskID == "" {
			response := types.CurrentTaskResponse{
				Success: true,
				Message: "No current task was set",
			}

			return &mcp.CallToolResultFor[types.CurrentTaskResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: "No current task was set",
					},
				},
				StructuredContent: response,
			}, nil
		}

		// Clear the current task
		if err := ClearCurrentTask(); err != nil {
			return &mcp.CallToolResultFor[types.CurrentTaskResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Failed to clear current task: %v", err),
					},
				},
				StructuredContent: types.CurrentTaskResponse{
					Success: false,
					Message: fmt.Sprintf("Failed to clear current task: %v", err),
				},
				IsError: true,
			}, nil
		}

		logInfo(fmt.Sprintf("Cleared current task: %s", currentTaskID))

		response := types.CurrentTaskResponse{
			Success: true,
			Message: fmt.Sprintf("Cleared current task: %s", currentTaskID),
		}

		return &mcp.CallToolResultFor[types.CurrentTaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: response.Message,
				},
			},
			StructuredContent: response,
		}, nil
	}
}

// Helper function to convert task to response pointer
func taskToResponsePtr(task models.Task) *types.TaskResponse {
	response := taskToResponse(task)
	return &response
}

// clearTasksHandler clears tasks with safety features and filtering options
func clearTasksHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.ClearTasksParams, types.ClearTasksResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ClearTasksParams]) (*mcp.CallToolResultFor[types.ClearTasksResponse], error) {
		args := params.Arguments
		startTime := time.Now()

		logToolCall("clear-tasks", args)

		// Build filter based on parameters
		filterFn := func(task models.Task) bool {
			// If --all is specified, match everything
			if args.All {
				return true
			}

			// If --completed is specified or no other filters, match completed tasks
			if args.Completed || (args.Status == "" && args.Priority == "") {
				return task.Status == models.StatusDone
			}

			// Check status filter
			if args.Status != "" {
				statusList := strings.Split(strings.ToLower(args.Status), ",")
				statusMatch := false
				for _, status := range statusList {
					status = strings.TrimSpace(status)
					switch status {
					case "todo":
						if task.Status == models.StatusTodo {
							statusMatch = true
						}
					case "doing":
						if task.Status == models.StatusDoing {
							statusMatch = true
						}
					case "review":
						if task.Status == models.StatusReview {
							statusMatch = true
						}
					case "done":
						if task.Status == models.StatusDone {
							statusMatch = true
						}
					}
				}
				if !statusMatch {
					return false
				}
			}

			// Check priority filter
			if args.Priority != "" {
				priorityList := strings.Split(strings.ToLower(args.Priority), ",")
				priorityMatch := false
				for _, priority := range priorityList {
					priority = strings.TrimSpace(priority)
					switch priority {
					case "low":
						if task.Priority == models.PriorityLow {
							priorityMatch = true
						}
					case "medium":
						if task.Priority == models.PriorityMedium {
							priorityMatch = true
						}
					case "high":
						if task.Priority == models.PriorityHigh {
							priorityMatch = true
						}
					case "urgent":
						if task.Priority == models.PriorityUrgent {
							priorityMatch = true
						}
					}
				}
				if !priorityMatch {
					return false
				}
			}

			return true
		}

		// Get tasks to be cleared
		tasksToDelete, err := taskStore.ListTasks(filterFn, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "clear-candidates")
		}

		if len(tasksToDelete) == 0 {
			response := types.ClearTasksResponse{
				Preview:     args.PreviewOnly,
				Message:     "No tasks match the clearing criteria",
				ExecutionMs: time.Since(startTime).Milliseconds(),
				Criteria: map[string]interface{}{
					"status":   args.Status,
					"priority": args.Priority,
					"all":      args.All,
				},
			}
			return &mcp.CallToolResultFor[types.ClearTasksResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "No tasks match the clearing criteria"},
				},
				StructuredContent: response,
			}, nil
		}

		// Convert tasks to response format
		taskResponses := make([]types.TaskResponse, len(tasksToDelete))
		for i, task := range tasksToDelete {
			taskResponses[i] = taskToResponse(task)
		}

		// If preview only, return the tasks that would be cleared
		if args.PreviewOnly {
			response := types.ClearTasksResponse{
				Preview:     true,
				Tasks:       taskResponses,
				Message:     fmt.Sprintf("Preview: %d tasks would be cleared", len(tasksToDelete)),
				ExecutionMs: time.Since(startTime).Milliseconds(),
				Criteria: map[string]interface{}{
					"status":   args.Status,
					"priority": args.Priority,
					"all":      args.All,
				},
			}
			return &mcp.CallToolResultFor[types.ClearTasksResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Preview: %d tasks would be cleared", len(tasksToDelete))},
				},
				StructuredContent: response,
			}, nil
		}

		// Safety check for dangerous operations
		if args.All && !args.Force {
			return nil, types.NewMCPError("CONFIRMATION_REQUIRED", "Clearing all tasks requires confirmation. Use 'force: true' or run with --force flag", map[string]interface{}{
				"task_count": len(tasksToDelete),
				"suggestion": "Use 'preview_only: true' first to see what would be cleared",
			})
		}

		// Create backup unless disabled
		var backupFile string
		if !args.NoBackup {
			cfg := GetConfig()
			backupDir := filepath.Join(cfg.Project.RootDir, "backups")

			if err := os.MkdirAll(backupDir, 0755); err == nil {
				timestamp := time.Now().Format("2006-01-02_15-04-05")
				backupFile = filepath.Join(backupDir, fmt.Sprintf("clear_backup_%s.json", timestamp))

				backupData := struct {
					Timestamp time.Time     `json:"timestamp"`
					Operation string        `json:"operation"`
					TaskCount int           `json:"task_count"`
					Tasks     []models.Task `json:"tasks"`
				}{
					Timestamp: time.Now(),
					Operation: "clear",
					TaskCount: len(tasksToDelete),
					Tasks:     tasksToDelete,
				}

				if backupErr := writeJSONFile(backupFile, backupData); backupErr != nil {
					logError(fmt.Errorf("failed to create backup: %w", backupErr))
					backupFile = ""
				}
			}
		}

		// Perform the clearing
		cleared := 0
		failed := 0
		for _, task := range tasksToDelete {
			if err := taskStore.DeleteTask(task.ID); err != nil {
				failed++
				logError(fmt.Errorf("failed to clear task '%s': %w", task.Title, err))
			} else {
				cleared++
			}
		}

		// Clear current task if it was deleted
		currentTaskID := GetCurrentTask()
		if currentTaskID != "" {
			for _, task := range tasksToDelete {
				if task.ID == currentTaskID {
					if err := ClearCurrentTask(); err != nil {
						logError(fmt.Errorf("could not clear current task reference: %w", err))
					}
					break
				}
			}
		}

		message := fmt.Sprintf("Successfully cleared %d tasks", cleared)
		if failed > 0 {
			message += fmt.Sprintf(" (%d failed)", failed)
		}
		if backupFile != "" {
			message += fmt.Sprintf(" (backup created: %s)", filepath.Base(backupFile))
		}

		response := types.ClearTasksResponse{
			Preview:       false,
			TasksCleared:  cleared,
			TasksFailed:   failed,
			BackupCreated: backupFile,
			Message:       message,
			ExecutionMs:   time.Since(startTime).Milliseconds(),
			Criteria: map[string]interface{}{
				"status":   args.Status,
				"priority": args.Priority,
				"all":      args.All,
			},
		}

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := message
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcp.CallToolResultFor[types.ClearTasksResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}
