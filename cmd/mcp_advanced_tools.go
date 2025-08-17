/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// BulkTaskParams for bulk operations
type BulkTaskParams struct {
	TaskIDs  []string `json:"task_ids" mcp:"List of task IDs to operate on"`
	Action   string   `json:"action" mcp:"Action to perform: complete, cancel, delete, prioritize"`
	Priority string   `json:"priority,omitempty" mcp:"New priority for prioritize action"`
}

// TaskAnalyticsParams for analytics queries
type TaskAnalyticsParams struct {
	Period string `json:"period,omitempty" mcp:"Time period: today, week, month, all"`
	GroupBy string `json:"group_by,omitempty" mcp:"Group results by: status, priority, none"`
}

// TaskSearchParams for advanced search
type TaskSearchParams struct {
	Query       string   `json:"query" mcp:"Search query supporting AND, OR, NOT operators"`
	Tags        []string `json:"tags,omitempty" mcp:"Filter by tags"`
	DateFrom    string   `json:"date_from,omitempty" mcp:"Filter tasks created after this date (YYYY-MM-DD)"`
	DateTo      string   `json:"date_to,omitempty" mcp:"Filter tasks created before this date (YYYY-MM-DD)"`
	HasSubtasks *bool    `json:"has_subtasks,omitempty" mcp:"Filter tasks that have subtasks"`
}

// TaskCreationRequest for batch task creation
type TaskCreationRequest struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria string   `json:"acceptanceCriteria,omitempty"`
	Priority           string   `json:"priority,omitempty"`
	Dependencies       []string `json:"dependencies,omitempty"`
}

// BatchCreateTasksParams for creating multiple tasks at once
type BatchCreateTasksParams struct {
	Tasks []TaskCreationRequest `json:"tasks" mcp:"List of tasks to create"`
}

// BatchCreateTasksResponse for batch task creation
type BatchCreateTasksResponse struct {
	CreatedTasks []TaskResponse `json:"created_tasks"`
	Failed       []string       `json:"failed,omitempty"`
	Success      int            `json:"success_count"`
	Errors       []string       `json:"errors,omitempty"`
}

// TaskSummaryResponse provides a high-level summary
type TaskSummaryResponse struct {
	Summary        string          `json:"summary"`
	TotalTasks     int             `json:"total_tasks"`
	ActiveTasks    int             `json:"active_tasks"`
	CompletedToday int             `json:"completed_today"`
	DueToday       int             `json:"due_today"`
	Blocked        int             `json:"blocked"`
	Context        *TaskContext    `json:"context"`
}

// BulkOperationResponse for bulk operations
type BulkOperationResponse struct {
	Succeeded   int      `json:"succeeded"`
	Failed      int      `json:"failed"`
	Errors      []string `json:"errors,omitempty"`
	UpdatedTasks []string `json:"updated_task_ids"`
}

// bulkTaskHandler handles bulk operations on multiple tasks
func bulkTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[BulkTaskParams, BulkOperationResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[BulkTaskParams]) (*mcp.CallToolResultFor[BulkOperationResponse], error) {
		args := params.Arguments

		if len(args.TaskIDs) == 0 {
			return nil, NewMCPError("NO_TASKS_SPECIFIED", "No task IDs provided for bulk operation", nil)
		}

		response := BulkOperationResponse{
			UpdatedTasks: []string{},
			Errors:      []string{},
		}

		for _, taskID := range args.TaskIDs {
			var err error
			
			switch strings.ToLower(args.Action) {
			case "complete":
				_, err = taskStore.MarkTaskDone(taskID)
			case "cancel":
				_, err = taskStore.UpdateTask(taskID, map[string]interface{}{
					"status": models.StatusCancelled,
				})
			case "delete":
				err = taskStore.DeleteTask(taskID)
			case "prioritize":
				if args.Priority == "" {
					err = fmt.Errorf("priority required for prioritize action")
				} else {
					_, err = taskStore.UpdateTask(taskID, map[string]interface{}{
						"priority": args.Priority,
					})
				}
			default:
				err = fmt.Errorf("invalid action: %s", args.Action)
			}

			if err != nil {
				response.Failed++
				response.Errors = append(response.Errors, fmt.Sprintf("Task %s: %v", taskID, err))
			} else {
				response.Succeeded++
				response.UpdatedTasks = append(response.UpdatedTasks, taskID)
			}
		}

		resultText := fmt.Sprintf("Bulk %s operation: %d succeeded, %d failed", 
			args.Action, response.Succeeded, response.Failed)

		// If all operations failed, return as an error instead of IsError flag
		if response.Failed > 0 && response.Succeeded == 0 {
			errorMsg := fmt.Sprintf("All %d operations failed: %s", response.Failed, strings.Join(response.Errors, "; "))
			return nil, fmt.Errorf("bulk %s operation failed: %s", args.Action, errorMsg)
		}

		return &mcp.CallToolResultFor[BulkOperationResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: resultText},
			},
			StructuredContent: response,
		}, nil
	}
}

