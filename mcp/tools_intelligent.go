/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package mcp

// Intelligent MCP tools: query, smart find, suggest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/taskutil"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Intelligent query handler with natural language support and advanced filtering
func queryTasksHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.FilterTasksParams, types.FilterTasksResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.FilterTasksParams]) (*mcpsdk.CallToolResultFor[types.FilterTasksResponse], error) {
		args := params.Arguments
		logToolCall("query-tasks", args)

		startTime := time.Now()

		// Enhanced validation with helpful examples
		if strings.TrimSpace(args.Filter) == "" && strings.TrimSpace(args.Expression) == "" && strings.TrimSpace(args.Query) == "" {
			return nil, types.NewMCPError("MISSING_FILTER", "At least one filter type is required", map[string]interface{}{
				"available_types": []string{"filter (simple)", "expression (complex)", "query (natural language)"},
				"examples": map[string]string{
					"filter":     "status=todo",
					"expression": "status=todo AND priority=high",
					"query":      "high priority unfinished tasks",
				},
				"help": "Use 'filter' for simple key=value filtering, 'expression' for complex logic, or 'query' for natural language",
			})
		}

		// Get all tasks
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Determine query type and apply appropriate filter
		var filteredTasks []models.Task
		var filterUsed string
		var queryType string
		var matchedFields map[string][]string

		if args.Query != "" {
			queryType = "natural"
			filterUsed = args.Query
			filteredTasks, matchedFields, err = applyNaturalLanguageFilter(tasks, args.Query, args.FuzzyMatch)
		} else if args.Expression != "" {
			queryType = "complex"
			filterUsed = args.Expression
			filteredTasks, matchedFields, err = applyEnhancedComplexFilter(tasks, args.Expression)
		} else {
			queryType = "simple"
			filterUsed = args.Filter
			filteredTasks, matchedFields, err = applyEnhancedSimpleFilter(tasks, args.Filter)
		}

		if err != nil {
			// Provide helpful error with suggestions
			errorDetails := map[string]interface{}{
				"query_type":  queryType,
				"filter_used": filterUsed,
				"error":       err.Error(),
			}

			// Add context-sensitive suggestions
			suggestions := generateErrorSuggestions(err.Error(), filterUsed, tasks)
			if len(suggestions) > 0 {
				errorDetails["suggestions"] = suggestions
			}

			return nil, types.NewMCPError("FILTER_ERROR", fmt.Sprintf("Filter execution failed: %v", err), errorDetails)
		}

		// Generate suggestions if no results found
		var suggestions []string
		if len(filteredTasks) == 0 {
			suggestions = generateFilterSuggestions(tasks, filterUsed, queryType)
		}

		// Apply limit
		if args.Limit > 0 && len(filteredTasks) > args.Limit {
			filteredTasks = filteredTasks[:args.Limit]
		}

		// Convert to response format
		taskResponses := make([]types.TaskResponse, len(filteredTasks))
		var fieldsReturned []string

		if args.Fields != "" {
			fields := strings.Split(args.Fields, ",")
			for i := range fields {
				fields[i] = strings.TrimSpace(fields[i])
			}
			fieldsReturned = fields

			for i, task := range filteredTasks {
				taskResponses[i] = taskToResponseWithFields(task, fields)
			}
		} else {
			for i, task := range filteredTasks {
				taskResponses[i] = taskToResponse(task)
			}
		}

		executionTime := time.Since(startTime).Milliseconds()

		response := types.FilterTasksResponse{
			Tasks:         taskResponses,
			Count:         len(taskResponses),
			Filter:        filterUsed,
			Fields:        fieldsReturned,
			ExecutionMs:   executionTime,
			MatchedFields: matchedFields,
			Suggestions:   suggestions,
			QueryType:     queryType,
		}

		// Create informative response text
		responseText := fmt.Sprintf("Filtered %d tasks using %s query '%s' (executed in %dms)",
			len(taskResponses), queryType, filterUsed, executionTime)

		// Show a compact list of top matches with short IDs for quick reuse
		if len(taskResponses) > 0 {
			maxShow := 5
			if len(taskResponses) < maxShow {
				maxShow = len(taskResponses)
			}
			responseText += "\nTop:"
			for i := 0; i < maxShow; i++ {
				responseText += fmt.Sprintf("\n - %s [%s]", taskResponses[i].Title, taskutil.ShortID(taskResponses[i].ID))
			}
		}

		if len(suggestions) > 0 {
			responseText += fmt.Sprintf(". No results found - try: %s", strings.Join(suggestions, ", "))
		} else if len(matchedFields) > 0 {
			// Add insight about what was matched
			var matchInfo []string
			for field, values := range matchedFields {
				if len(values) > 0 {
					matchInfo = append(matchInfo, fmt.Sprintf("%s: %s", field, strings.Join(values[:min(len(values), 3)], ", ")))
				}
			}
			if len(matchInfo) > 0 {
				responseText += fmt.Sprintf(". Matched in: %s", strings.Join(matchInfo, "; "))
			}
		}

		// Get context for enriched response
		context, _ := BuildTaskContext(taskStore)
		if context != nil {
			responseText = EnrichToolResponse(responseText, context)
		}

		return &mcpsdk.CallToolResultFor[types.FilterTasksResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}

// Smart task finder with fuzzy matching and intelligent suggestions
func findTaskHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.ResolveTaskReferenceParams, types.ResolveTaskReferenceResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ResolveTaskReferenceParams]) (*mcpsdk.CallToolResultFor[types.ResolveTaskReferenceResponse], error) {
		args := params.Arguments
		logToolCall("resolve-task-reference", args)

		if strings.TrimSpace(args.Reference) == "" {
			return nil, types.NewMCPError("MISSING_REFERENCE", "Reference is required for task resolution", map[string]interface{}{
				"examples": []string{
					"Full UUID: 7b3e4f2a-8c9d-4e5f-b0a1-2c3d4e5f6a7b",
					"Partial ID: 7b3e4f2a",
					"Task title: 'implement user authentication'",
					"Description text: 'database migration'",
				},
			})
		}

		reference := strings.TrimSpace(args.Reference)
		maxSuggestions := args.MaxSuggestions
		if maxSuggestions <= 0 {
			maxSuggestions = 5
		}

		// Get all tasks
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		response := types.ResolveTaskReferenceResponse{
			Reference: reference,
			Resolved:  false,
		}

		// Step 1: Try exact ID match first
		if exactTask := findTaskByExactID(reference, tasks); exactTask != nil {
			response.Match = &types.TaskMatch{
				Task:  taskToResponse(*exactTask),
				Score: 1.0,
				Type:  "exact_id",
			}
			response.Resolved = true
			response.Message = fmt.Sprintf("✓ Exact ID match: %s", exactTask.Title)

			return &mcpsdk.CallToolResultFor[types.ResolveTaskReferenceResponse]{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: response.Message},
				},
				StructuredContent: response,
			}, nil
		}

		if !args.Exact {
			// Step 2: Apply intelligent fuzzy matching
			var allMatches []types.TaskMatch

			// Determine which fields to search
			searchFields := args.Fields
			if len(searchFields) == 0 {
				searchFields = []string{"id", "title", "description"}
			}

			// Try partial ID match if reference looks like UUID fragment
			if len(reference) >= 8 && isUUIDFragment(reference) {
				if contains(searchFields, "id") {
					idMatches := findTasksByPartialID(reference, tasks)
					allMatches = append(allMatches, idMatches...)
				}
			}

			// Try title matching
			if contains(searchFields, "title") {
				titleMatches := findSmartTaskMatches(reference, tasks, "title", true) // Always use fuzzy for smart matching
				allMatches = append(allMatches, titleMatches...)
			}

			// Try description matching
			if contains(searchFields, "description") {
				descMatches := findSmartTaskMatches(reference, tasks, "description", true) // Always use fuzzy for smart matching
				allMatches = append(allMatches, descMatches...)
			}

			// Remove duplicates and sort by score
			uniqueMatches := removeDuplicateMatches(allMatches)
			sort.SliceStable(uniqueMatches, func(i, j int) bool {
				return uniqueMatches[i].Score > uniqueMatches[j].Score
			})

			// Prefer current task and related tasks if requested
			if args.PreferCurrent {
				uniqueMatches = boostCurrentTaskMatches(uniqueMatches)
			}

			if len(uniqueMatches) > 0 {
				if uniqueMatches[0].Score > 0.8 {
					// High confidence match
					response.Match = &uniqueMatches[0]
					response.Resolved = true
					response.Message = fmt.Sprintf("✓ High confidence match: %s [%s] (%.1f%%, matched by %s)",
						uniqueMatches[0].Task.Title, taskutil.ShortID(uniqueMatches[0].Task.ID), uniqueMatches[0].Score*100, uniqueMatches[0].Type)
				} else {
					// Multiple matches or lower confidence
					if len(uniqueMatches) > maxSuggestions {
						uniqueMatches = uniqueMatches[:maxSuggestions]
					}
					response.Matches = uniqueMatches
					response.Message = fmt.Sprintf("Found %d potential matches for '%s'. Top: %s [%s] (%.1f%%)",
						len(uniqueMatches), reference, uniqueMatches[0].Task.Title, taskutil.ShortID(uniqueMatches[0].Task.ID), uniqueMatches[0].Score*100)
				}
			} else {
				// No matches found - provide helpful suggestions
				suggestions := generateNoMatchSuggestions(reference, tasks)
				response.Message = fmt.Sprintf("❌ No tasks found matching '%s'. %s", reference, strings.Join(suggestions, " "))
			}
		} else {
			response.Message = fmt.Sprintf("❌ No exact match found for '%s'. Try without 'exact=true' for fuzzy matching.", reference)
		}

		return &mcpsdk.CallToolResultFor[types.ResolveTaskReferenceResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: response.Message},
			},
			StructuredContent: response,
		}, nil
	}
}

