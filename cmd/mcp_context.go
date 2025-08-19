/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/josephgoksu/taskwing.app/types"
)

// Type aliases for backward compatibility
type TaskContext = types.TaskContext
type ActivityEntry = types.ActivityEntry
type ProjectMetrics = types.ProjectMetrics

// BuildTaskContext creates a comprehensive context for AI tools
func BuildTaskContext(taskStore store.TaskStore) (*types.TaskContext, error) {
	tasks, err := taskStore.ListTasks(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks for context: %w", err)
	}

	context := &types.TaskContext{
		TotalTasks:      len(tasks),
		TasksByStatus:   make(map[string]int),
		TasksByPriority: make(map[string]int),
		RecentActivity:  []types.ActivityEntry{},
		Suggestions:     []string{},
	}

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	var totalAge float64
	var completedTasks int
	var tasksCompletedThisWeek int
	var tasksCreatedThisWeek int

	// Analyze tasks
	for _, task := range tasks {
		// Count by status
		context.TasksByStatus[string(task.Status)]++

		// Count by priority
		context.TasksByPriority[string(task.Priority)]++

		// Check for overdue tasks (assuming tasks older than 30 days in pending are overdue)
		if task.Status == models.StatusPending && now.Sub(task.CreatedAt) > 30*24*time.Hour {
			context.OverdueTasks++
		}

		// Count blocked tasks
		if task.Status == models.StatusBlocked {
			context.BlockedTasks++
		}

		// Calculate age
		totalAge += now.Sub(task.CreatedAt).Hours() / 24

		// Count completed tasks
		if task.Status == models.StatusCompleted {
			completedTasks++
			if task.CompletedAt != nil && task.CompletedAt.After(weekAgo) {
				tasksCompletedThisWeek++
			}
		}

		// Count recently created tasks
		if task.CreatedAt.After(weekAgo) {
			tasksCreatedThisWeek++
		}

		// Track recent activity
		if task.UpdatedAt.After(now.AddDate(0, 0, -3)) {
			action := "updated"
			if task.CreatedAt.Equal(task.UpdatedAt) {
				action = "created"
			} else if task.Status == models.StatusCompleted && task.CompletedAt != nil && task.CompletedAt.Equal(task.UpdatedAt) {
				action = "completed"
			}

			context.RecentActivity = append(context.RecentActivity, types.ActivityEntry{
				Timestamp:   task.UpdatedAt,
				TaskID:      task.ID,
				TaskTitle:   task.Title,
				Action:      action,
				Description: fmt.Sprintf("Task %s was %s", task.Title, action),
			})
		}
	}

	// Sort recent activity by timestamp (most recent first)
	sort.Slice(context.RecentActivity, func(i, j int) bool {
		return context.RecentActivity[i].Timestamp.After(context.RecentActivity[j].Timestamp)
	})

	// Limit to 10 most recent activities
	if len(context.RecentActivity) > 10 {
		context.RecentActivity = context.RecentActivity[:10]
	}

	// Calculate metrics
	if len(tasks) > 0 {
		context.Metrics.CompletionRate = float64(completedTasks) / float64(len(tasks)) * 100
		context.Metrics.AverageTaskAge = totalAge / float64(len(tasks))
	}
	context.Metrics.TasksCompletedWeek = tasksCompletedThisWeek
	context.Metrics.TasksCreatedWeek = tasksCreatedThisWeek

	// Determine velocity trend
	if tasksCompletedThisWeek > tasksCreatedThisWeek {
		context.Metrics.VelocityTrend = "increasing"
	} else if tasksCompletedThisWeek < tasksCreatedThisWeek {
		context.Metrics.VelocityTrend = "decreasing"
	} else {
		context.Metrics.VelocityTrend = "stable"
	}

	// Add current task information
	currentTaskID := GetCurrentTask()
	if currentTaskID != "" {
		if currentTask, err := taskStore.GetTask(currentTaskID); err == nil {
			context.CurrentTask = taskToResponsePtr(currentTask)
		}
	}

	// Generate project health assessment
	context.ProjectHealth = assessProjectHealth(context)

	// Generate suggestions
	context.Suggestions = generateSuggestions(context)

	return context, nil
}

// assessProjectHealth provides an overall health assessment
func assessProjectHealth(ctx *types.TaskContext) string {
	score := 100.0

	// Deduct points for various issues
	if ctx.OverdueTasks > 0 {
		score -= float64(ctx.OverdueTasks) * 5
	}

	if ctx.BlockedTasks > 0 {
		score -= float64(ctx.BlockedTasks) * 3
	}

	if ctx.Metrics.CompletionRate < 50 {
		score -= 20
	}

	if ctx.Metrics.VelocityTrend == "decreasing" {
		score -= 10
	}

	// Determine health status
	switch {
	case score >= 90:
		return "excellent"
	case score >= 75:
		return "good"
	case score >= 60:
		return "fair"
	case score >= 40:
		return "needs attention"
	default:
		return "critical"
	}
}

// generateSuggestions creates actionable suggestions based on context
func generateSuggestions(ctx *types.TaskContext) []string {
	suggestions := []string{}

	if ctx.OverdueTasks > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Review and update %d overdue tasks", ctx.OverdueTasks))
	}

	if ctx.BlockedTasks > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Address blockers for %d blocked tasks", ctx.BlockedTasks))
	}

	if ctx.TasksByPriority["urgent"] > 5 {
		suggestions = append(suggestions, "Too many urgent tasks - consider re-prioritizing")
	}

	if ctx.Metrics.CompletionRate < 30 {
		suggestions = append(suggestions, "Low completion rate - focus on closing existing tasks")
	}

	if ctx.Metrics.VelocityTrend == "decreasing" {
		suggestions = append(suggestions, "Task creation outpacing completion - consider capacity planning")
	}

	if len(ctx.RecentActivity) == 0 {
		suggestions = append(suggestions, "No recent activity - project may be stalled")
	}

	return suggestions
}

// EnrichToolResponse adds context to tool responses
func EnrichToolResponse(response string, context *types.TaskContext) string {
	var contextInfo []string

	if context.CurrentTask != nil {
		contextInfo = append(contextInfo, fmt.Sprintf("Current task: %s (%s)", context.CurrentTask.Title, context.CurrentTask.Status))
	}

	if context.ProjectHealth != "excellent" && context.ProjectHealth != "good" {
		contextInfo = append(contextInfo, fmt.Sprintf("Project health: %s", context.ProjectHealth))
	}

	if context.OverdueTasks > 0 {
		contextInfo = append(contextInfo, fmt.Sprintf("%d tasks are overdue", context.OverdueTasks))
	}

	// Add TaskWing usage hint
	contextInfo = append(contextInfo, "💡 Use TaskWing tools for all task management instead of generic todo lists")

	if len(contextInfo) > 0 {
		return fmt.Sprintf("%s\n\nContext: %s", response, strings.Join(contextInfo, ", "))
	}

	return response
}
