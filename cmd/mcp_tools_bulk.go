/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

// Bulk and advanced MCP tools: batch create, bulk ops, search, summary

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bulkTaskHandler handles bulk operations on multiple tasks
func bulkTaskHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.BulkTaskParams, types.BulkOperationResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.BulkTaskParams]) (*mcp.CallToolResultFor[types.BulkOperationResponse], error) {
		args := params.Arguments

		if len(args.TaskIDs) == 0 {
			return nil, types.NewMCPError("NO_TASKS_SPECIFIED", "No task IDs provided for bulk operation", nil)
		}

		response := types.BulkOperationResponse{
			UpdatedTasks: []string{},
			Errors:       []string{},
		}

		// Preload tasks for reference resolution
		allTasks, _ := taskStore.ListTasks(nil, nil)

		for _, ref := range args.TaskIDs {
			var err error
			taskID := ref
			// If direct op fails later, we will retry with resolved ID

			action := strings.ToLower(args.Action)
			if action == "cancel" {
				return nil, types.NewMCPError("UNSUPPORTED_ACTION", "Bulk action 'cancel' is deprecated. Use 'delete' or update status explicitly.", nil)
			}

			switch action {
			case "complete":
				// Try direct
				if _, derr := taskStore.MarkTaskDone(taskID); derr != nil {
					if resolvedID, _, ok := resolveReference(ref, allTasks); ok {
						_, err = taskStore.MarkTaskDone(resolvedID)
						taskID = resolvedID
					} else {
						err = derr
					}
				}
			case "delete":
				if derr := taskStore.DeleteTask(taskID); derr != nil {
					if resolvedID, _, ok := resolveReference(ref, allTasks); ok {
						err = taskStore.DeleteTask(resolvedID)
						taskID = resolvedID
					} else {
						err = derr
					}
				}
			case "prioritize":
				if args.Priority == "" {
					err = fmt.Errorf("priority required for prioritize action")
				} else {
					// Normalize priority before applying
					canon := args.Priority
					if np, nerr := normalizePriorityString(canon); nerr == nil {
						canon = np
					} else {
						err = types.NewMCPError("INVALID_PRIORITY", nerr.Error(), map[string]interface{}{
							"value":        args.Priority,
							"valid_values": []string{"low", "medium", "high", "urgent"},
						})
						break
					}
					if _, uerr := taskStore.UpdateTask(taskID, map[string]interface{}{"priority": canon}); uerr != nil {
						if resolvedID, _, ok := resolveReference(ref, allTasks); ok {
							_, err = taskStore.UpdateTask(resolvedID, map[string]interface{}{"priority": canon})
							taskID = resolvedID
						} else {
							err = uerr
						}
					}
				}
			default:
				err = fmt.Errorf("invalid action: %s", args.Action)
			}

			if err != nil {
				response.Failed++
				response.Errors = append(response.Errors, fmt.Sprintf("Task %s: %v", ref, err))
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

		return &mcp.CallToolResultFor[types.BulkOperationResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: resultText},
			},
			StructuredContent: response,
		}, nil
	}
}

// taskSummaryHandler provides a high-level task summary
func taskSummaryHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[struct{}, types.TaskSummaryResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[struct{}]) (*mcp.CallToolResultFor[types.TaskSummaryResponse], error) {
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

		response := types.TaskSummaryResponse{
			TotalTasks: len(tasks),
			Context:    context,
		}

		for _, task := range tasks {
			// Count active tasks
			if task.Status == models.StatusTodo || task.Status == models.StatusDoing {
				response.ActiveTasks++
			}

			// Count completed today
			if task.Status == models.StatusDone && task.CompletedAt != nil && task.CompletedAt.After(today) {
				response.CompletedToday++
			}

			// Count blocked
			if task.Status == models.StatusReview {
				response.Blocked++
			}

			// Count due today (urgent and not completed)
			if task.Priority == models.PriorityUrgent && task.Status != models.StatusDone {
				response.DueToday++
			}
		}

		// Generate summary text
		summaryParts := []string{
			fmt.Sprintf("%d total tasks", response.TotalTasks),
			fmt.Sprintf("%d active", response.ActiveTasks),
			fmt.Sprintf("%d completed today", response.CompletedToday),
		}

		if response.DueToday > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%d due today", response.DueToday))
		}

		if response.Blocked > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%d blocked", response.Blocked))
		}

		response.Summary = strings.Join(summaryParts, ", ")

		// Add context-based insights
		if context.ProjectHealth != "excellent" {
			response.Summary += fmt.Sprintf(". Project health: %s", context.ProjectHealth)
		}

		return &mcp.CallToolResultFor[types.TaskSummaryResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: response.Summary},
			},
			StructuredContent: response,
		}, nil
	}
}