// Intelligent task suggestions with context awareness
func suggestTasksHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.TaskAutocompleteParams, types.TaskAutocompleteResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.TaskAutocompleteParams]) (*mcpsdk.CallToolResultFor[types.TaskAutocompleteResponse], error) {
		args := params.Arguments
		logToolCall("task-autocomplete", args)

		if strings.TrimSpace(args.Input) == "" {
			return nil, types.NewMCPError("MISSING_INPUT", "Input is required for autocomplete", map[string]interface{}{
				"examples": []string{
					"'impl' → suggests tasks with 'implement' in title",
					"'auth' → suggests authentication-related tasks",
					"'test' → suggests testing tasks",
				},
			})
		}

		limit := args.Limit
		if limit <= 0 {
			limit = 10
		}

		// Get all tasks
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Filter based on context
		filteredTasks := tasks
		if args.Context != "" {
			filteredTasks = filterTasksBySmartContext(tasks, args.Context)
		}

		// Find intelligent autocomplete suggestions
		suggestions := findSmartAutocompleteSuggestions(args.Input, filteredTasks, limit)

		response := types.TaskAutocompleteResponse{
			Suggestions: suggestions,
			Input:       args.Input,
			Count:       len(suggestions),
		}

		responseText := fmt.Sprintf("Found %d autocomplete suggestions for '%s'", len(suggestions), args.Input)
		if len(suggestions) > 0 {
			responseText += fmt.Sprintf(". Top: %s [%s] (%.1f%%)",
				suggestions[0].Task.Title, taskutil.ShortID(suggestions[0].Task.ID), suggestions[0].Score*100)
		}

		if args.Context != "" {
			responseText += fmt.Sprintf(" (context: %s)", args.Context)
		}

		return &mcpsdk.CallToolResultFor[types.TaskAutocompleteResponse]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}

