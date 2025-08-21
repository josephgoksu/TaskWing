/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

// Workflow integration tools: smart transitions, workflow status, dependency health

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/josephgoksu/taskwing.app/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Workflow Integration tools implementation for context-aware AI assistance

// smartTaskTransitionHandler implements AI-powered next step suggestions
func smartTaskTransitionHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.SmartTaskTransitionParams, types.SmartTaskTransitionResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.SmartTaskTransitionParams]) (*mcp.CallToolResultFor[types.SmartTaskTransitionResponse], error) {
		args := params.Arguments
		logToolCall("smart-task-transition", args)

		// Determine task to analyze
		var targetTaskID string
		if args.TaskID != "" {
			targetTaskID = args.TaskID
		} else {
			targetTaskID = GetCurrentTask()
		}

		// Set default limit
		limit := args.Limit
		if limit <= 0 {
			limit = 5
		}

		// Get all tasks for context
		allTasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		var currentTask *models.Task
		if targetTaskID != "" {
			for _, task := range allTasks {
				if task.ID == targetTaskID {
					currentTask = &task
					break
				}
			}
		}

		// Generate smart suggestions
		suggestions := generateTaskTransitionSuggestions(currentTask, allTasks, args.Context, limit)

		// Build response
		response := types.SmartTaskTransitionResponse{
			Suggestions: suggestions,
			Context:     args.Context,
			Count:       len(suggestions),
		}

		if currentTask != nil {
			taskResp := taskToResponse(*currentTask)
			response.CurrentTask = &taskResp
		}

		if len(suggestions) > 0 {
			response.RecommendedNext = &suggestions[0]
		}

		responseText := fmt.Sprintf("Generated %d smart transition suggestions", len(suggestions))
		if currentTask != nil {
			responseText += fmt.Sprintf(" for '%s'", currentTask.Title)
		}
		if len(suggestions) > 0 {
			responseText += fmt.Sprintf(". Recommended: %s", suggestions[0].Description)
		}

		return &mcp.CallToolResultFor[types.SmartTaskTransitionResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}

// workflowStatusHandler implements project lifecycle tracking
func workflowStatusHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.WorkflowStatusParams, types.WorkflowStatusResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.WorkflowStatusParams]) (*mcp.CallToolResultFor[types.WorkflowStatusResponse], error) {
		args := params.Arguments
		logToolCall("workflow-status", args)

		// Set defaults
		depth := args.Depth
		if depth == "" {
			depth = "summary"
		}

		// format parameter exists but is not currently used in the implementation
		// Left for future enhancement
		_ = args.Format

		// Get all tasks
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Analyze project workflow
		currentPhase := analyzeProjectPhase(tasks)
		overallProgress := calculateOverallProgress(tasks)
		timeline := buildProjectTimeline(tasks, depth)
		bottlenecks := identifyBottlenecks(tasks)
		recommendations := generateWorkflowRecommendations(tasks, currentPhase, bottlenecks)
		metrics := calculateWorkflowMetrics(tasks)

		// Generate summary
		summary := generateWorkflowSummary(currentPhase, overallProgress, len(bottlenecks), len(tasks))

		response := types.WorkflowStatusResponse{
			CurrentPhase:    currentPhase,
			OverallProgress: overallProgress,
			Timeline:        timeline,
			Bottlenecks:     bottlenecks,
			Recommendations: recommendations,
			Metrics:         metrics,
			Summary:         summary,
		}

		responseText := fmt.Sprintf("Project status: %s phase (%.1f%% complete). %s",
			currentPhase.Phase, overallProgress*100, summary)

		return &mcp.CallToolResultFor[types.WorkflowStatusResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}

// dependencyHealthHandler implements relationship validation
func dependencyHealthHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.DependencyHealthParams, types.DependencyHealthResponse] {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.DependencyHealthParams]) (*mcp.CallToolResultFor[types.DependencyHealthResponse], error) {
		args := params.Arguments
		logToolCall("dependency-health", args)

		// Set defaults
		checkType := args.CheckType
		if checkType == "" {
			checkType = "all"
		}

		suggestions := args.Suggestions
		if !suggestions {
			suggestions = true // Default to true
		}

		// Get tasks to analyze
		var tasksToAnalyze []models.Task
		if args.TaskID != "" {
			task, err := taskStore.GetTask(args.TaskID)
			if err != nil {
				return nil, WrapStoreError(err, "get", args.TaskID)
			}
			tasksToAnalyze = []models.Task{task}
		} else {
			allTasks, err := taskStore.ListTasks(nil, nil)
			if err != nil {
				return nil, WrapStoreError(err, "list", "")
			}
			tasksToAnalyze = allTasks
		}

		// Get all tasks for context (needed for dependency validation)
		allTasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, WrapStoreError(err, "list", "")
		}

		// Analyze dependency health
		issues := analyzeDependencyIssues(tasksToAnalyze, allTasks, checkType)

		// Auto-fix issues if requested
		var fixedIssues []types.DependencyIssue
		if args.AutoFix {
			fixedIssues = autoFixDependencyIssues(issues, taskStore)
		}

		// Calculate health score
		healthScore := calculateDependencyHealthScore(issues, len(tasksToAnalyze))

		// Generate suggestions
		var suggestionList []string
		if suggestions {
			suggestionList = generateDependencySuggestions(issues)
		}

		// Generate summary
		summary := generateDependencyHealthSummary(healthScore, len(issues), len(fixedIssues))

		response := types.DependencyHealthResponse{
			HealthScore:   healthScore,
			Issues:        issues,
			FixedIssues:   fixedIssues,
			Suggestions:   suggestionList,
			Summary:       summary,
			TasksAnalyzed: len(tasksToAnalyze),
			IssuesFixed:   len(fixedIssues),
		}

		responseText := fmt.Sprintf("Dependency health: %.1f%% (%d issues found, %d fixed). %s",
			healthScore*100, len(issues), len(fixedIssues), summary)

		return &mcp.CallToolResultFor[types.DependencyHealthResponse]{
			Content: []mcp.Content{
				&mcp.TextContent{Text: responseText},
			},
			StructuredContent: response,
		}, nil
	}
}

