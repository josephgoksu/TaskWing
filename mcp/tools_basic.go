/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package mcp

// Basic MCP tools: add, list, get, update, delete, mark-done

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid" // Kept as it's used by addTaskHandler
	"github.com/josephgoksu/TaskWing/internal/taskutil"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// addTaskHandler creates a new task
func addTaskHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.AddTaskParams, types.TaskResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.AddTaskParams]) (*mcpsdk.CallToolResultFor[types.TaskResponse], error) {
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
			if canon, nerr := taskutil.NormalizePriorityString(normalizedPriority); nerr == nil {
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

		// Note: Parent's SubtaskIDs are automatically updated by the store's CreateTask method

		logInfo(fmt.Sprintf("Created task: %s", createdTask.ID))

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		responseText := fmt.Sprintf("Created task '%s' with ID: %s", createdTask.Title, createdTask.ID)
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcpsdk.CallToolResultFor[types.TaskResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: taskToResponse(createdTask),
			IsError:           false,
		}, nil
	}
}

// listTasksHandler lists tasks with optional filtering
func listTasksHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.ListTasksParams, types.TaskListResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ListTasksParams]) (*mcpsdk.CallToolResultFor[types.TaskListResponse], error) {
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
				responseText += fmt.Sprintf("\n - %s [%s]", tasks[i].Title, taskutil.ShortID(tasks[i].ID))
			}
			responseText += "\nTip: Use set-current-task with id:'<8+ char ID>' or reference:'<title>'"
		}
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcpsdk.CallToolResultFor[types.TaskListResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: response,
		}, nil
	}
}

// updateTaskHandler updates an existing task
func updateTaskHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.UpdateTaskParams, types.TaskResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.UpdateTaskParams]) (*mcpsdk.CallToolResultFor[types.TaskResponse], error) {
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
			if np, nerr := taskutil.NormalizePriorityString(canon); nerr == nil {
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
			// Try to resolve reference
			tasks, lerr := taskStore.ListTasks(nil, nil)
			if lerr == nil {
				if resolved, rerr := taskutil.ResolveTaskReference(id, tasks); rerr == nil {
					if retryTask, retryErr := taskStore.UpdateTask(resolved.ID, updates); retryErr == nil {
						err = nil
						updatedTask = retryTask
					}
				} else {
					return nil, types.NewMCPError("TASK_NOT_FOUND", rerr.Error(), nil)
				}
			}
		}

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

		return &mcpsdk.CallToolResultFor[types.TaskResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: taskToResponse(updatedTask),
		}, nil
	}
}

// deleteTaskHandler deletes a task
func deleteTaskHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.DeleteTaskParams, types.DeleteTaskResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.DeleteTaskParams]) (*mcpsdk.CallToolResultFor[types.DeleteTaskResponse], error) {
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
			// Try to resolve reference
			tasks, lerr := taskStore.ListTasks(nil, nil)
			if lerr == nil {
				if resolved, rerr := taskutil.ResolveTaskReference(id, tasks); rerr == nil {
					task = *resolved
					err = nil
				} else {
					return nil, types.NewMCPError("TASK_NOT_FOUND", rerr.Error(), nil)
				}
			}
		}

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

		return &mcpsdk.CallToolResultFor[types.DeleteTaskResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: response,
		}, nil
	}
}

// markDoneHandler marks a task as completed
func markDoneHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.MarkDoneParams, types.TaskResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.MarkDoneParams]) (*mcpsdk.CallToolResultFor[types.TaskResponse], error) {
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
			// Try to resolve reference
			tasks, lerr := taskStore.ListTasks(nil, nil)
			if lerr == nil {
				if resolved, rerr := taskutil.ResolveTaskReference(args.ID, tasks); rerr == nil {
					completedTask, err = taskStore.MarkTaskDone(resolved.ID)
				} else {
					return nil, types.NewMCPError("TASK_NOT_FOUND", rerr.Error(), nil)
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

		return &mcpsdk.CallToolResultFor[types.TaskResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: responseText,
				},
			},
			StructuredContent: taskToResponse(completedTask),
		}, nil
	}
}