// Helper functions for intelligent filtering

// applyNaturalLanguageFilter processes natural language queries
func applyNaturalLanguageFilter(tasks []models.Task, query string, fuzzyMatch bool) ([]models.Task, map[string][]string, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	matchedFields := make(map[string][]string)
	var filtered []models.Task

	// Natural language patterns
	patterns := map[string]func(models.Task) bool{
		// Status patterns
		"unfinished|incomplete|todo|pending": func(t models.Task) bool {
			return t.Status == models.StatusTodo
		},
		"in progress|doing|working on|current": func(t models.Task) bool {
			return t.Status == models.StatusDoing
		},
		"review|needs review|waiting": func(t models.Task) bool {
			return t.Status == models.StatusReview
		},
		"done|completed|finished": func(t models.Task) bool {
			return t.Status == models.StatusDone
		},
		// Priority patterns
		"urgent|critical|emergency": func(t models.Task) bool {
			return t.Priority == models.PriorityUrgent
		},
		"high priority|important": func(t models.Task) bool {
			return t.Priority == models.PriorityHigh
		},
		"low priority|minor": func(t models.Task) bool {
			return t.Priority == models.PriorityLow
		},
		// Combined patterns
		"high priority.*unfinished|unfinished.*high priority": func(t models.Task) bool {
			return t.Priority == models.PriorityHigh && (t.Status == models.StatusTodo || t.Status == models.StatusDoing)
		},
	}

	// Check patterns
	for pattern, checkFunc := range patterns {
		if matchesPattern(query, pattern) {
			for _, task := range tasks {
				if checkFunc(task) {
					filtered = append(filtered, task)
					addToMatchedFields(matchedFields, "pattern", pattern)
				}
			}
			if len(filtered) > 0 {
				return filtered, matchedFields, nil
			}
		}
	}

	// Fallback to fuzzy text search if no patterns matched
	for _, task := range tasks {
		score := calculateAdvancedFuzzyScore(query, task, fuzzyMatch)
		if score > 0.3 {
			filtered = append(filtered, task)
			addTaskToMatchedFields(matchedFields, task, query)
		}
	}

	// Sort by relevance if using fuzzy matching
	if fuzzyMatch {
		sort.SliceStable(filtered, func(i, j int) bool {
			scoreI := calculateAdvancedFuzzyScore(query, filtered[i], fuzzyMatch)
			scoreJ := calculateAdvancedFuzzyScore(query, filtered[j], fuzzyMatch)
			return scoreI > scoreJ
		})
	}

	return filtered, matchedFields, nil
}