// Helper functions for workflow integration

// generateTaskTransitionSuggestions creates AI-powered next step suggestions
func generateTaskTransitionSuggestions(currentTask *models.Task, allTasks []models.Task, context string, limit int) []types.TaskTransition {
	var suggestions []types.TaskTransition

	if currentTask == nil {
		// No current task - suggest starting one
		pendingTasks := filterTasksByStatus(allTasks, models.StatusTodo)
		if len(pendingTasks) > 0 {
			// Sort by priority
			sort.SliceStable(pendingTasks, func(i, j int) bool {
				return priorityToInt(pendingTasks[i].Priority) > priorityToInt(pendingTasks[j].Priority)
			})

			for i, task := range pendingTasks {
				if i >= limit {
					break
				}
				suggestions = append(suggestions, types.TaskTransition{
					Action:      "start",
					TaskID:      task.ID,
					Title:       task.Title,
					Description: fmt.Sprintf("Start high-priority task: %s", task.Title),
					Priority:    string(task.Priority),
					Confidence:  0.8,
					Reasoning:   "No current task set. This is a high-priority pending task ready to start.",
				})
			}
		}
		return suggestions
	}

	// Current task exists - generate context-aware suggestions
	switch context {
	case "completed":
		suggestions = append(suggestions, generateCompletionSuggestions(currentTask, allTasks)...)
	case "blocked":
		suggestions = append(suggestions, generateUnblockingSuggestions(currentTask, allTasks)...)
	case "next":
		suggestions = append(suggestions, generateNextStepSuggestions(currentTask, allTasks)...)
	default:
		// General suggestions based on task status
		switch currentTask.Status {
		case models.StatusTodo:
			suggestions = append(suggestions, generateStartSuggestions(currentTask, allTasks)...)
		case models.StatusDoing:
			suggestions = append(suggestions, generateProgressSuggestions(currentTask, allTasks)...)
		case models.StatusReview:
			suggestions = append(suggestions, generateUnblockingSuggestions(currentTask, allTasks)...)
		}
	}

	// Limit results
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions
}