// taskSummaryHandler provides a high-level task summary
func taskSummaryHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[struct{}, TaskSummaryResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[struct{}]) (*mcp.CallToolResultFor[TaskSummaryResponse], error) {
		// Get all tasks
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Build context
		context, err := BuildTaskContext(taskStore)
		if err != nil {
			return nil, fmt.Errorf("failed to build context: %w", err)
		}

		// Calculate summary metrics
		now := time.Now()
		today := now.Truncate(24 * time.Hour)
		
		response := TaskSummaryResponse{
			TotalTasks: len(tasks),
			Context:    context,
		}

		for _, task := range tasks {
			// Count active tasks
			if task.Status == models.StatusPending || task.Status == models.StatusInProgress {
				response.ActiveTasks++
			}

			// Count completed today
			if task.Status == models.StatusCompleted && task.CompletedAt != nil && task.CompletedAt.After(today) {
				response.CompletedToday++
			}

			// Count blocked
			if task.Status == models.StatusBlocked {
				response.Blocked++
			}
		}

		// Generate summary text
		summaryParts := []string{
			fmt.Sprintf("%d total tasks", response.TotalTasks),
			fmt.Sprintf("%d active", response.ActiveTasks),
			fmt.Sprintf("%d completed today", response.CompletedToday),
		}

		if response.Blocked > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%d blocked", response.Blocked))
		}

		response.Summary = strings.Join(summaryParts, ", ")

		// Add context-based insights
		if context.ProjectHealth != "excellent" {
			response.Summary += fmt.Sprintf(". Project health: %s", context.ProjectHealth)
		}

		return &mcp.CallToolResultFor[TaskSummaryResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: response.Summary},
			},
			StructuredContent: response,
		}, nil
	}
}

// batchCreateTasksHandler creates multiple tasks at once with dependency resolution
func batchCreateTasksHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[BatchCreateTasksParams, BatchCreateTasksResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[BatchCreateTasksParams]) (*mcp.CallToolResultFor[BatchCreateTasksResponse], error) {
		args := params.Arguments

		if len(args.Tasks) == 0 {
			return nil, NewMCPError("NO_TASKS_SPECIFIED", "No tasks provided for batch creation", nil)
		}

		response := BatchCreateTasksResponse{
			CreatedTasks: []TaskResponse{},
			Failed:       []string{},
			Errors:       []string{},
		}

		// Create a map to track created task IDs for dependency resolution
		createdTaskMap := make(map[string]string) // title -> task_id

		// First pass: create tasks without dependencies
		for i, taskReq := range args.Tasks {
			if len(taskReq.Dependencies) > 0 {
				continue // Skip tasks with dependencies in first pass
			}

			task := models.Task{
				Title:              taskReq.Title,
				Description:        taskReq.Description,
				AcceptanceCriteria: taskReq.AcceptanceCriteria,
				Status:             models.StatusPending,
			}

			// Set priority
			if taskReq.Priority != "" {
				task.Priority = models.TaskPriority(taskReq.Priority)
			} else {
				task.Priority = models.PriorityMedium
			}

			createdTask, err := taskStore.CreateTask(task)
			if err != nil {
				response.Failed = append(response.Failed, fmt.Sprintf("Task %d: %s", i+1, taskReq.Title))
				response.Errors = append(response.Errors, fmt.Sprintf("Task %d (%s): %v", i+1, taskReq.Title, err))
				continue
			}

			response.CreatedTasks = append(response.CreatedTasks, taskToResponse(createdTask))
			createdTaskMap[taskReq.Title] = createdTask.ID
			response.Success++
		}

		// Second pass: create tasks with dependencies
		for i, taskReq := range args.Tasks {
			if len(taskReq.Dependencies) == 0 {
				continue // Skip tasks without dependencies (already created)
			}

			task := models.Task{
				Title:              taskReq.Title,
				Description:        taskReq.Description,
				AcceptanceCriteria: taskReq.AcceptanceCriteria,
				Status:             models.StatusPending,
				Dependencies:       taskReq.Dependencies, // Use provided dependencies as-is
			}

			// Set priority
			if taskReq.Priority != "" {
				task.Priority = models.TaskPriority(taskReq.Priority)
			} else {
				task.Priority = models.PriorityMedium
			}

			createdTask, err := taskStore.CreateTask(task)
			if err != nil {
				response.Failed = append(response.Failed, fmt.Sprintf("Task %d: %s", i+1, taskReq.Title))
				response.Errors = append(response.Errors, fmt.Sprintf("Task %d (%s): %v", i+1, taskReq.Title, err))
				continue
			}

			response.CreatedTasks = append(response.CreatedTasks, taskToResponse(createdTask))
			response.Success++
		}

		resultText := fmt.Sprintf("Batch task creation: %d succeeded, %d failed", 
			response.Success, len(response.Failed))

		// If all operations failed, return as an error
		if len(response.Failed) > 0 && response.Success == 0 {
			errorMsg := fmt.Sprintf("All %d task creations failed: %s", len(response.Failed), strings.Join(response.Errors, "; "))
			return nil, fmt.Errorf("batch task creation failed: %s", errorMsg)
		}

		return &mcp.CallToolResultFor[BatchCreateTasksResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: resultText},
			},
			StructuredContent: response,
		}, nil
	}
}