// applyEnhancedSimpleFilter provides better simple filtering
func applyEnhancedSimpleFilter(tasks []models.Task, filter string) ([]models.Task, map[string][]string, error) {
	matchedFields := make(map[string][]string)
	filter = strings.TrimSpace(filter)

	// Parse key=value or field:value format
	var key, value string
	if strings.Contains(filter, "=") {
		parts := strings.SplitN(filter, "=", 2)
		key = strings.TrimSpace(parts[0])
		value = strings.TrimSpace(parts[1])
	} else if strings.Contains(filter, ":") {
		parts := strings.SplitN(filter, ":", 2)
		key = strings.TrimSpace(parts[0])
		value = strings.TrimSpace(parts[1])
	} else {
		return nil, nil, fmt.Errorf("invalid filter format. Use 'field=value' or 'field:value'. Valid fields: %s",
			strings.Join([]string{"status", "priority", "title", "description", "id"}, ", "))
	}

	// Remove quotes if present
	value = strings.Trim(value, "\"'")

	var filtered []models.Task
	for _, task := range tasks {
		if matchesEnhancedFilter(task, key, value) {
			filtered = append(filtered, task)
			addToMatchedFields(matchedFields, key, value)
		}
	}

	if len(filtered) == 0 && len(tasks) > 0 {
		// Suggest valid values
		return nil, nil, fmt.Errorf("no tasks match %s=%s. %s", key, value, getValidValuesSuggestion(key, tasks))
	}

	return filtered, matchedFields, nil
}