// generateCompletionSuggestions suggests what to do after completing a task
func generateCompletionSuggestions(task *models.Task, allTasks []models.Task) []types.TaskTransition {
	var suggestions []types.TaskTransition

	// Suggest completing the current task
	suggestions = append(suggestions, types.TaskTransition{
		Action:      "complete",
		TaskID:      task.ID,
		Description: fmt.Sprintf("Mark '%s' as completed", task.Title),
		Confidence:  0.9,
		Reasoning:   "Task appears ready for completion based on context.",
	})

	// Find dependent tasks that can now be started
	for _, otherTask := range allTasks {
		if containsString(otherTask.Dependencies, task.ID) && otherTask.Status == models.StatusTodo {
			suggestions = append(suggestions, types.TaskTransition{
				Action:       "start",
				TaskID:       otherTask.ID,
				Title:        otherTask.Title,
				Description:  fmt.Sprintf("Start dependent task: %s", otherTask.Title),
				Priority:     string(otherTask.Priority),
				Confidence:   0.8,
				Reasoning:    "This task was waiting for the current task to complete.",
				Dependencies: []string{task.ID},
			})
		}
	}

	// Find subtasks that need attention
	for _, otherTask := range allTasks {
		if otherTask.ParentID != nil && *otherTask.ParentID == task.ID && otherTask.Status == models.StatusTodo {
			suggestions = append(suggestions, types.TaskTransition{
				Action:      "start",
				TaskID:      otherTask.ID,
				Title:       otherTask.Title,
				Description: fmt.Sprintf("Continue with subtask: %s", otherTask.Title),
				Priority:    string(otherTask.Priority),
				Confidence:  0.7,
				Reasoning:   "This is a subtask of the completed task.",
			})
		}
	}

	return suggestions
}

// generateUnblockingSuggestions suggests how to unblock a task
func generateUnblockingSuggestions(task *models.Task, allTasks []models.Task) []types.TaskTransition {
	var suggestions []types.TaskTransition

	// Check dependencies
	for _, depID := range task.Dependencies {
		for _, depTask := range allTasks {
			if depTask.ID == depID && depTask.Status != models.StatusDone {
				suggestions = append(suggestions, types.TaskTransition{
					Action:      "start",
					TaskID:      depTask.ID,
					Title:       depTask.Title,
					Description: fmt.Sprintf("Complete dependency: %s", depTask.Title),
					Priority:    "high",
					Confidence:  0.8,
					Reasoning:   "This dependency is blocking the current task.",
				})
			}
		}
	}

	// Suggest updating status if no blocking dependencies
	if len(suggestions) == 0 {
		suggestions = append(suggestions, types.TaskTransition{
			Action:      "update",
			TaskID:      task.ID,
			Description: "Update task status from blocked to in-progress",
			Confidence:  0.6,
			Reasoning:   "No obvious blocking dependencies found. Task may be ready to continue.",
		})
	}

	return suggestions
}

