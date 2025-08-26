/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

// Resolution tools: resolve references, fuzzy title search, autocomplete

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Task resolution tools implementation uses types directly

// findTaskByTitleHandler implements fuzzy title matching
func findTaskByTitleHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.FindTaskByTitleParams, types.FindTaskByTitleResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.FindTaskByTitleParams]) (*mcp.CallToolResultFor[types.FindTaskByTitleResponse], error) {
		args := params.Arguments
		logToolCall("find-task-by-title", args)

		if strings.TrimSpace(args.Title) == "" {
			return nil, types.NewMCPError("MISSING_TITLE", "Title is required for task search", nil)
		}

		// Set default limit
		limit := args.Limit
		if limit <= 0 {
			limit = 5
		}

		// Get all tasks
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Find matches using fuzzy string matching
		matches := findTaskMatches(args.Title, tasks, "title")

		// Sort by score descending
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].Score > matches[j].Score
		})

		// Limit results
		if len(matches) > limit {
			matches = matches[:limit]
		}

		response := types.FindTaskByTitleResponse{
			Matches: matches,
			Query:   args.Title,
			Count:   len(matches),
		}

		responseText := fmt.Sprintf("Found %d tasks matching '%s'", len(matches), args.Title)
		if len(matches) > 0 {
			responseText += fmt.Sprintf(". Best: '%s' [%s] (%.1f%%)",
				matches[0].Task.Title, shortID(matches[0].Task.ID), matches[0].Score*100)
			// Append compact list
			maxShow := 5
			if len(matches) < maxShow {
				maxShow = len(matches)
			}
			responseText += "\nTop:"
			for i := 0; i < maxShow; i++ {
				responseText += fmt.Sprintf("\n - %s [%s] (%.1f%%)", matches[i].Task.Title, shortID(matches[i].Task.ID), matches[i].Score*100)
			}
		}

		return &mcp.CallToolResultFor[types.FindTaskByTitleResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}

// resolveTaskReferenceHandler implements smart task resolution
func resolveTaskReferenceHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.ResolveTaskReferenceParams, types.ResolveTaskReferenceResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ResolveTaskReferenceParams]) (*mcp.CallToolResultFor[types.ResolveTaskReferenceResponse], error) {
		args := params.Arguments
		logToolCall("resolve-task-reference", args)

		if strings.TrimSpace(args.Reference) == "" {
			return nil, types.NewMCPError("MISSING_REFERENCE", "Reference is required for task resolution", nil)
		}

		reference := strings.TrimSpace(args.Reference)

		// Get all tasks
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		response := types.ResolveTaskReferenceResponse{
			Reference: reference,
			Resolved:  false,
		}

		// Try exact ID match first
		if task := findTaskByID(reference, tasks); task != nil {
			response.Match = &types.TaskMatch{
				Task:  taskToResponse(*task),
				Score: 1.0,
				Type:  "id",
			}
			response.Resolved = true
			response.Message = fmt.Sprintf("Exact ID match found: %s", task.Title)
		} else if !args.Exact {
			// Try fuzzy matching on all fields
			var allMatches []types.TaskMatch

			// Try partial ID match
			if len(reference) >= 8 { // Minimum meaningful UUID portion
				idMatches := findTaskMatches(reference, tasks, "id")
				allMatches = append(allMatches, idMatches...)
			}

			// Try title matching
			titleMatches := findTaskMatches(reference, tasks, "title")
			allMatches = append(allMatches, titleMatches...)

			// Try description matching
			descMatches := findTaskMatches(reference, tasks, "description")
			allMatches = append(allMatches, descMatches...)

			// Remove duplicates and sort by score
			uniqueMatches := removeDuplicateMatches(allMatches)
			sort.SliceStable(uniqueMatches, func(i, j int) bool {
				return uniqueMatches[i].Score > uniqueMatches[j].Score
			})

			if len(uniqueMatches) > 0 {
				if uniqueMatches[0].Score > 0.8 && len(uniqueMatches) == 1 {
					// High confidence single match
					response.Match = &uniqueMatches[0]
					response.Resolved = true
					response.Message = fmt.Sprintf("High confidence: %s [%s] (%.1f%%)",
						uniqueMatches[0].Task.Title, shortID(uniqueMatches[0].Task.ID), uniqueMatches[0].Score*100)
				} else {
					// Multiple matches or lower confidence
					response.Matches = uniqueMatches
					if len(uniqueMatches) > 5 {
						response.Matches = uniqueMatches[:5]
					}
					// Include compact list of top suggestions
					msg := fmt.Sprintf("Found %d potential matches", len(response.Matches))
					maxShow := len(response.Matches)
					if maxShow > 5 {
						maxShow = 5
					}
					msg += "\nTop:"
					for i := 0; i < maxShow; i++ {
						m := response.Matches[i]
						msg += fmt.Sprintf("\n - %s [%s] (%.1f%%)", m.Task.Title, shortID(m.Task.ID), m.Score*100)
					}
					response.Message = msg
				}
			} else {
				response.Message = fmt.Sprintf("No tasks found matching '%s'", reference)
			}
		} else {
			response.Message = fmt.Sprintf("No exact match found for '%s'", reference)
		}

		return &mcp.CallToolResultFor[types.ResolveTaskReferenceResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: response.Message},
			},
			StructuredContent: response,
		}, nil
	}
}