// applyEnhancedComplexFilter provides better complex filtering with helpful errors
func applyEnhancedComplexFilter(tasks []models.Task, expression string) ([]models.Task, map[string][]string, error) {
	matchedFields := make(map[string][]string)
	expression = strings.TrimSpace(expression)

	var filtered []models.Task

	if strings.Contains(expression, " AND ") {
		parts := strings.Split(expression, " AND ")
		for _, task := range tasks {
			matches := true
			for _, part := range parts {
				if !evaluateEnhancedExpression(task, strings.TrimSpace(part)) {
					matches = false
					break
				}
			}
			if matches {
				filtered = append(filtered, task)
				for _, part := range parts {
					addExpressionToMatchedFields(matchedFields, part)
				}
			}
		}
	} else if strings.Contains(expression, " OR ") {
		parts := strings.Split(expression, " OR ")
		for _, task := range tasks {
			for _, part := range parts {
				if evaluateEnhancedExpression(task, strings.TrimSpace(part)) {
					filtered = append(filtered, task)
					addExpressionToMatchedFields(matchedFields, part)
					break
				}
			}
		}
	} else {
		// Single expression
		for _, task := range tasks {
			if evaluateEnhancedExpression(task, expression) {
				filtered = append(filtered, task)
				addExpressionToMatchedFields(matchedFields, expression)
			}
		}
	}

	return filtered, matchedFields, nil
}

// Enhanced helper functions

func matchesPattern(query, pattern string) bool {
	patterns := strings.Split(pattern, "|")
	for _, p := range patterns {
		if strings.Contains(query, strings.TrimSpace(p)) {
			return true
		}
	}
	return false
}

func calculateAdvancedFuzzyScore(query string, task models.Task, fuzzyMatch bool) float64 {
	query = strings.ToLower(query)

	// Exact matches get highest score
	title := strings.ToLower(task.Title)
	desc := strings.ToLower(task.Description)

	if strings.Contains(title, query) {
		return 0.9 + (float64(len(query)) / float64(len(title)) * 0.1)
	}

	if strings.Contains(desc, query) {
		return 0.7 + (float64(len(query)) / float64(len(desc)) * 0.1)
	}

	if !fuzzyMatch {
		return 0
	}

	// Word-level fuzzy matching
	queryWords := strings.Fields(query)
	titleWords := strings.Fields(title)
	descWords := strings.Fields(desc)

	titleMatches := countWordMatches(queryWords, titleWords)
	descMatches := countWordMatches(queryWords, descWords)

	if titleMatches > 0 {
		return 0.5 + (float64(titleMatches) / float64(len(queryWords)) * 0.3)
	}

	if descMatches > 0 {
		return 0.3 + (float64(descMatches) / float64(len(queryWords)) * 0.2)
	}

	return 0
}

func countWordMatches(queryWords, textWords []string) int {
	matches := 0
	for _, qWord := range queryWords {
		for _, tWord := range textWords {
			if strings.Contains(tWord, qWord) || strings.Contains(qWord, tWord) {
				matches++
				break
			}
		}
	}
	return matches
}

func matchesEnhancedFilter(task models.Task, key, value string) bool {
	switch strings.ToLower(key) {
	case "status":
		return strings.EqualFold(string(task.Status), value)
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
		return strings.Contains(strings.ToLower(task.ID), strings.ToLower(value))
	default:
		return false
	}
}

func evaluateEnhancedExpression(task models.Task, expr string) bool {
	// Support multiple comparison operators
	operators := []string{"==", "=", "!=", "~="}

	for _, op := range operators {
		if strings.Contains(expr, op) {
			parts := strings.SplitN(expr, op, 2)
			if len(parts) == 2 {
				field := strings.TrimSpace(parts[0])
				value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

				switch op {
				case "==", "=":
					return matchesEnhancedFilter(task, field, value)
				case "!=":
					return !matchesEnhancedFilter(task, field, value)
				case "~=":
					// Fuzzy match
					return strings.Contains(strings.ToLower(getTaskFieldValue(task, field)), strings.ToLower(value))
				}
			}
		}
	}

	return false
}

func getTaskFieldValue(task models.Task, field string) string {
	switch strings.ToLower(field) {
	case "status":
		return string(task.Status)
	case "priority":
		return string(task.Priority)
	case "title":
		return task.Title
	case "description":
		return task.Description
	case "id":
		return task.ID
	default:
		return ""
	}
}