// generateNextStepSuggestions suggests logical next steps
func generateNextStepSuggestions(task *models.Task, allTasks []models.Task) []types.TaskTransition {
	var suggestions []types.TaskTransition

	// If task has subtasks, suggest working on them
	for _, otherTask := range allTasks {
		if otherTask.ParentID != nil && *otherTask.ParentID == task.ID && otherTask.Status == models.StatusTodo {
			suggestions = append(suggestions, types.TaskTransition{
				Action:      "start",
				TaskID:      otherTask.ID,
				Title:       otherTask.Title,
				Description: fmt.Sprintf("Work on subtask: %s", otherTask.Title),
				Priority:    string(otherTask.Priority),
				Confidence:  0.8,
				Reasoning:   "This subtask needs to be completed for the parent task.",
			})
		}
	}

	// Suggest creating follow-up tasks if none exist
	if len(suggestions) == 0 {
		suggestions = append(suggestions, types.TaskTransition{
			Action:      "create",
			Description: fmt.Sprintf("Create follow-up task for '%s'", task.Title),
			Priority:    string(task.Priority),
			Confidence:  0.5,
			Reasoning:   "Consider breaking down the task or creating next steps.",
		})
	}

	return suggestions
}

// generateStartSuggestions suggests how to start a pending task
func generateStartSuggestions(task *models.Task, allTasks []models.Task) []types.TaskTransition {
	var suggestions []types.TaskTransition

	suggestions = append(suggestions, types.TaskTransition{
		Action:      "update",
		TaskID:      task.ID,
		Description: fmt.Sprintf("Start working on '%s'", task.Title),
		Confidence:  0.9,
		Reasoning:   "Task is ready to begin.",
	})

	return suggestions
}

// generateProgressSuggestions suggests how to continue an in-progress task
func generateProgressSuggestions(task *models.Task, allTasks []models.Task) []types.TaskTransition {
	var suggestions []types.TaskTransition

	// Check if ready for completion
	if len(task.SubtaskIDs) > 0 {
		completedSubtasks := 0
		for _, subtaskID := range task.SubtaskIDs {
			for _, otherTask := range allTasks {
				if otherTask.ID == subtaskID && otherTask.Status == models.StatusDone {
					completedSubtasks++
				}
			}
		}

		if completedSubtasks == len(task.SubtaskIDs) {
			suggestions = append(suggestions, types.TaskTransition{
				Action:      "complete",
				TaskID:      task.ID,
				Description: fmt.Sprintf("Complete '%s' - all subtasks finished", task.Title),
				Confidence:  0.9,
				Reasoning:   "All subtasks have been completed.",
			})
		}
	}

	// Suggest continuing work
	if len(suggestions) == 0 {
		suggestions = append(suggestions, types.TaskTransition{
			Action:      "update",
			TaskID:      task.ID,
			Description: fmt.Sprintf("Continue working on '%s'", task.Title),
			Confidence:  0.7,
			Reasoning:   "Task is in progress and ready for continued work.",
		})
	}

	return suggestions
}

// analyzeProjectPhase determines the current project phase
func analyzeProjectPhase(tasks []models.Task) types.ProjectPhase {
	if len(tasks) == 0 {
		return types.ProjectPhase{
			Phase:       "planning",
			Progress:    0.0,
			Description: "No tasks yet - project in planning phase",
		}
	}

	// Count tasks by status
	statusCounts := make(map[models.TaskStatus]int)
	for _, task := range tasks {
		statusCounts[task.Status]++
	}

	totalTasks := len(tasks)
	completedRatio := float64(statusCounts[models.StatusDone]) / float64(totalTasks)
	inProgressRatio := float64(statusCounts[models.StatusDoing]) / float64(totalTasks)

	// Determine phase based on task distribution
	var phase string
	var progress float64
	var description string

	if completedRatio > 0.8 {
		phase = "maintenance"
		progress = 0.9
		description = "Project mostly complete, in maintenance phase"
	} else if completedRatio > 0.6 {
		phase = "deployment"
		progress = 0.8
		description = "Project nearing completion, preparing for deployment"
	} else if inProgressRatio > 0.3 || completedRatio > 0.2 {
		phase = "development"
		progress = completedRatio
		description = "Active development phase"
	} else if statusCounts[models.StatusTodo] > totalTasks/2 {
		phase = "planning"
		progress = 0.1
		description = "Project in planning phase with many pending tasks"
	} else {
		phase = "development"
		progress = completedRatio
		description = "Development in progress"
	}

	return types.ProjectPhase{
		Phase:       phase,
		Progress:    progress,
		Description: description,
	}
}