// batchCreateTasksHandler creates multiple tasks at once with dependency resolution
func batchCreateTasksHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.BatchCreateTasksParams, types.BatchCreateTasksResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.BatchCreateTasksParams]) (*mcp.CallToolResultFor[types.BatchCreateTasksResponse], error) {
		args := params.Arguments

		if len(args.Tasks) == 0 {
			return nil, types.NewMCPError("NO_TASKS_SPECIFIED", "No tasks provided for batch creation", nil)
		}

		// Pre-validate placeholders; allow TempIDs (integers) for parent/dependencies
		for i, taskReq := range args.Tasks {
			if taskReq.ParentID != "" {
				// Check if it's a TempID (integer) - these are allowed
				if _, err := strconv.Atoi(taskReq.ParentID); err != nil {
					// Not a TempID, check for placeholder patterns
					if strings.HasPrefix(taskReq.ParentID, "task_") ||
						strings.Contains(taskReq.ParentID, "placeholder") ||
						(!strings.Contains(taskReq.ParentID, "-") && func() bool { _, err := uuid.Parse(taskReq.ParentID); return err != nil }()) { // allow valid UUIDs
						return nil, fmt.Errorf("task %d (%s): parentId '%s' appears to be a placeholder. Use list-tasks to get real UUID values like '7b3e4f2a-8c9d-4e5f-b0a1-2c3d4e5f6a7b', or use TempID (integer) for batch parent-child relationships", i+1, taskReq.Title, taskReq.ParentID)
					}
				}
			}
			// Also check dependencies for placeholder patterns
			for _, depID := range taskReq.Dependencies {
				// Allow TempID (integer) or a valid UUID; reject obvious placeholders
				if _, err := strconv.Atoi(depID); err == nil {
					continue
				}
				if _, err := uuid.Parse(depID); err == nil {
					continue
				}
				if strings.HasPrefix(depID, "task_") || strings.Contains(depID, "placeholder") {
					return nil, fmt.Errorf("task %d (%s): dependency '%s' appears to be a placeholder. Use tempId integers within the batch or real UUID values", i+1, taskReq.Title, depID)
				}
			}
		}

		response := types.BatchCreateTasksResponse{
			CreatedTasks: []types.TaskResponse{},
			Failed:       []string{},
			Errors:       []string{},
		}

		// TempID (from request's TempID) -> final UUID
		tempIDToFinalID := make(map[int]string)
		tasksToCreate := args.Tasks

		// Process tasks in multiple passes to handle parent-child relationships
		for len(tasksToCreate) > 0 {
			tasksCreatedInPass := 0
			var remainingTasks []types.TaskCreationRequest

			for _, taskReq := range tasksToCreate {
				var parentID *string
				if taskReq.ParentID != "" {
					parentTempID, err := strconv.Atoi(taskReq.ParentID)
					if err != nil {
						// If it's not an integer, assume it's a UUID of an *existing* task
						if _, err := uuid.Parse(taskReq.ParentID); err == nil {
							parentID = &taskReq.ParentID
						} else {
							response.Failed = append(response.Failed, taskReq.Title)
							response.Errors = append(response.Errors, fmt.Sprintf("invalid parentID: %s", taskReq.ParentID))
							continue
						}
					} else {
						if finalParentID, ok := tempIDToFinalID[parentTempID]; ok {
							parentID = &finalParentID
						} else {
							// Parent not created yet, try again in the next pass
							remainingTasks = append(remainingTasks, taskReq)
							continue
						}
					}
				}

				// Create without dependencies first; we'll apply dependencies after all tasks exist
				createdTask, err := createTaskFromRequest(taskStore, taskReq, parentID)
				if err != nil {
					response.Failed = append(response.Failed, taskReq.Title)
					response.Errors = append(response.Errors, err.Error())
				} else {
					response.CreatedTasks = append(response.CreatedTasks, taskToResponse(createdTask))
					response.Success++
					if taskReq.TempID != 0 {
						tempIDToFinalID[taskReq.TempID] = createdTask.ID
					}
					tasksCreatedInPass++
				}
			}

			if tasksCreatedInPass == 0 && len(remainingTasks) > 0 {
				for _, taskReq := range remainingTasks {
					response.Failed = append(response.Failed, taskReq.Title)
					response.Errors = append(response.Errors, fmt.Sprintf("could not find parent with TempID: %s", taskReq.ParentID))
				}
				break
			}
			tasksToCreate = remainingTasks
		}

		// Post-pass: resolve and apply dependencies using TempID mapping (and UUIDs when provided)
		for _, taskReq := range args.Tasks {
			if len(taskReq.Dependencies) == 0 {
				continue
			}
			// Resolve the created task's final ID
			var targetID string
			if taskReq.TempID != 0 {
				if id, ok := tempIDToFinalID[taskReq.TempID]; ok {
					targetID = id
				} else {
					response.Failed = append(response.Failed, taskReq.Title)
					response.Errors = append(response.Errors, fmt.Sprintf("could not resolve final ID for tempId %d", taskReq.TempID))
					continue
				}
			} else {
				// Fallback by matching title among created tasks (best-effort, may be ambiguous)
				matchCount := 0
				for _, ct := range response.CreatedTasks {
					if ct.Title == taskReq.Title {
						targetID = ct.ID
						matchCount++
					}
				}
				if targetID == "" || matchCount > 1 {
					response.Failed = append(response.Failed, taskReq.Title)
					if matchCount > 1 {
						response.Errors = append(response.Errors, fmt.Sprintf("ambiguous title '%s' for dependency resolution; provide tempId", taskReq.Title))
					} else {
						response.Errors = append(response.Errors, fmt.Sprintf("could not resolve created task ID for title '%s'", taskReq.Title))
					}
					continue
				}
			}

			// Build final dependency IDs list
			var finalDeps []string
			depErr := false
			for _, dep := range taskReq.Dependencies {
				if n, err := strconv.Atoi(dep); err == nil {
					if fid, ok := tempIDToFinalID[n]; ok {
						finalDeps = append(finalDeps, fid)
					} else {
						response.Errors = append(response.Errors, fmt.Sprintf("dependency tempId %d not found for task '%s'", n, taskReq.Title))
						depErr = true
					}
					continue
				}
				if _, err := uuid.Parse(dep); err == nil {
					finalDeps = append(finalDeps, dep)
					continue
				}
				response.Errors = append(response.Errors, fmt.Sprintf("invalid dependency reference '%s' for task '%s'", dep, taskReq.Title))
				depErr = true
			}
			if depErr {
				response.Failed = append(response.Failed, taskReq.Title)
				continue
			}

			if _, err := taskStore.UpdateTask(targetID, map[string]interface{}{"dependencies": finalDeps}); err != nil {
				response.Failed = append(response.Failed, taskReq.Title)
				response.Errors = append(response.Errors, fmt.Sprintf("failed to set dependencies for '%s': %v", taskReq.Title, err))
			}
		}

		resultText := fmt.Sprintf("Batch task creation: %d succeeded, %d failed",
			response.Success, len(response.Failed))

		// If all operations failed, return as an error
		if len(response.Failed) > 0 && response.Success == 0 {
			errorMsg := fmt.Sprintf("All %d task creations failed: %s", len(response.Failed), strings.Join(response.Errors, "; "))
			return nil, fmt.Errorf("batch task creation failed: %s", errorMsg)
		}

		return &mcp.CallToolResultFor[types.BatchCreateTasksResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: resultText},
			},
			StructuredContent: response,
		}, nil
	}
}