func findTaskByExactID(id string, tasks []models.Task) *models.Task {
	for i, task := range tasks {
		if task.ID == id {
			return &tasks[i]
		}
	}
	return nil
}

func isUUIDFragment(s string) bool {
	// Check if string looks like a UUID fragment (hex characters and maybe hyphens)
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') && r != '-' {
			return false
		}
	}
	return true
}

func findTasksByPartialID(partialID string, tasks []models.Task) []types.TaskMatch {
	var matches []types.TaskMatch
	partialID = strings.ToLower(partialID)

	for _, task := range tasks {
		taskID := strings.ToLower(task.ID)
		if strings.HasPrefix(taskID, partialID) {
			score := 0.9 - (float64(len(taskID)-len(partialID)) / float64(len(taskID)) * 0.2)
			matches = append(matches, types.TaskMatch{
				Task:  taskToResponse(task),
				Score: score,
				Type:  "partial_id",
			})
		}
	}

	return matches
}

func findSmartTaskMatches(query string, tasks []models.Task, matchType string, fuzzyMatch bool) []types.TaskMatch {
	var matches []types.TaskMatch
	query = strings.ToLower(query)

	for _, task := range tasks {
		var score float64
		var text string

		switch matchType {
		case "title":
			text = strings.ToLower(task.Title)
		case "description":
			text = strings.ToLower(task.Description)
		default:
			continue
		}

		if text == "" {
			continue
		}

		score = calculateEnhancedMatchScore(query, text, fuzzyMatch)
		if score > 0.1 { // Minimum threshold
			matches = append(matches, types.TaskMatch{
				Task:  taskToResponse(task),
				Score: score,
				Type:  matchType,
			})
		}
	}

	return matches
}

func calculateEnhancedMatchScore(query, text string, fuzzyMatch bool) float64 {
	if query == text {
		return 1.0
	}

	// Exact substring match
	if strings.Contains(text, query) {
		return 0.9 - (float64(len(text)-len(query)) / float64(len(text)) * 0.3)
	}

	if !fuzzyMatch {
		return 0
	}

	// Word-level matching
	queryWords := strings.Fields(query)
	textWords := strings.Fields(text)

	if len(queryWords) == 0 || len(textWords) == 0 {
		return 0
	}

	matchedWords := countWordMatches(queryWords, textWords)
	if matchedWords > 0 {
		return float64(matchedWords) / float64(len(queryWords)) * 0.7
	}

	return 0
}

func boostCurrentTaskMatches(matches []types.TaskMatch) []types.TaskMatch {
	currentTaskID := currentTaskID()
	if currentTaskID == "" {
		return matches
	}

	for i := range matches {
		if matches[i].Task.ID == currentTaskID {
			matches[i].Score = math.Min(matches[i].Score*1.2, 1.0)
		} else if matches[i].Task.ParentID != nil && *matches[i].Task.ParentID == currentTaskID {
			matches[i].Score = math.Min(matches[i].Score*1.1, 1.0)
		}
	}

	// Re-sort after boosting
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches
}

func filterTasksBySmartContext(tasks []models.Task, context string) []models.Task {
	switch strings.ToLower(context) {
	case "current", "active":
		currentTaskID := currentTaskID()
		if currentTaskID == "" {
			// Return active tasks if no current task
			var active []models.Task
			for _, task := range tasks {
				if task.Status == models.StatusTodo || task.Status == models.StatusDoing {
					active = append(active, task)
				}
			}
			return active
		}

		var filtered []models.Task
		for _, task := range tasks {
			if task.ID == currentTaskID ||
				(task.ParentID != nil && *task.ParentID == currentTaskID) ||
				contains(task.Dependencies, currentTaskID) ||
				contains(task.Dependents, currentTaskID) {
				filtered = append(filtered, task)
			}
		}
		return filtered

	case "recent":
		// Return tasks created or updated in the last 7 days
		cutoff := time.Now().AddDate(0, 0, -7)
		var recent []models.Task
		for _, task := range tasks {
			if task.CreatedAt.After(cutoff) || task.UpdatedAt.After(cutoff) {
				recent = append(recent, task)
			}
		}
		return recent

	case "priority", "urgent":
		var priority []models.Task
		for _, task := range tasks {
			if task.Priority == models.PriorityHigh || task.Priority == models.PriorityUrgent {
				priority = append(priority, task)
			}
		}
		return priority

	default:
		return tasks
	}
}

