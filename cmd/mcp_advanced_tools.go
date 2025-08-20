/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/josephgoksu/taskwing.app/types"
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

		for _, taskID := range args.TaskIDs {
			var err error

			switch strings.ToLower(args.Action) {
			case "complete":
				_, err = taskStore.MarkTaskDone(taskID)
			case "cancel":
				_, err = taskStore.UpdateTask(taskID, map[string]interface{}{
					"status": models.StatusDone,
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

		// Pre-validate placeholder IDs but allow TempIDs for parent-child relationships
		for i, taskReq := range args.Tasks {
			if taskReq.ParentID != "" {
				// Check if it's a TempID (integer) - these are allowed
				if _, err := strconv.Atoi(taskReq.ParentID); err != nil {
					// Not a TempID, check for placeholder patterns
					if strings.HasPrefix(taskReq.ParentID, "task_") || 
					   strings.Contains(taskReq.ParentID, "placeholder") ||
					   !strings.Contains(taskReq.ParentID, "-") { // UUIDs always contain hyphens
						return nil, fmt.Errorf("task %d (%s): parentId '%s' appears to be a placeholder. Use list-tasks to get real UUID values like '7b3e4f2a-8c9d-4e5f-b0a1-2c3d4e5f6a7b', or use TempID (integer) for batch parent-child relationships", i+1, taskReq.Title, taskReq.ParentID)
					}
				}
			}
			// Also check dependencies for placeholder patterns
			for _, depID := range taskReq.Dependencies {
				if strings.HasPrefix(depID, "task_") || 
				   strings.Contains(depID, "placeholder") ||
				   !strings.Contains(depID, "-") {
					return nil, fmt.Errorf("task %d (%s): dependency '%s' appears to be a placeholder. Use list-tasks to get real UUID values", i+1, taskReq.Title, depID)
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

		return &mcp.CallToolResultFor[types.TaskListResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Found %d tasks matching search criteria", len(tasks)),
				},
			},
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
		Dependencies:       taskReq.Dependencies,
	}

	if taskReq.Priority != "" {
		task.Priority = models.TaskPriority(taskReq.Priority)
	} else {
		task.Priority = models.PriorityMedium
	}

	return taskStore.CreateTask(task)
}

// suggestPatternHandler suggests task patterns based on description
func suggestPatternHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.SuggestPatternParams, types.SuggestPatternResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.SuggestPatternParams]) (*mcp.CallToolResultFor[types.SuggestPatternResponse], error) {
		args := params.Arguments
		
		if args.Description == "" {
			return nil, types.NewMCPError("DESCRIPTION_REQUIRED", "Description is required to suggest patterns", nil)
		}
		
		cfg := GetConfig()
		
		// Load pattern library
		library, err := loadPatternLibrary(cfg)
		if err != nil {
			// Return empty suggestions if no pattern library exists
			response := types.SuggestPatternResponse{
				MatchingPatterns: []types.PatternSuggestion{},
				Suggestions:      "No pattern library found. Consider using 'taskwing patterns extract' to build patterns from archived projects.",
				LibraryStats: map[string]interface{}{
					"total_patterns": 0,
					"library_status": "not_found",
				},
			}
			
			return &mcp.CallToolResultFor[types.SuggestPatternResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{Text: response.Suggestions},
				},
				StructuredContent: response,
			}, nil
		}
		
		if len(library.Patterns) == 0 {
			response := types.SuggestPatternResponse{
				MatchingPatterns: []types.PatternSuggestion{},
				Suggestions:      "Pattern library is empty. Use 'taskwing patterns extract' to analyze archived projects.",
				LibraryStats: map[string]interface{}{
					"total_patterns": 0,
					"library_status": "empty",
				},
			}
			
			return &mcp.CallToolResultFor[types.SuggestPatternResponse]{
				Content: []mcp.Content{
					&mcp.TextContent{Text: response.Suggestions},
				},
				StructuredContent: response,
			}, nil
		}
		
		// Find matching patterns
		matches := findMatchingPatternsForMCP(args.Description, library.Patterns)
		
		// Convert internal patterns to MCP response format
		var suggestions []types.PatternSuggestion
		for _, match := range matches {
			suggestion := convertPatternToSuggestion(match.Pattern, match.Score)
			suggestions = append(suggestions, suggestion)
		}
		
		response := types.SuggestPatternResponse{
			MatchingPatterns: suggestions,
			LibraryStats: map[string]interface{}{
				"total_patterns":      len(library.Patterns),
				"most_used_pattern":   library.Statistics.MostUsedPattern,
				"average_success_rate": library.Statistics.AverageSuccessRate,
				"library_status":      "active",
			},
		}
		
		// Set best match
		if len(suggestions) > 0 {
			response.BestMatch = &suggestions[0]
		}
		
		// Generate helpful suggestions text
		if len(suggestions) == 0 {
			response.Suggestions = fmt.Sprintf("No patterns match '%s'. Consider these general approaches: %s", 
				args.Description, generateGeneralSuggestions(library))
		} else {
			bestMatch := suggestions[0]
			response.Suggestions = fmt.Sprintf("Best match: '%s' (%.0f%% confidence, %.0f%% success rate). Typical duration: %.1f hours. Consider following the %d-phase breakdown: %s",
				bestMatch.Name, 
				bestMatch.MatchScore*100,
				bestMatch.SuccessRate,
				bestMatch.AverageDuration,
				len(bestMatch.TaskBreakdown),
				generatePhaseNames(bestMatch.TaskBreakdown))
		}
		
		logInfo(fmt.Sprintf("Pattern suggestion for '%s': %d matches found", args.Description, len(suggestions)))
		
		return &mcp.CallToolResultFor[types.SuggestPatternResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: response.Suggestions},
			},
			StructuredContent: response,
		}, nil
	}
}