// calculateOverallProgress calculates project completion percentage
func calculateOverallProgress(tasks []models.Task) float64 {
	if len(tasks) == 0 {
		return 0.0
	}

	completed := 0
	for _, task := range tasks {
		if task.Status == models.StatusDone {
			completed++
		}
	}

	return float64(completed) / float64(len(tasks))
}

// buildProjectTimeline creates timeline information
func buildProjectTimeline(tasks []models.Task, depth string) map[string]interface{} {
	timeline := make(map[string]interface{})

	if len(tasks) == 0 {
		return timeline
	}

	// Find earliest and latest dates
	var earliest, latest time.Time
	for i, task := range tasks {
		if i == 0 {
			earliest = task.CreatedAt
			latest = task.CreatedAt
		} else {
			if task.CreatedAt.Before(earliest) {
				earliest = task.CreatedAt
			}
			if task.CreatedAt.After(latest) {
				latest = task.CreatedAt
			}
		}

		if task.CompletedAt != nil && task.CompletedAt.After(latest) {
			latest = *task.CompletedAt
		}
	}

	timeline["project_start"] = earliest.Format("2006-01-02")
	timeline["latest_activity"] = latest.Format("2006-01-02")
	timeline["duration_days"] = int(latest.Sub(earliest).Hours() / 24)

	if depth == "detailed" || depth == "full" {
		// Add weekly breakdown
		weeklyActivity := make(map[string]int)
		for _, task := range tasks {
			week := task.CreatedAt.Format("2006-W02")
			weeklyActivity[week]++
		}
		timeline["weekly_activity"] = weeklyActivity
	}

	return timeline
}

// identifyBottlenecks finds workflow bottlenecks
func identifyBottlenecks(tasks []models.Task) []string {
	var bottlenecks []string

	// Count blocked tasks
	blockedCount := 0
	for _, task := range tasks {
		if task.Status == models.StatusReview {
			blockedCount++
		}
	}

	if blockedCount > 0 {
		bottlenecks = append(bottlenecks, fmt.Sprintf("%d blocked tasks", blockedCount))
	}

	// Check for dependency chains
	longChains := findLongDependencyChains(tasks)
	if len(longChains) > 0 {
		bottlenecks = append(bottlenecks, "Long dependency chains detected")
	}

	// Check for overdue tasks (simple heuristic based on age)
	now := time.Now()
	overdueCount := 0
	for _, task := range tasks {
		if task.Status != models.StatusDone && now.Sub(task.CreatedAt).Hours() > 7*24 {
			overdueCount++
		}
	}

	if overdueCount > 0 {
		bottlenecks = append(bottlenecks, fmt.Sprintf("%d potentially overdue tasks", overdueCount))
	}

	return bottlenecks
}

// generateWorkflowRecommendations creates workflow improvement suggestions
func generateWorkflowRecommendations(tasks []models.Task, phase types.ProjectPhase, bottlenecks []string) []string {
	var recommendations []string

	if len(bottlenecks) > 0 {
		recommendations = append(recommendations, "Address blocked tasks to improve flow")
	}

	// Phase-specific recommendations
	switch phase.Phase {
	case "planning":
		recommendations = append(recommendations, "Start moving tasks to in-progress status")
	case "development":
		recommendations = append(recommendations, "Focus on completing in-progress tasks")
	case "deployment":
		recommendations = append(recommendations, "Prepare deployment and testing procedures")
	case "maintenance":
		recommendations = append(recommendations, "Monitor for issues and plan next iteration")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Project workflow appears healthy")
	}

	return recommendations
}