func findSmartAutocompleteSuggestions(input string, tasks []models.Task, limit int) []types.TaskMatch {
	var allMatches []types.TaskMatch

	// Try different matching strategies
	titleMatches := findSmartTaskMatches(input, tasks, "title", true)
	descMatches := findSmartTaskMatches(input, tasks, "description", true)

	allMatches = append(allMatches, titleMatches...)
	allMatches = append(allMatches, descMatches...)

	// Remove duplicates and boost relevance
	uniqueMatches := removeDuplicateMatches(allMatches)

	// Boost matches based on task status (prefer active tasks)
	for i := range uniqueMatches {
		if uniqueMatches[i].Task.Status == "todo" || uniqueMatches[i].Task.Status == "doing" {
			uniqueMatches[i].Score *= 1.2
		}
	}

	// Sort by score
	sort.SliceStable(uniqueMatches, func(i, j int) bool {
		return uniqueMatches[i].Score > uniqueMatches[j].Score
	})

	if len(uniqueMatches) > limit {
		uniqueMatches = uniqueMatches[:limit]
	}

	return uniqueMatches
}

// Error handling and suggestion helpers

func generateErrorSuggestions(errorMsg, filter string, tasks []models.Task) []string {
	var suggestions []string

	if strings.Contains(errorMsg, "unknown field") || strings.Contains(errorMsg, "invalid filter") {
		suggestions = append(suggestions, "Valid fields: status, priority, title, description, id")
		suggestions = append(suggestions, "Valid statuses: todo, doing, review, done")
		suggestions = append(suggestions, "Valid priorities: low, medium, high, urgent")
	}

	if strings.Contains(errorMsg, "no tasks match") {
		// Extract field from filter to provide specific suggestions
		if strings.Contains(filter, "status=") {
			suggestions = append(suggestions, getAvailableStatuses(tasks)...)
		}
		if strings.Contains(filter, "priority=") {
			suggestions = append(suggestions, getAvailablePriorities(tasks)...)
		}
	}

	return suggestions
}

func generateFilterSuggestions(tasks []models.Task, filter, queryType string) []string {
	var suggestions []string

	if len(tasks) == 0 {
		return []string{"No tasks available to filter"}
	}

	// Get available values
	statuses := getUniqueStatuses(tasks)
	priorities := getUniquePriorities(tasks)

	switch queryType {
	case "natural":
		suggestions = append(suggestions, "try: 'high priority tasks'", "'unfinished tasks'", "'tasks in progress'")
	case "simple":
		if len(statuses) > 0 {
			suggestions = append(suggestions, fmt.Sprintf("status=%s", statuses[0]))
		}
		if len(priorities) > 0 {
			suggestions = append(suggestions, fmt.Sprintf("priority=%s", priorities[0]))
		}
	case "complex":
		if len(statuses) > 0 && len(priorities) > 0 {
			suggestions = append(suggestions, fmt.Sprintf("status=%s AND priority=%s", statuses[0], priorities[0]))
		}
	}

	return suggestions
}

func generateNoMatchSuggestions(reference string, tasks []models.Task) []string {
	var suggestions []string

	// If it looks like a UUID fragment, suggest checking the full ID
	if len(reference) >= 8 && isUUIDFragment(reference) {
		suggestions = append(suggestions, "Try using the full task ID or use fuzzy matching.")
	}

	// If it looks like a title search, suggest similar titles
	if len(reference) > 2 {
		similar := findSimilarTitles(reference, tasks, 3)
		if len(similar) > 0 {
			suggestions = append(suggestions, fmt.Sprintf("Similar tasks: %s", strings.Join(similar, ", ")))
		}
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Use 'list-tasks' to see all available tasks.")
	}

	return suggestions
}