// Helper functions for pattern suggestion

func convertPatternToSuggestion(pattern TaskPattern, score float64) types.PatternSuggestion {
	suggestion := types.PatternSuggestion{
		PatternID:       pattern.PatternID,
		Name:           pattern.Name,
		MatchScore:     score,
		Category:       pattern.Category,
		Description:    pattern.Description,
		SuccessRate:    pattern.Metrics.SuccessRate,
		AverageDuration: pattern.Metrics.AverageDurationHours,
		SuccessFactors: pattern.SuccessFactors,
		WhenToUse:      pattern.WhenToUse,
	}
	
	// Convert task breakdown
	for _, phase := range pattern.TaskBreakdown {
		suggestion.TaskBreakdown = append(suggestion.TaskBreakdown, types.PatternPhase{
			Phase:                phase.Phase,
			Tasks:                phase.Tasks,
			TypicalDurationHours: phase.TypicalDurationHours,
			Priority:             phase.Priority,
		})
	}
	
	// Convert AI guidance
	suggestion.AIGuidance = types.PatternAIGuidance{
		TaskGenerationHints: pattern.AIGuidance.TaskGenerationHints,
		PrioritySuggestions: pattern.AIGuidance.PrioritySuggestions,
		DependencyPatterns:  pattern.AIGuidance.DependencyPatterns,
	}
	
	return suggestion
}

func findMatchingPatternsForMCP(description string, patterns []TaskPattern) []PatternMatch {
	// Reuse the pattern matching logic from patterns.go
	var matches []PatternMatch
	
	descLower := strings.ToLower(description)
	words := strings.Fields(descLower)
	
	for _, pattern := range patterns {
		score := calculatePatternMatchScore(descLower, words, pattern)
		if score > 0.1 { // Minimum threshold
			matches = append(matches, PatternMatch{
				Pattern: pattern,
				Score:   score,
			})
		}
	}
	
	// Sort by score descending
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].Score < matches[j].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
	
	// Limit to top 5 matches
	if len(matches) > 5 {
		matches = matches[:5]
	}
	
	return matches
}

