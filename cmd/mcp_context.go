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
)

// TaskContext provides rich context about tasks for AI tools
type TaskContext struct {
	TotalTasks      int                       `json:"total_tasks"`
	TasksByStatus   map[string]int            `json:"tasks_by_status"`
	TasksByPriority map[string]int            `json:"tasks_by_priority"`
	OverdueTasks    int                       `json:"overdue_tasks"`
	BlockedTasks    int                       `json:"blocked_tasks"`
	RecentActivity  []ActivityEntry           `json:"recent_activity"`
	Suggestions     []string                  `json:"suggestions"`
	ProjectHealth   string                    `json:"project_health"`
	Metrics         ProjectMetrics            `json:"metrics"`
}

// ActivityEntry represents a recent task activity
type ActivityEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	TaskID      string    `json:"task_id"`
	TaskTitle   string    `json:"task_title"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
}

// ProjectMetrics provides project-level metrics
type ProjectMetrics struct {
	CompletionRate      float64 `json:"completion_rate"`
	AverageTaskAge      float64 `json:"average_task_age_days"`
	TasksCompletedWeek  int     `json:"tasks_completed_this_week"`
	TasksCreatedWeek    int     `json:"tasks_created_this_week"`
	VelocityTrend       string  `json:"velocity_trend"` // increasing, decreasing, stable
}

// BuildTaskContext creates a comprehensive context for AI tools
func BuildTaskContext(taskStore store.TaskStore) (*TaskContext, error) {
	tasks, err := taskStore.ListTasks(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks for context: %w", err)
	}

	context := &TaskContext{
		TotalTasks:      len(tasks),
		TasksByStatus:   make(map[string]int),
		TasksByPriority: make(map[string]int),
		RecentActivity:  []ActivityEntry{},
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

			context.RecentActivity = append(context.RecentActivity, ActivityEntry{
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

	// Generate project health assessment
	context.ProjectHealth = assessProjectHealth(context)

	// Generate suggestions
	context.Suggestions = generateSuggestions(context)

	return context, nil
}

// assessProjectHealth provides an overall health assessment
func assessProjectHealth(ctx *TaskContext) string {
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
func generateSuggestions(ctx *TaskContext) []string {
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
func EnrichToolResponse(response string, context *TaskContext) string {
	var contextInfo []string

	if context.ProjectHealth != "excellent" && context.ProjectHealth != "good" {
		contextInfo = append(contextInfo, fmt.Sprintf("Project health: %s", context.ProjectHealth))
	}

	if context.OverdueTasks > 0 {
		contextInfo = append(contextInfo, fmt.Sprintf("%d tasks are overdue", context.OverdueTasks))
	}

	if len(contextInfo) > 0 {
		return fmt.Sprintf("%s\n\nContext: %s", response, strings.Join(contextInfo, ", "))
	}

	return response
}