// getTaskHandler retrieves a specific task
func getTaskHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.GetTaskParams, types.TaskResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.GetTaskParams]) (*mcpsdk.CallToolResultFor[types.TaskResponse], error) {
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
			// Try to resolve reference
			tasks, lerr := taskStore.ListTasks(nil, nil)
			if lerr == nil {
				if resolved, rerr := taskutil.ResolveTaskReference(id, tasks); rerr == nil {
					task = *resolved
					err = nil
				} else {
					return nil, types.NewMCPError("TASK_NOT_FOUND", rerr.Error(), nil)
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

		return &mcpsdk.CallToolResultFor[types.TaskResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
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
				less = taskutil.StatusToInt(t1.Status) < taskutil.StatusToInt(t2.Status)
			case "priority":
				less = taskutil.PriorityToInt(t1.Priority) < taskutil.PriorityToInt(t2.Priority)
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
func setCurrentTaskHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.SetCurrentTaskParams, types.CurrentTaskResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.SetCurrentTaskParams]) (*mcpsdk.CallToolResultFor[types.CurrentTaskResponse], error) {
		args := params.Arguments
		logToolCall("set-current-task", args)

		// Accept either ID or Reference
		if strings.TrimSpace(args.ID) == "" && strings.TrimSpace(args.Reference) == "" {
			return &mcpsdk.CallToolResultFor[types.CurrentTaskResponse]{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{
						Text: "Task ID or reference is required",
					},
				},
				StructuredContent: types.CurrentTaskResponse{
					Success: false,
					Message: "Task ID or reference is required",
				},
				IsError: true,
			}, nil
		}

		// Determine lookup key
		ref := strings.TrimSpace(args.ID)
		if ref == "" {
			ref = strings.TrimSpace(args.Reference)
		}

		// Verify the task exists (with resolution fallback)
		task, err := taskStore.GetTask(ref)
		if err != nil {
			// Try to resolve reference
			tasks, lerr := taskStore.ListTasks(nil, nil)
			if lerr == nil {
				if resolved, rerr := taskutil.ResolveTaskReference(ref, tasks); rerr == nil {
					task = *resolved
					err = nil
				} else {
					return nil, types.NewMCPError("TASK_NOT_FOUND", rerr.Error(), nil)
				}
			}
		}

		if err != nil {
			return nil, WrapStoreError(err, "get", ref)
		}

		// Set the current task
		if err := setCurrentTask(task.ID); err != nil {
			return &mcpsdk.CallToolResultFor[types.CurrentTaskResponse]{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{
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

		return &mcpsdk.CallToolResultFor[types.CurrentTaskResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{
					Text: response.Message,
				},
			},
			StructuredContent: response,
		}, nil
	}
}

// taskSummaryHandler returns a summary of the project status
func taskSummaryHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[struct{}, types.TaskSummaryResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[struct{}]) (*mcpsdk.CallToolResultFor[types.TaskSummaryResponse], error) {

		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "summary")
		}

		total := len(tasks)
		var active, completedToday, dueToday, blocked int

		today := time.Now().Format("2006-01-02")

		for _, t := range tasks {
			if t.Status != models.StatusDone {
				active++
			}
			if t.Status == models.StatusDone {
				if t.CompletedAt != nil && t.CompletedAt.Format("2006-01-02") == today {
					completedToday++
				}
			}
			// Simplified due date check (since model might not have structured due date easily accessible without parsing)
			// Skipping dueToday logic for simplicity in this prune.

			// Blocked check
			if len(t.Dependencies) > 0 {
				blocked++ // Naive blocked check
			}
		}

		summary := fmt.Sprintf("Project Status: %d total tasks, %d active.", total, active)

		resp := types.TaskSummaryResponse{
			Summary:        summary,
			TotalTasks:     total,
			ActiveTasks:    active,
			CompletedToday: completedToday,
			DueToday:       dueToday,
			Blocked:        blocked,
		}

		return &mcpsdk.CallToolResultFor[types.TaskSummaryResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: summary},
			},
			StructuredContent: resp,
		}, nil
	}
}