// advancedSearchHandler provides powerful search capabilities
func advancedSearchHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.TaskSearchParams, types.TaskListResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.TaskSearchParams]) (*mcp.CallToolResultFor[types.TaskListResponse], error) {
		args := params.Arguments

		// Parse date filters
		var dateFrom, dateTo time.Time
		var err error

		if args.DateFrom != "" {
			dateFrom, err = time.Parse("2006-01-02", args.DateFrom)
			if err != nil {
				return nil, types.NewMCPError("INVALID_DATE", "Invalid date_from format", map[string]interface{}{
					"expected": "YYYY-MM-DD",
					"provided": args.DateFrom,
				})
			}
		}

		if args.DateTo != "" {
			dateTo, err = time.Parse("2006-01-02", args.DateTo)
			if err != nil {
				return nil, types.NewMCPError("INVALID_DATE", "Invalid date_to format", map[string]interface{}{
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
		taskResponses := make([]types.TaskResponse, len(tasks))
		for i, task := range tasks {
			taskResponses[i] = taskToResponse(task)
		}

		response := types.TaskListResponse{
			Tasks: taskResponses,
			Count: len(taskResponses),
		}

		// Build compact text with top results and short IDs
		text := fmt.Sprintf("Found %d tasks matching search criteria", len(tasks))
		if len(response.Tasks) > 0 {
			maxShow := 5
			if len(response.Tasks) < maxShow {
				maxShow = len(response.Tasks)
			}
			text += "\nTop:"
			for i := 0; i < maxShow; i++ {
				text += fmt.Sprintf("\n - %s [%s]", response.Tasks[i].Title, shortID(response.Tasks[i].ID))
			}
		}

		return &mcp.CallToolResultFor[types.TaskListResponse]{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: response,
		}, nil
	}
}

func createTaskFromRequest(taskStore store.TaskStore, taskReq types.TaskCreationRequest, parentID *string) (models.Task, error) {
	task := models.Task{
		Title:              taskReq.Title,
		Description:        taskReq.Description,
		AcceptanceCriteria: taskReq.AcceptanceCriteria,
		Status:             models.StatusTodo,
		ParentID:           parentID,
		SubtaskIDs:         []string{},
		// Create without dependencies; apply after all tasks created
		Dependencies: []string{},
	}

	if taskReq.Priority != "" {
		task.Priority = models.TaskPriority(taskReq.Priority)
	} else {
		task.Priority = models.PriorityMedium
	}

	return taskStore.CreateTask(task)
}

// suggestPatternHandler suggests task patterns based on description
// Pattern suggestion functionality removed as part of cleanup.

// RegisterAdvancedMCPTools registers additional MCP tools
func RegisterAdvancedMCPTools(server *mcp.Server, taskStore store.TaskStore) error {
	// Batch create tasks tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "batch-create-tasks",
		Description: "Create many tasks at once. Supports TempID-based parent-child linking, dependencies, and priorities. Returns created_tasks, errors, and success_count.",
	}, batchCreateTasksHandler(taskStore))

	// Bulk operations tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "bulk-tasks",
		Description: "Bulk operate on task_ids (IDs or references). Actions: complete, delete, prioritize (requires priority [low|medium|high|urgent]). Returns succeeded/failed and updated_task_ids.",
	}, bulkTaskHandler(taskStore))

	// Bulk by filter tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "bulk-by-filter",
		Description: "Bulk operate by filter/expression/query with preview/confirm. Actions: complete, delete, prioritize (requires priority).",
	}, bulkByFilterHandler(taskStore))

	// Advanced search tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search-tasks",
		Description: "🔍 ADVANCED SEARCH (use for complex queries): Boolean operators (AND/OR/NOT), date ranges, tags. Example: '(urgent OR high) AND NOT done'. Best for power users.",
	}, advancedSearchHandler(taskStore))

	// Note: suggest-patterns tool removed.

	return nil
}

