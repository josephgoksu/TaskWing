/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package types

import "time"

// TaskContext provides rich context about tasks for AI tools
type TaskContext struct {
	TotalTasks      int             `json:"total_tasks"`
	TasksByStatus   map[string]int  `json:"tasks_by_status"`
	TasksByPriority map[string]int  `json:"tasks_by_priority"`
	OverdueTasks    int             `json:"overdue_tasks"`
	BlockedTasks    int             `json:"blocked_tasks"`
	RecentActivity  []ActivityEntry `json:"recent_activity"`
	Suggestions     []string        `json:"suggestions"`
	ProjectHealth   string          `json:"project_health"`
	Metrics         ProjectMetrics  `json:"metrics"`
	CurrentTask     *TaskResponse   `json:"current_task,omitempty"`
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
	CompletionRate     float64 `json:"completion_rate"`
	AverageTaskAge     float64 `json:"average_task_age_days"`
	TasksCompletedWeek int     `json:"tasks_completed_this_week"`
	TasksCreatedWeek   int     `json:"tasks_created_this_week"`
	VelocityTrend      string  `json:"velocity_trend"` // increasing, decreasing, stable
}

// TaskUpdates represents the fields that can be updated on a task
type TaskUpdates struct {
	Title              *string    `json:"title,omitempty"`
	Description        *string    `json:"description,omitempty"`
	AcceptanceCriteria *string    `json:"acceptanceCriteria,omitempty"`
	Status             *string    `json:"status,omitempty"`
	Priority           *string    `json:"priority,omitempty"`
	ParentID           *string    `json:"parentId,omitempty"`
	Dependencies       *[]string  `json:"dependencies,omitempty"`
	CompletedAt        *time.Time `json:"completedAt,omitempty"`
}
