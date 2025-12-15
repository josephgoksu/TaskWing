package cmd

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
)

// statusIcon provides a compact emoji indicator for task status
func statusIcon(s models.TaskStatus) string {
	switch s {
	case models.StatusTodo:
		return "⭕"
	case models.StatusDoing:
		return "🔄"
	case models.StatusReview:
		return "🔍"
	case models.StatusDone:
		return "✅"
	default:
		return string(s)
	}
}

// priorityIcon provides a compact emoji badge for task priority
func priorityIcon(p models.TaskPriority) string {
	switch p {
	case models.PriorityUrgent:
		return "🟥 urgent"
	case models.PriorityHigh:
		return "🟧 high"
	case models.PriorityMedium:
		return "🟨 medium"
	case models.PriorityLow:
		return "🟩 low"
	default:
		return string(p)
	}
}

// resolveTaskID resolves a partial task ID to a full task ID.
// Simplified version supporting only ID-based resolution.
// For fuzzy matching, CLI commands can fall back to prompting the user.
func resolveTaskID(st store.TaskStore, partialID string) (string, error) {
	partialID = strings.TrimSpace(strings.ToLower(partialID))
	if partialID == "" {
		return "", fmt.Errorf("task ID cannot be empty")
	}

	tasks, err := st.ListTasks(nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list tasks: %w", err)
	}

	// Exact match
	for _, task := range tasks {
		if strings.ToLower(task.ID) == partialID {
			return task.ID, nil
		}
	}

	// Prefix match (4+ chars)
	if len(partialID) >= 4 {
		var matches []string
		for _, task := range tasks {
			if strings.HasPrefix(strings.ToLower(task.ID), partialID) {
				matches = append(matches, task.ID)
			}
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
		if len(matches) > 1 {
			return "", fmt.Errorf("ambiguous ID '%s' matches %d tasks", partialID, len(matches))
		}
	}

	return "", fmt.Errorf("no task found with ID '%s'", partialID)
}