// bulkByFilterHandler applies bulk operations to tasks matched by filter/expression/query
func bulkByFilterHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.BulkByFilterParams, types.BulkByFilterResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.BulkByFilterParams]) (*mcp.CallToolResultFor[types.BulkByFilterResponse], error) {
		args := params.Arguments

		// Load all tasks
		all, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Reuse enhanced filters
		var matched []models.Task
		var ferr error
		criteria := ""
		if strings.TrimSpace(args.Query) != "" {
			matched, _, ferr = applyNaturalLanguageFilter(all, args.Query, true)
			criteria = "query=" + args.Query
		} else if strings.TrimSpace(args.Expression) != "" {
			matched, _, ferr = applyEnhancedComplexFilter(all, args.Expression)
			criteria = "expression=" + args.Expression
		} else if strings.TrimSpace(args.Filter) != "" {
			matched, _, ferr = applyEnhancedSimpleFilter(all, args.Filter)
			criteria = "filter=" + args.Filter
		} else {
			return nil, types.NewMCPError("MISSING_FILTER", "Provide query, expression, or filter", nil)
		}
		if ferr != nil {
			return nil, types.NewMCPError("FILTER_ERROR", ferr.Error(), nil)
		}

		if args.Limit > 0 && len(matched) > args.Limit {
			matched = matched[:args.Limit]
		}

		if args.PreviewOnly || !args.Confirm {
			resp := types.BulkByFilterResponse{
				Preview:  true,
				Criteria: criteria,
				Matched:  len(matched),
			}
			text := fmt.Sprintf("Preview: %d tasks match (%s)", len(matched), criteria)
			return &mcp.CallToolResultFor[types.BulkByFilterResponse]{Content: []mcp.Content{&mcp.TextContent{Text: text}}, StructuredContent: resp}, nil
		}

		// Apply action
		succeeded := 0
		failed := 0
		var ids []string
		var errorsList []string

		for _, t := range matched {
			var aerr error
			acted := false
			switch strings.ToLower(args.Action) {
			case "complete", "done", "mark-done":
				if t.Status != models.StatusDone {
					if _, aerr = taskStore.MarkTaskDone(t.ID); aerr == nil {
						acted = true
					}
				}
			case "delete":
				if err := taskStore.DeleteTask(t.ID); err != nil {
					aerr = err
				} else {
					acted = true
				}
			case "prioritize":
				if args.Priority == "" {
					aerr = fmt.Errorf("priority required for prioritize action")
				} else {
					if _, aerr = taskStore.UpdateTask(t.ID, map[string]interface{}{"priority": args.Priority}); aerr == nil {
						acted = true
					}
				}
			default:
				aerr = fmt.Errorf("invalid action: %s", args.Action)
			}
			if aerr != nil {
				failed++
				errorsList = append(errorsList, fmt.Sprintf("%s: %v", t.ID, aerr))
			} else if acted {
				succeeded++
				ids = append(ids, t.ID)
			}
		}

		resp := types.BulkByFilterResponse{
			Preview:      false,
			Criteria:     criteria,
			Matched:      len(matched),
			Acted:        succeeded,
			Failed:       failed,
			UpdatedTasks: ids,
			Errors:       errorsList,
		}
		text := fmt.Sprintf("Bulk-by-filter %s: %d acted, %d failed (%s)", args.Action, succeeded, failed, criteria)
		return &mcp.CallToolResultFor[types.BulkByFilterResponse]{Content: []mcp.Content{&mcp.TextContent{Text: text}}, StructuredContent: resp}, nil
	}
}
