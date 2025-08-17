/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/josephgoksu/taskwing.app/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addTaskHandler creates a new task
func addTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.AddTaskParams, types.TaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.AddTaskParams]) (*mcp.CallToolResultFor[types.TaskResponse], error) {
		args := params.Arguments
		logToolCall("add-task", args)

		// Validate required fields
		if strings.TrimSpace(args.Title) == "" {
			return nil, NewMCPError("MISSING_TITLE", "Task title is required", map[string]interface{}{
				"field": "title",
			})
		}

		// Validate input
		if err := ValidateTaskInput(args.Title, args.Priority, ""); err != nil {
			return nil, err
		}

		// Set priority with default
		priority := models.PriorityMedium
		if args.Priority != "" {
			switch strings.ToLower(args.Priority) {
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
				return nil, NewMCPError("PARENT_NOT_FOUND", fmt.Sprintf("Parent task %s not found", args.ParentID), map[string]interface{}{
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
			Status:             models.StatusPending,
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
			return nil, NewMCPError("VALIDATION_FAILED", fmt.Sprintf("Task validation failed: %s", err.Error()), nil)
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

		return &mcp.CallToolResultFor[TaskResponse]{
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
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcp.CallToolResultFor[TaskListResponse]{
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

		// Validate required fields
		if strings.TrimSpace(args.ID) == "" {
			return nil, NewMCPError("MISSING_ID", "Task ID is required for update", nil)
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
			if err := ValidateTaskInput("", args.Priority, ""); err != nil {
				return nil, err
			}
			updates["priority"] = models.TaskPriority(args.Priority)
		}

		if args.Dependencies != nil {
			updates["dependencies"] = args.Dependencies
		}

		if args.ParentID != "" {
			// Validate that parent task exists
			_, err := taskStore.GetTask(args.ParentID)
			if err != nil {
				return nil, NewMCPError("PARENT_NOT_FOUND", fmt.Sprintf("Parent task %s not found", args.ParentID), map[string]interface{}{
					"parent_id": args.ParentID,
				})
			}
			updates["parentId"] = &args.ParentID
		}

		// Update task
		updatedTask, err := taskStore.UpdateTask(args.ID, updates)
		if err != nil {
			return nil, WrapStoreError(err, "update", args.ID)
		}

		logInfo(fmt.Sprintf("Updated task: %s", updatedTask.ID))

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := fmt.Sprintf("Updated task '%s' (ID: %s)", updatedTask.Title, updatedTask.ID)
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcp.CallToolResultFor[TaskResponse]{
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

		// Validate required fields
		if strings.TrimSpace(args.ID) == "" {
			return nil, NewMCPError("MISSING_ID", "Task ID is required for deletion", nil)
		}

		// Get task to check for dependents and for response text
		task, err := taskStore.GetTask(args.ID)
		if err != nil {
			return nil, WrapStoreError(err, "get_for_delete", args.ID)
		}

		// Check if task has dependents
		if len(task.Dependents) > 0 {
			return nil, NewMCPError("HAS_DEPENDENTS", "Cannot delete task with dependent tasks", map[string]interface{}{
				"task_id":    args.ID,
				"dependents": task.Dependents,
			})
		}

		// Delete task
		if err := taskStore.DeleteTask(args.ID); err != nil {
			return nil, WrapStoreError(err, "delete", args.ID)
		}

		logInfo(fmt.Sprintf("Deleted task: %s", args.ID))

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := fmt.Sprintf("Deleted task '%s' (ID: %s)", task.Title, task.ID)
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		response := types.DeleteTaskResponse{
			Success: true,
			TaskID:  args.ID,
			Message: fmt.Sprintf("Task '%s' deleted successfully", task.Title),
		}

		return &mcp.CallToolResultFor[DeleteTaskResponse]{
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

		// Validate required fields
		if strings.TrimSpace(args.ID) == "" {
			return nil, NewMCPError("MISSING_ID", "Task ID is required to mark as done", nil)
		}

		// Mark task as done
		completedTask, err := taskStore.MarkTaskDone(args.ID)
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

		return &mcp.CallToolResultFor[TaskResponse]{
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

		// Validate required fields
		if strings.TrimSpace(args.ID) == "" {
			return nil, NewMCPError("MISSING_ID", "Task ID is required to get a task", nil)
		}

		// Get task
		task, err := taskStore.GetTask(args.ID)
		if err != nil {
			return nil, WrapStoreError(err, "get", args.ID)
		}

		logInfo(fmt.Sprintf("Retrieved task: %s", task.ID))

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := fmt.Sprintf("Task '%s' (ID: %s)", task.Title, task.ID)
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcp.CallToolResultFor[TaskResponse]{
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