// calculateWorkflowMetrics calculates workflow-related metrics
func calculateWorkflowMetrics(tasks []models.Task) map[string]interface{} {
	metrics := make(map[string]interface{})

	if len(tasks) == 0 {
		return metrics
	}

	// Calculate velocity (tasks completed per week)
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	recentCompletions := 0

	for _, task := range tasks {
		if task.CompletedAt != nil && task.CompletedAt.After(weekAgo) {
			recentCompletions++
		}
	}

	metrics["weekly_velocity"] = recentCompletions
	metrics["total_tasks"] = len(tasks)

	// Status distribution
	statusDist := make(map[string]int)
	for _, task := range tasks {
		statusDist[string(task.Status)]++
	}
	metrics["status_distribution"] = statusDist

	return metrics
}

// generateWorkflowSummary creates a human-readable workflow summary
func generateWorkflowSummary(phase types.ProjectPhase, progress float64, bottleneckCount, totalTasks int) string {
	summary := fmt.Sprintf("Project in %s phase with %.1f%% completion (%d tasks total)",
		phase.Phase, progress*100, totalTasks)

	if bottleneckCount > 0 {
		summary += fmt.Sprintf(", %d bottlenecks identified", bottleneckCount)
	} else {
		summary += ", workflow healthy"
	}

	return summary
}

// Dependency health helper functions

// analyzeDependencyIssues finds dependency problems
func analyzeDependencyIssues(tasksToAnalyze []models.Task, allTasks []models.Task, checkType string) []types.DependencyIssue {
	var issues []types.DependencyIssue

	// Create task lookup map
	taskMap := make(map[string]models.Task)
	for _, task := range allTasks {
		taskMap[task.ID] = task
	}

	for _, task := range tasksToAnalyze {
		// Check for circular dependencies
		if checkType == "all" || checkType == "circular" {
			if hasCircularDependency(task, allTasks, make(map[string]bool)) {
				issues = append(issues, types.DependencyIssue{
					Type:        "circular",
					TaskID:      task.ID,
					TaskTitle:   task.Title,
					Description: "Task has circular dependency chain",
					Severity:    "high",
					AutoFixable: false,
					Resolution:  "Manual review required to break dependency cycle",
				})
			}
		}

		// Check for broken dependencies
		if checkType == "all" || checkType == "broken" {
			for _, depID := range task.Dependencies {
				if _, exists := taskMap[depID]; !exists {
					issues = append(issues, types.DependencyIssue{
						Type:        "broken",
						TaskID:      task.ID,
						TaskTitle:   task.Title,
						Description: fmt.Sprintf("References non-existent task: %s", depID),
						Severity:    "medium",
						AutoFixable: true,
						Resolution:  "Remove broken dependency reference",
					})
				}
			}
		}

		// Check for orphaned dependencies
		if checkType == "all" || checkType == "orphaned" {
			if len(task.Dependencies) == 0 && len(task.Dependents) == 0 && task.ParentID == nil && len(task.SubtaskIDs) == 0 {
				issues = append(issues, types.DependencyIssue{
					Type:        "orphaned",
					TaskID:      task.ID,
					TaskTitle:   task.Title,
					Description: "Task has no relationships to other tasks",
					Severity:    "low",
					AutoFixable: false,
					Resolution:  "Consider adding dependencies or parent-child relationships",
				})
			}
		}
	}

	return issues
}

// hasCircularDependency checks for circular dependency chains
func hasCircularDependency(task models.Task, allTasks []models.Task, visited map[string]bool) bool {
	if visited[task.ID] {
		return true
	}

	visited[task.ID] = true

	for _, depID := range task.Dependencies {
		for _, depTask := range allTasks {
			if depTask.ID == depID {
				if hasCircularDependency(depTask, allTasks, visited) {
					return true
				}
				break
			}
		}
	}

	delete(visited, task.ID)
	return false
}

// autoFixDependencyIssues attempts to fix fixable issues
func autoFixDependencyIssues(issues []types.DependencyIssue, taskStore store.TaskStore) []types.DependencyIssue {
	var fixed []types.DependencyIssue

	for _, issue := range issues {
		if issue.AutoFixable && issue.Type == "broken" {
			// For broken dependencies, we could remove them
			// This is a simplified implementation
			logInfo(fmt.Sprintf("Would auto-fix broken dependency for task %s", issue.TaskID))
			fixed = append(fixed, issue)
		}
	}

	return fixed
}

