/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addTaskHandler creates a new task
func addTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[AddTaskParams, TaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[AddTaskParams]) (*mcp.CallToolResultFor[TaskResponse], error) {
		args := params.Arguments

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

		// Create new task
		task := models.Task{
			ID:                 uuid.New().String(),
			Title:              strings.TrimSpace(args.Title),
			Description:        strings.TrimSpace(args.Description),
			AcceptanceCriteria: strings.TrimSpace(args.AcceptanceCriteria),
			Status:             models.StatusPending,
			Priority:           priority,
			Dependencies:       args.Dependencies,
			Dependents:         []string{},
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		// Validate task
		if err := models.ValidateStruct(task); err != nil {
			return nil, fmt.Errorf("task validation failed: %w", err)
		}

		// Create task in store
		createdTask, err := taskStore.CreateTask(task)
		if err != nil {
			return nil, WrapStoreError(err, "create", task.ID)
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
			IsError: false,
		}, nil
	}
}

// listTasksHandler lists tasks with optional filtering
func listTasksHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[ListTasksParams, TaskListResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[ListTasksParams]) (*mcp.CallToolResultFor[TaskListResponse], error) {
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
			return nil, fmt.Errorf("failed to list tasks: %w", err)
		}

		// Convert to response format
		taskResponses := make([]TaskResponse, len(tasks))
		for i, task := range tasks {
			taskResponses[i] = taskToResponse(task)
		}

		response := TaskListResponse{
			Tasks: taskResponses,
			Count: len(taskResponses),
		}

		logInfo(fmt.Sprintf("Listed %d tasks", len(tasks)))

		return &mcp.CallToolResultFor[TaskListResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Found %d tasks", len(tasks)),
				},
			},
			StructuredContent: response,
		}, nil
	}
}

// updateTaskHandler updates an existing task
func updateTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[UpdateTaskParams, TaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[UpdateTaskParams]) (*mcp.CallToolResultFor[TaskResponse], error) {
		args := params.Arguments

		// Validate required fields
		if strings.TrimSpace(args.ID) == "" {
			return nil, fmt.Errorf("task ID is required")
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
			// Validate status
			switch strings.ToLower(args.Status) {
			case "pending":
				updates["status"] = models.StatusPending
			case "in-progress":
				updates["status"] = models.StatusInProgress
			case "completed":
				updates["status"] = models.StatusCompleted
			case "cancelled":
				updates["status"] = models.StatusCancelled
			case "on-hold":
				updates["status"] = models.StatusOnHold
			case "blocked":
				updates["status"] = models.StatusBlocked
			case "needs-review":
				updates["status"] = models.StatusNeedsReview
			default:
				return nil, fmt.Errorf("invalid status: %s", args.Status)
			}
		}

		if args.Priority != "" {
			// Validate priority
			switch strings.ToLower(args.Priority) {
			case "low":
				updates["priority"] = models.PriorityLow
			case "medium":
				updates["priority"] = models.PriorityMedium
			case "high":
				updates["priority"] = models.PriorityHigh
			case "urgent":
				updates["priority"] = models.PriorityUrgent
			default:
				return nil, fmt.Errorf("invalid priority: %s", args.Priority)
			}
		}

		if args.Dependencies != nil {
			updates["dependencies"] = args.Dependencies
		}

		// Update task
		updatedTask, err := taskStore.UpdateTask(args.ID, updates)
		if err != nil {
			return nil, fmt.Errorf("failed to update task: %w", err)
		}

		logInfo(fmt.Sprintf("Updated task: %s", updatedTask.ID))

		return &mcp.CallToolResultFor[TaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Updated task '%s' (ID: %s)", updatedTask.Title, updatedTask.ID),
				},
			},
			StructuredContent: taskToResponse(updatedTask),
		}, nil
	}
}

// deleteTaskHandler deletes a task
func deleteTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[DeleteTaskParams, bool] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[DeleteTaskParams]) (*mcp.CallToolResultFor[bool], error) {
		args := params.Arguments

		// Validate required fields
		if strings.TrimSpace(args.ID) == "" {
			return nil, fmt.Errorf("task ID is required")
		}

		// Get task to check for dependents
		task, err := taskStore.GetTask(args.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task: %w", err)
		}

		// Check if task has dependents
		if len(task.Dependents) > 0 {
			return nil, fmt.Errorf("cannot delete task with dependents: %v", task.Dependents)
		}

		// Delete task
		if err := taskStore.DeleteTask(args.ID); err != nil {
			return nil, fmt.Errorf("failed to delete task: %w", err)
		}

		logInfo(fmt.Sprintf("Deleted task: %s", args.ID))

		return &mcp.CallToolResultFor[bool]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Deleted task '%s' (ID: %s)", task.Title, task.ID),
				},
			},
			StructuredContent: true,
		}, nil
	}
}

// markDoneHandler marks a task as completed
func markDoneHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[MarkDoneParams, TaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[MarkDoneParams]) (*mcp.CallToolResultFor[TaskResponse], error) {
		args := params.Arguments

		// Validate required fields
		if strings.TrimSpace(args.ID) == "" {
			return nil, fmt.Errorf("task ID is required")
		}

		// Mark task as done
		completedTask, err := taskStore.MarkTaskDone(args.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to mark task as done: %w", err)
		}

		logInfo(fmt.Sprintf("Marked task as done: %s", completedTask.ID))

		return &mcp.CallToolResultFor[TaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Marked task '%s' as completed (ID: %s)", completedTask.Title, completedTask.ID),
				},
			},
			StructuredContent: taskToResponse(completedTask),
		}, nil
	}
}

// getTaskHandler retrieves a specific task
func getTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[GetTaskParams, TaskResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[GetTaskParams]) (*mcp.CallToolResultFor[TaskResponse], error) {
		args := params.Arguments

		// Validate required fields
		if strings.TrimSpace(args.ID) == "" {
			return nil, fmt.Errorf("task ID is required")
		}

		// Get task
		task, err := taskStore.GetTask(args.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task: %w", err)
		}

		logInfo(fmt.Sprintf("Retrieved task: %s", task.ID))

		return &mcp.CallToolResultFor[TaskResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Task '%s' (ID: %s)", task.Title, task.ID),
				},
			},
			StructuredContent: taskToResponse(task),
		}, nil
	}
}

// Helper function to create sort function
func createSortFunction(sortBy, sortOrder string) func([]models.Task) []models.Task {
	return func(tasks []models.Task) []models.Task {
		// For now, return tasks as-is since the store interface doesn't specify sorting
		// In a real implementation, you'd sort based on the sortBy field
		return tasks
	}
}