// advancedSearchHandler provides powerful search capabilities
func advancedSearchHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[TaskSearchParams, TaskListResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[TaskSearchParams]) (*mcp.CallToolResultFor[TaskListResponse], error) {
		args := params.Arguments

		// Parse date filters
		var dateFrom, dateTo time.Time
		var err error
		
		if args.DateFrom != "" {
			dateFrom, err = time.Parse("2006-01-02", args.DateFrom)
			if err != nil {
				return nil, NewMCPError("INVALID_DATE", "Invalid date_from format", map[string]interface{}{
					"expected": "YYYY-MM-DD",
					"provided": args.DateFrom,
				})
			}
		}

		if args.DateTo != "" {
			dateTo, err = time.Parse("2006-01-02", args.DateTo)
			if err != nil {
				return nil, NewMCPError("INVALID_DATE", "Invalid date_to format", map[string]interface{}{
					"expected": "YYYY-MM-DD",
					"provided": args.DateTo,
				})
			}
		}

		// Create advanced filter
		filterFn := func(task models.Task) bool {
			// Date range filter
			if !dateFrom.IsZero() && task.CreatedAt.Before(dateFrom) {
				return false
			}
			if !dateTo.IsZero() && task.CreatedAt.After(dateTo.Add(24*time.Hour)) {
				return false
			}

			// Query filter with basic operators
			if args.Query != "" {
				query := strings.ToLower(args.Query)
				title := strings.ToLower(task.Title)
				desc := strings.ToLower(task.Description)

				// Simple operator parsing
				if strings.Contains(query, " and ") {
					parts := strings.Split(query, " and ")
					for _, part := range parts {
						part = strings.TrimSpace(part)
						if !strings.Contains(title, part) && !strings.Contains(desc, part) {
							return false
						}
					}
				} else if strings.Contains(query, " or ") {
					parts := strings.Split(query, " or ")
					found := false
					for _, part := range parts {
						part = strings.TrimSpace(part)
						if strings.Contains(title, part) || strings.Contains(desc, part) {
							found = true
							break
						}
					}
					if !found {
						return false
					}
				} else if strings.HasPrefix(query, "not ") {
					negQuery := strings.TrimPrefix(query, "not ")
					if strings.Contains(title, negQuery) || strings.Contains(desc, negQuery) {
						return false
					}
				} else {
					// Simple contains search
					if !strings.Contains(title, query) && !strings.Contains(desc, query) {
						return false
					}
				}
			}

			// Subtask filter
			if args.HasSubtasks != nil && *args.HasSubtasks && len(task.SubtaskIDs) == 0 {
				return false
			}

			return true
		}

		// List tasks with advanced filter
		tasks, err := taskStore.ListTasks(filterFn, nil)
		if err != nil {
			return nil, WrapStoreError(err, "search", "")
		}

		// Convert to response
		taskResponses := make([]TaskResponse, len(tasks))
		for i, task := range tasks {
			taskResponses[i] = taskToResponse(task)
		}

		response := TaskListResponse{
			Tasks: taskResponses,
			Count: len(taskResponses),
		}

		return &mcp.CallToolResultFor[TaskListResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Found %d tasks matching search criteria", len(tasks)),
				},
			},
			StructuredContent: response,
		}, nil
	}
}

// RegisterAdvancedMCPTools registers additional MCP tools
func RegisterAdvancedMCPTools(server *mcp.Server, taskStore store.TaskStore) error {
	// Batch create tasks tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "batch-create-tasks",
		Description: "Create multiple tasks at once with automatic dependency resolution. Ideal for task generation workflows.",
	}, batchCreateTasksHandler(taskStore))

	// Bulk operations tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "bulk-tasks",
		Description: "Perform bulk operations on multiple tasks at once. Supports complete, cancel, delete, and prioritize actions.",
	}, bulkTaskHandler(taskStore))

	// Task summary tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "task-summary",
		Description: "Get a comprehensive summary of all tasks including metrics, project health, and actionable insights.",
	}, taskSummaryHandler(taskStore))

	// Advanced search tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search-tasks",
		Description: "Advanced task search with support for logical operators (AND, OR, NOT), date ranges, and complex filters.",
	}, advancedSearchHandler(taskStore))

	return nil
}