/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package mcp

// JSON processing tools: filter, extract ids, analytics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/taskutil"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// JSON Processing tools implementation replaces jq usage

// filterTasksHandler implements advanced filtering with JSONPath-style expressions
func filterTasksHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.FilterTasksParams, types.FilterTasksResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.FilterTasksParams]) (*mcpsdk.CallToolResultFor[types.FilterTasksResponse], error) {
		args := params.Arguments
		logToolCall("filter-tasks", args)

		startTime := time.Now()

		if strings.TrimSpace(args.Filter) == "" && strings.TrimSpace(args.Expression) == "" {
			return nil, types.NewMCPError("MISSING_FILTER", "Either filter or expression is required", nil)
		}

		// Get all tasks
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Apply filter
		var filteredTasks []models.Task
		filterUsed := args.Filter

		if args.Expression != "" {
			filterUsed = args.Expression
			// Prefer enhanced complex filter with helpful errors
			filteredTasks, _, err = applyEnhancedComplexFilter(tasks, args.Expression)
		} else {
			// Support multiple syntaxes: JSONPath ($.field == "value"), simple field=value or field:value,
			// and compact status==todo (normalize to status=todo)
			simple := strings.TrimSpace(args.Filter)
			if strings.Contains(simple, "==") && !strings.Contains(simple, "$.") {
				simple = strings.ReplaceAll(simple, "==", "=")
			}

			// Try simple first for common cases like status=todo, priority=high
			if ft, _, ferr := applyEnhancedSimpleFilter(tasks, simple); ferr == nil {
				filteredTasks = ft
				filterUsed = simple
			} else {
				// Fallback to JSONPath-style
				filteredTasks, err = applyJSONPathFilter(tasks, args.Filter)
				if err != nil {
					// Provide a more actionable error
					return nil, types.NewMCPError("FILTER_ERROR",
						fmt.Sprintf("Filter execution failed: %v", err), map[string]interface{}{
							"examples": []string{"status=todo", "priority:high", "$.status == 'todo'"},
							"tip":      "Use field=value or JSONPath (e.g., $.status == 'todo').",
						})
				}
			}
		}

		if err != nil {
			return nil, types.NewMCPError("FILTER_ERROR", fmt.Sprintf("Filter execution failed: %v", err), nil)
		}

		// Apply limit
		if args.Limit > 0 && len(filteredTasks) > args.Limit {
			filteredTasks = filteredTasks[:args.Limit]
		}

		// Convert to response format with field filtering
		var taskResponses []types.TaskResponse
		var fieldsReturned []string

		if args.Fields != "" {
			fields := strings.Split(args.Fields, ",")
			for i := range fields {
				fields[i] = strings.TrimSpace(fields[i])
			}
			fieldsReturned = fields

			taskResponses = make([]types.TaskResponse, len(filteredTasks))
			for i, task := range filteredTasks {
				taskResponses[i] = taskToResponseWithFields(task, fields)
			}
		} else {
			taskResponses = make([]types.TaskResponse, len(filteredTasks))
			for i, task := range filteredTasks {
				taskResponses[i] = taskToResponse(task)
			}
		}

		executionTime := time.Since(startTime).Milliseconds()

		response := types.FilterTasksResponse{
			Tasks:       taskResponses,
			Count:       len(taskResponses),
			Filter:      filterUsed,
			Fields:      fieldsReturned,
			ExecutionMs: executionTime,
		}

		responseText := fmt.Sprintf("Filtered %d tasks using '%s' (executed in %dms)",
			len(taskResponses), filterUsed, executionTime)

		return &mcpsdk.CallToolResultFor[types.FilterTasksResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}

// extractTaskIDsHandler implements bulk ID extraction with criteria
func extractTaskIDsHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.ExtractTaskIDsParams, types.ExtractTaskIDsResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ExtractTaskIDsParams]) (*mcpsdk.CallToolResultFor[types.ExtractTaskIDsResponse], error) {
		args := params.Arguments
		logToolCall("extract-task-ids", args)

		startTime := time.Now()

		// Set default format
		format := args.Format
		if format == "" {
			format = "array"
		}

		// Build filter criteria
		var criteriaUsed []string
		// Pre-normalize priority if provided
		normalizedPrio := ""
		if args.Priority != "" {
			if canon, err := taskutil.NormalizePriorityString(args.Priority); err == nil {
				normalizedPrio = canon
			} else {
				normalizedPrio = strings.ToLower(args.Priority)
			}
		}

		filterFn := func(task models.Task) bool {
			// Status filter
			if args.Status != "" {
				if string(task.Status) != args.Status {
					return false
				}
			}

			// Priority filter
			if args.Priority != "" {
				if strings.ToLower(string(task.Priority)) != normalizedPrio {
					return false
				}
			}

			// Search filter
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

		// Build criteria description
		if args.Status != "" {
			criteriaUsed = append(criteriaUsed, fmt.Sprintf("status=%s", args.Status))
		}
		if args.Priority != "" {
			crit := normalizedPrio
			if crit == "" {
				crit = args.Priority
			}
			criteriaUsed = append(criteriaUsed, fmt.Sprintf("priority=%s", crit))
		}
		if args.Search != "" {
			criteriaUsed = append(criteriaUsed, fmt.Sprintf("search=%s", args.Search))
		}
		if len(criteriaUsed) == 0 {
			criteriaUsed = append(criteriaUsed, "all_tasks")
		}

		// Get filtered tasks
		tasks, err := taskStore.ListTasks(filterFn, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Extract IDs (and optional refs)
		taskIDs := make([]string, len(tasks))
		refs := make([]types.TaskRef, 0, len(tasks))
		for i, task := range tasks {
			taskIDs[i] = task.ID
			if format == "objects" || format == "refs" {
				refs = append(refs, types.TaskRef{ID: task.ID, Title: task.Title})
			}
		}

		executionTime := time.Since(startTime).Milliseconds()

		response := types.ExtractTaskIDsResponse{
			TaskIDs:     taskIDs,
			Refs:        refs,
			Count:       len(taskIDs),
			Format:      format,
			Criteria:    strings.Join(criteriaUsed, ", "),
			ExecutionMs: executionTime,
		}

		responseText := fmt.Sprintf("Extracted %d task IDs with criteria: %s (executed in %dms)",
			len(taskIDs), response.Criteria, executionTime)
		if format == "objects" || format == "refs" {
			responseText += ". Includes id/title pairs."
		}

		return &mcpsdk.CallToolResultFor[types.ExtractTaskIDsResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}

// taskAnalyticsHandler implements aggregation and statistics
func taskAnalyticsHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.TaskAnalyticsParams, types.TaskAnalyticsResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.TaskAnalyticsParams]) (*mcpsdk.CallToolResultFor[types.TaskAnalyticsResponse], error) {
		args := params.Arguments
		logToolCall("task-analytics", args)

		startTime := time.Now()

		// Set defaults
		groupBy := args.GroupBy
		if groupBy == "" {
			groupBy = "status"
		}

		dateRange := args.DateRange
		if dateRange == "" {
			dateRange = "all"
		}

		// format parameter exists but is not currently used in the implementation
		// Left for future enhancement
		_ = args.Format

		// Get tasks within date range
		tasks, err := getTasksInDateRange(taskStore, dateRange)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Calculate metrics
		metrics := calculateTaskMetrics(tasks, args.Metrics)
		groups := groupTasksBy(tasks, groupBy)

		executionTime := time.Since(startTime).Milliseconds()

		// Generate summary
		summary := generateAnalyticsSummary(metrics, groups, len(tasks), dateRange)

		response := types.TaskAnalyticsResponse{
			Summary:     summary,
			Metrics:     metrics,
			Groups:      groups,
			DateRange:   dateRange,
			ExecutionMs: executionTime,
		}

		responseText := fmt.Sprintf("Analytics for %d tasks (%s): %s (executed in %dms)",
			len(tasks), dateRange, summary, executionTime)

		return &mcpsdk.CallToolResultFor[types.TaskAnalyticsResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}

// Helper functions for JSON processing

// applyJSONPathFilter applies JSONPath-style filtering
func applyJSONPathFilter(tasks []models.Task, filter string) ([]models.Task, error) {
	var filtered []models.Task

	// Simple JSONPath implementation for common patterns
	filter = strings.TrimSpace(filter)

	// Handle $.field == "value" pattern
	if strings.Contains(filter, "$.") && strings.Contains(filter, "==") {
		parts := strings.Split(filter, "==")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid filter format")
		}

		fieldPath := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		// Extract field name
		field := strings.TrimPrefix(fieldPath, "$.")

		for _, task := range tasks {
			if matchesJSONPathFilter(task, field, value) {
				filtered = append(filtered, task)
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported filter format: %s", filter)
	}

	return filtered, nil
}

// matchesJSONPathFilter checks if task matches JSONPath filter
func matchesJSONPathFilter(task models.Task, field, value string) bool {
	switch field {
	case "status":
		return string(task.Status) == value
	case "priority":
		if canon, err := taskutil.NormalizePriorityString(value); err == nil && canon != "" {
			return string(task.Priority) == canon
		}
		return strings.EqualFold(string(task.Priority), value)
	case "title":
		return strings.Contains(strings.ToLower(task.Title), strings.ToLower(value))
	case "description":
		return strings.Contains(strings.ToLower(task.Description), strings.ToLower(value))
	case "id":
		return task.ID == value
	default:
		return false
	}
}

// taskToResponseWithFields converts task to response with specific fields
func taskToResponseWithFields(task models.Task, fields []string) types.TaskResponse {
	full := taskToResponse(task)

	// For simplicity, return full response
	// In production, this could use reflection to filter fields
	return full
}

// getTasksInDateRange filters tasks by date range
func getTasksInDateRange(taskStore store.TaskStore, dateRange string) ([]models.Task, error) {
	now := time.Now()
	var startDate time.Time

	switch dateRange {
	case "today":
		startDate = now.Truncate(24 * time.Hour)
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	case "all":
		// No date filtering
		return taskStore.ListTasks(nil, nil)
	default:
		return taskStore.ListTasks(nil, nil)
	}

	filterFn := func(task models.Task) bool {
		return task.CreatedAt.After(startDate)
	}

	return taskStore.ListTasks(filterFn, nil)
}

// calculateTaskMetrics calculates requested metrics
func calculateTaskMetrics(tasks []models.Task, metricsStr string) map[string]interface{} {
	metrics := make(map[string]interface{})

	if metricsStr == "" {
		metricsStr = "count"
	}

	requestedMetrics := strings.Split(metricsStr, ",")
	for i := range requestedMetrics {
		requestedMetrics[i] = strings.TrimSpace(requestedMetrics[i])
	}

	for _, metric := range requestedMetrics {
		switch metric {
		case "count":
			metrics["total_count"] = len(tasks)
		case "duration":
			metrics["average_duration_days"] = calculateAverageDuration(tasks)
		case "completion_rate":
			metrics["completion_rate"] = calculateCompletionRate(tasks)
		}
	}

	return metrics
}

// groupTasksBy groups tasks by specified field
func groupTasksBy(tasks []models.Task, groupBy string) map[string]interface{} {
	groups := make(map[string]interface{})

	switch groupBy {
	case "status":
		statusGroups := make(map[string]int)
		for _, task := range tasks {
			statusGroups[string(task.Status)]++
		}
		groups["by_status"] = statusGroups
	case "priority":
		priorityGroups := make(map[string]int)
		for _, task := range tasks {
			priorityGroups[string(task.Priority)]++
		}
		groups["by_priority"] = priorityGroups
	case "created_date":
		dateGroups := make(map[string]int)
		for _, task := range tasks {
			date := task.CreatedAt.Format("2006-01-02")
			dateGroups[date]++
		}
		groups["by_created_date"] = dateGroups
	}

	return groups
}

// calculateAverageDuration calculates average task duration
func calculateAverageDuration(tasks []models.Task) float64 {
	if len(tasks) == 0 {
		return 0
	}

	var totalDays float64
	completedCount := 0

	for _, task := range tasks {
		if task.CompletedAt != nil {
			duration := task.CompletedAt.Sub(task.CreatedAt)
			totalDays += duration.Hours() / 24
			completedCount++
		}
	}

	if completedCount == 0 {
		return 0
	}

	return totalDays / float64(completedCount)
}

// calculateCompletionRate calculates task completion rate
func calculateCompletionRate(tasks []models.Task) float64 {
	if len(tasks) == 0 {
		return 0
	}

	completed := 0
	for _, task := range tasks {
		if task.Status == models.StatusDone {
			completed++
		}
	}

	return float64(completed) / float64(len(tasks)) * 100
}

// generateAnalyticsSummary generates a human-readable summary
func generateAnalyticsSummary(metrics map[string]interface{}, groups map[string]interface{}, totalTasks int, dateRange string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%d tasks", totalTasks))

	if completionRate, ok := metrics["completion_rate"]; ok {
		if rate, ok := completionRate.(float64); ok {
			parts = append(parts, fmt.Sprintf("%.1f%% completion rate", rate))
		}
	}

	if avgDuration, ok := metrics["average_duration_days"]; ok {
		if duration, ok := avgDuration.(float64); ok {
			parts = append(parts, fmt.Sprintf("%.1f days avg duration", duration))
		}
	}

	return strings.Join(parts, ", ")
}

// RegisterJSONProcessingTools registers the JSON processing MCP tools
func RegisterJSONProcessingTools(server *mcpsdk.Server, taskStore store.TaskStore) error {
	// Filter tasks tool
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "filter-tasks",
		Description: "⚙️ TECHNICAL FILTER (for developers): JSONPath or expression-based filtering. Examples: $.status==\"todo\"; priority=urgent. Returns raw JSON data.",
	}, filterTasksHandler(taskStore))

	// Extract task IDs tool
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "extract-task-ids",
		Description: "Extract only task IDs with simple criteria. Args: status, priority, search, format [array|string|newline]. Returns ids+count.",
	}, extractTaskIDsHandler(taskStore))

	// Task analytics tool
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "task-analytics",
		Description: "Compute metrics. Args: metrics [count,duration,completion_rate], group_by [status|priority|created_date], date_range [today|week|month|all].",
	}, taskAnalyticsHandler(taskStore))

	return nil
}