// taskAutocompleteHandler implements predictive task suggestions
func taskAutocompleteHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.TaskAutocompleteParams, types.TaskAutocompleteResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.TaskAutocompleteParams]) (*mcp.CallToolResultFor[types.TaskAutocompleteResponse], error) {
		args := params.Arguments
		logToolCall("task-autocomplete", args)

		if strings.TrimSpace(args.Input) == "" {
			return nil, types.NewMCPError("MISSING_INPUT", "Input is required for autocomplete", nil)
		}

		// Set default limit
		limit := args.Limit
		if limit <= 0 {
			limit = 10
		}

		// Get all tasks
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Filter based on context if provided
		filteredTasks := tasks
		if args.Context != "" {
			filteredTasks = filterTasksByContext(tasks, args.Context)
		}

		// Find autocomplete suggestions
		suggestions := findAutocompleteSuggestions(args.Input, filteredTasks, limit)

		response := types.TaskAutocompleteResponse{
			Suggestions: suggestions,
			Input:       args.Input,
			Count:       len(suggestions),
		}

		responseText := fmt.Sprintf("Found %d autocomplete suggestions for '%s'", len(suggestions), args.Input)

		return &mcp.CallToolResultFor[types.TaskAutocompleteResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}

// Helper functions

// findTaskMatches performs fuzzy matching on tasks
func findTaskMatches(query string, tasks []models.Task, matchType string) []types.TaskMatch {
	var matches []types.TaskMatch
	queryLower := strings.ToLower(query)

	for _, task := range tasks {
		var score float64
		var text string

		switch matchType {
		case "title":
			text = strings.ToLower(task.Title)
		case "description":
			text = strings.ToLower(task.Description)
		case "id":
			text = strings.ToLower(task.ID)
		default:
			continue
		}

		score = calculateFuzzyScore(queryLower, text)
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

// calculateFuzzyScore calculates similarity between query and text
func calculateFuzzyScore(query, text string) float64 {
	if query == text {
		return 1.0
	}

	if strings.Contains(text, query) {
		// Exact substring match
		return 0.9 - (float64(len(text)-len(query)) / float64(len(text)) * 0.3)
	}

	// Check for partial word matches
	queryWords := strings.Fields(query)
	textWords := strings.Fields(text)

	matchedWords := 0
	for _, qWord := range queryWords {
		for _, tWord := range textWords {
			if strings.Contains(tWord, qWord) || strings.Contains(qWord, tWord) {
				matchedWords++
				break
			}
		}
	}

	if matchedWords > 0 {
		return float64(matchedWords) / float64(len(queryWords)) * 0.7
	}

	// Check for character similarity
	return calculateCharacterSimilarity(query, text)
}

// calculateCharacterSimilarity uses a simple character-based similarity
func calculateCharacterSimilarity(s1, s2 string) float64 {
	if len(s1) == 0 || len(s2) == 0 {
		return 0
	}

	// Simple Levenshtein-inspired approach
	matches := 0
	for _, char := range s1 {
		if strings.ContainsRune(s2, char) {
			matches++
		}
	}

	similarity := float64(matches) / float64(len(s1))
	if similarity > 0.5 {
		return similarity * 0.5 // Scale down character-only matches
	}
	return 0
}

// findTaskByID finds a task by exact ID match
func findTaskByID(id string, tasks []models.Task) *models.Task {
	for _, task := range tasks {
		if task.ID == id {
			return &task
		}
	}
	return nil
}

// removeDuplicateMatches removes duplicate tasks from matches
func removeDuplicateMatches(matches []types.TaskMatch) []types.TaskMatch {
	seen := make(map[string]bool)
	var unique []types.TaskMatch

	for _, match := range matches {
		if !seen[match.Task.ID] {
			seen[match.Task.ID] = true
			unique = append(unique, match)
		} else {
			// Keep the match with higher score
			for i, existing := range unique {
				if existing.Task.ID == match.Task.ID && match.Score > existing.Score {
					unique[i] = match
					break
				}
			}
		}
	}

	return unique
}

// filterTasksByContext filters tasks based on context
func filterTasksByContext(tasks []models.Task, context string) []models.Task {
	switch strings.ToLower(context) {
	case "current":
		// Return only current and related tasks
		currentTaskID := GetCurrentTask()
		if currentTaskID == "" {
			return tasks
		}

		var filtered []models.Task
		for _, task := range tasks {
			if task.ID == currentTaskID ||
				(task.ParentID != nil && *task.ParentID == currentTaskID) ||
				containsString(task.Dependencies, currentTaskID) ||
				containsString(task.Dependents, currentTaskID) {
				filtered = append(filtered, task)
			}
		}
		return filtered
	case "active":
		var filtered []models.Task
		for _, task := range tasks {
			if task.Status == models.StatusTodo || task.Status == models.StatusDoing {
				filtered = append(filtered, task)
			}
		}
		return filtered
	case "recent":
		// Return tasks created or updated in the last 7 days
		// For now, just return all tasks - could implement date filtering
		return tasks
	default:
		return tasks
	}
}

// findAutocompleteSuggestions finds autocomplete suggestions
func findAutocompleteSuggestions(input string, tasks []models.Task, limit int) []types.TaskMatch {
	// Combine title and description matches
	titleMatches := findTaskMatches(input, tasks, "title")
	descMatches := findTaskMatches(input, tasks, "description")

	allMatches := append(titleMatches, descMatches...)
	uniqueMatches := removeDuplicateMatches(allMatches)

	// Sort by score
	sort.SliceStable(uniqueMatches, func(i, j int) bool {
		return uniqueMatches[i].Score > uniqueMatches[j].Score
	})

	if len(uniqueMatches) > limit {
		uniqueMatches = uniqueMatches[:limit]
	}

	return uniqueMatches
}

// containsString checks if slice contains string
func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// RegisterTaskResolutionTools registers the task resolution MCP tools
func RegisterTaskResolutionTools(server *mcp.Server, taskStore store.TaskStore) error {
	// Find task by title tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "find-task-by-title",
		Description: "Find tasks by fuzzy title match. Prefer 'find-task' for partial IDs and context-aware ranking.",
	}, findTaskByTitleHandler(taskStore))

	// Resolve task reference tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "resolve-task-reference",
		Description: "Resolve a task ref from partial ID/title/description. Returns a single resolved task or guidance if ambiguous.",
	}, resolveTaskReferenceHandler(taskStore))

	// Task autocomplete tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "task-autocomplete",
		Description: "Autocomplete titles by partial input. Returns suggestions; use 'suggest-tasks' for context-aware ranking.",
	}, taskAutocompleteHandler(taskStore))

	return nil
}