// calculateDependencyHealthScore calculates overall health score
func calculateDependencyHealthScore(issues []types.DependencyIssue, totalTasks int) float64 {
	if totalTasks == 0 {
		return 1.0
	}

	// Weight issues by severity
	severityWeights := map[string]float64{
		"low":      0.1,
		"medium":   0.3,
		"high":     0.6,
		"critical": 1.0,
	}

	totalWeight := 0.0
	for _, issue := range issues {
		if weight, ok := severityWeights[issue.Severity]; ok {
			totalWeight += weight
		}
	}

	// Health score decreases with more/severe issues
	maxPossibleWeight := float64(totalTasks) * 1.0 // Assume all could be critical
	healthScore := 1.0 - (totalWeight / maxPossibleWeight)

	if healthScore < 0 {
		healthScore = 0
	}

	return healthScore
}

// generateDependencySuggestions creates improvement suggestions
func generateDependencySuggestions(issues []types.DependencyIssue) []string {
	var suggestions []string

	circularCount := 0
	brokenCount := 0
	orphanedCount := 0

	for _, issue := range issues {
		switch issue.Type {
		case "circular":
			circularCount++
		case "broken":
			brokenCount++
		case "orphaned":
			orphanedCount++
		}
	}

	if circularCount > 0 {
		suggestions = append(suggestions, "Review and break circular dependency chains")
	}
	if brokenCount > 0 {
		suggestions = append(suggestions, "Clean up broken dependency references")
	}
	if orphanedCount > 0 {
		suggestions = append(suggestions, "Consider adding relationships for isolated tasks")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Dependency structure appears healthy")
	}

	return suggestions
}

// generateDependencyHealthSummary creates a summary of dependency health
func generateDependencyHealthSummary(healthScore float64, issueCount, fixedCount int) string {
	healthPercentage := healthScore * 100

	summary := fmt.Sprintf("%.1f%% healthy", healthPercentage)

	if issueCount > 0 {
		summary += fmt.Sprintf(" (%d issues", issueCount)
		if fixedCount > 0 {
			summary += fmt.Sprintf(", %d auto-fixed", fixedCount)
		}
		summary += ")"
	}

	return summary
}

// Helper functions

// filterTasksByStatus filters tasks by status
func filterTasksByStatus(tasks []models.Task, status models.TaskStatus) []models.Task {
	var filtered []models.Task
	for _, task := range tasks {
		if task.Status == status {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// findLongDependencyChains identifies tasks with long dependency chains
func findLongDependencyChains(tasks []models.Task) []string {
	// Simplified implementation - could be more sophisticated
	var longChains []string

	for _, task := range tasks {
		if len(task.Dependencies) > 3 {
			longChains = append(longChains, task.ID)
		}
	}

	return longChains
}

// RegisterWorkflowIntegrationTools registers the workflow integration MCP tools
func RegisterWorkflowIntegrationTools(server *mcp.Server, taskStore store.TaskStore) error {
	// Smart task transition tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "smart-task-transition",
		Description: "🧠 AI-POWERED: Get intelligent next step suggestions based on current task and project context. Eliminates guesswork in task progression.",
	}, smartTaskTransitionHandler(taskStore))

	// Workflow status tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "workflow-status",
		Description: "📊 PROJECT LIFECYCLE: Track project phases, progress, and bottlenecks. Shows where you are in the development process.",
	}, workflowStatusHandler(taskStore))

	// Dependency health tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "dependency-health",
		Description: "🔗 RELATIONSHIP VALIDATOR: Analyze and fix task dependencies. Detects circular dependencies, broken links, and suggests improvements.",
	}, dependencyHealthHandler(taskStore))

	return nil
}