func getValidValuesSuggestion(field string, tasks []models.Task) string {
	switch strings.ToLower(field) {
	case "status":
		statuses := getUniqueStatuses(tasks)
		return fmt.Sprintf("Available statuses: %s", strings.Join(statuses, ", "))
	case "priority":
		priorities := getUniquePriorities(tasks)
		return fmt.Sprintf("Available priorities: %s", strings.Join(priorities, ", "))
	default:
		return "Use 'list-tasks' to see available values"
	}
}

func findSimilarTitles(reference string, tasks []models.Task, limit int) []string {
	var similar []string
	reference = strings.ToLower(reference)

	for _, task := range tasks {
		title := strings.ToLower(task.Title)
		if strings.Contains(title, reference) || strings.Contains(reference, title) {
			similar = append(similar, fmt.Sprintf("'%s'", task.Title))
			if len(similar) >= limit {
				break
			}
		}
	}

	return similar
}

// Utility functions

func getUniqueStatuses(tasks []models.Task) []string {
	seen := make(map[string]bool)
	var statuses []string

	for _, task := range tasks {
		status := string(task.Status)
		if !seen[status] {
			seen[status] = true
			statuses = append(statuses, status)
		}
	}

	return statuses
}

func getUniquePriorities(tasks []models.Task) []string {
	seen := make(map[string]bool)
	var priorities []string

	for _, task := range tasks {
		priority := string(task.Priority)
		if !seen[priority] {
			seen[priority] = true
			priorities = append(priorities, priority)
		}
	}

	return priorities
}

func getAvailableStatuses(tasks []models.Task) []string {
	statuses := getUniqueStatuses(tasks)
	var suggestions []string
	for _, status := range statuses {
		suggestions = append(suggestions, fmt.Sprintf("Try: status=%s", status))
	}
	return suggestions
}

func getAvailablePriorities(tasks []models.Task) []string {
	priorities := getUniquePriorities(tasks)
	var suggestions []string
	for _, priority := range priorities {
		suggestions = append(suggestions, fmt.Sprintf("Try: priority=%s", priority))
	}
	return suggestions
}

func addToMatchedFields(matchedFields map[string][]string, field, value string) {
	if matchedFields[field] == nil {
		matchedFields[field] = []string{}
	}
	if !contains(matchedFields[field], value) {
		matchedFields[field] = append(matchedFields[field], value)
	}
}

func addTaskToMatchedFields(matchedFields map[string][]string, task models.Task, query string) {
	if strings.Contains(strings.ToLower(task.Title), query) {
		addToMatchedFields(matchedFields, "title", task.Title)
	}
	if strings.Contains(strings.ToLower(task.Description), query) {
		addToMatchedFields(matchedFields, "description", task.Description)
	}
}

func addExpressionToMatchedFields(matchedFields map[string][]string, expr string) {
	// Parse the expression to extract field and value
	if strings.Contains(expr, "=") {
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) == 2 {
			field := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			addToMatchedFields(matchedFields, field, value)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RegisterIntelligentMCPTools registers intelligent MCP tools with natural language support
func RegisterIntelligentMCPTools(server *mcpsdk.Server, taskStore store.TaskStore) error {
	// Intelligent query tasks tool
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "query-tasks",
		Description: "🔍 NATURAL LANGUAGE SEARCH (preferred for general search): Examples: 'high priority unfinished', 'what needs review'. Supports fuzzy matching and smart interpretation.",
	}, queryTasksHandler(taskStore))

	// Smart task finding tool
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "find-task",
		Description: "🔍 FIND SINGLE TASK (use for specific task lookup): Find by partial ID, fuzzy title, or description. Handles typos. Best for: 'find task abc123' or 'find login task'.",
	}, findTaskHandler(taskStore))

	// Task suggestion tool
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "suggest-tasks",
		Description: "🤖 AI-POWERED SUGGESTIONS: Get relevant task recommendations based on context and input. Returns ranked suggestions with confidence scores.",
	}, suggestTasksHandler(taskStore))

	return nil
}