func calculatePatternMatchScore(description string, words []string, pattern TaskPattern) float64 {
	score := 0.0
	maxScore := 0.0
	
	// Check name match
	maxScore += 1.0
	if strings.Contains(strings.ToLower(pattern.Name), description) {
		score += 1.0
	}
	
	// Check description match
	maxScore += 1.0
	if strings.Contains(strings.ToLower(pattern.Description), description) {
		score += 1.0
	}
	
	// Check tags
	maxScore += 1.0
	for _, tag := range pattern.Tags {
		if strings.Contains(description, strings.ToLower(tag)) {
			score += 1.0
			break
		}
	}
	
	// Check when to use criteria
	maxScore += 1.0
	for _, criteria := range pattern.WhenToUse {
		if containsAnyWordInText(strings.ToLower(criteria), words) {
			score += 1.0
			break
		}
	}
	
	// Category bonus
	maxScore += 0.5
	if strings.Contains(description, strings.ToLower(pattern.Category)) {
		score += 0.5
	}
	
	if maxScore == 0 {
		return 0
	}
	
	return score / maxScore
}

func containsAnyWordInText(text string, words []string) bool {
	for _, word := range words {
		if len(word) > 3 && strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func generateGeneralSuggestions(library *PatternLibrary) string {
	if len(library.Patterns) == 0 {
		return "No patterns available"
	}
	
	// Sort by success rate and show top patterns
	topPatterns := make([]TaskPattern, len(library.Patterns))
	copy(topPatterns, library.Patterns)
	
	for i := 0; i < len(topPatterns)-1; i++ {
		for j := i + 1; j < len(topPatterns); j++ {
			if topPatterns[i].Metrics.SuccessRate < topPatterns[j].Metrics.SuccessRate {
				topPatterns[i], topPatterns[j] = topPatterns[j], topPatterns[i]
			}
		}
	}
	
	suggestions := []string{}
	limit := 3
	if len(topPatterns) < limit {
		limit = len(topPatterns)
	}
	
	for i := 0; i < limit; i++ {
		pattern := topPatterns[i]
		suggestions = append(suggestions, fmt.Sprintf("%s (%.0f%% success)", pattern.Name, pattern.Metrics.SuccessRate))
	}
	
	return strings.Join(suggestions, ", ")
}

func generatePhaseNames(phases []types.PatternPhase) string {
	names := []string{}
	for _, phase := range phases {
		names = append(names, phase.Phase)
	}
	return strings.Join(names, " → ")
}

// RegisterAdvancedMCPTools registers additional MCP tools
func RegisterAdvancedMCPTools(server *mcp.Server, taskStore store.TaskStore) error {
	// Batch create tasks tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "batch-create-tasks",
		Description: "🎯 PREFERRED: Create multiple TaskWing tasks at once with parent-child relationships. Use this for task breakdown instead of creating separate todo items.",
	}, batchCreateTasksHandler(taskStore))

	// Bulk operations tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "bulk-tasks",
		Description: "🎯 PREFERRED: Perform bulk operations on TaskWing tasks (complete, cancel, delete, prioritize). More powerful than generic task operations.",
	}, bulkTaskHandler(taskStore))

	// Task summary tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "task-summary",
		Description: "🎯 ALWAYS USE: Get TaskWing project health summary with metrics and insights. Use this to understand project status before suggesting new tasks.",
	}, taskSummaryHandler(taskStore))

	// Advanced search tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search-tasks",
		Description: "🎯 PREFERRED: Advanced TaskWing search with logical operators and date ranges. More powerful than generic task search tools.",
	}, advancedSearchHandler(taskStore))

	// Pattern suggestion tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "suggest-patterns",
		Description: "🎯 AI ENHANCED: Suggest task patterns based on work description. Uses TaskWing's pattern library to recommend proven approaches with success rates and task breakdowns.",
	}, suggestPatternHandler(taskStore))

	return nil
}
