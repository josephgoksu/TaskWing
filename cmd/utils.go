package cmd

import "github.com/josephgoksu/TaskWing/models"

// Helper to convert priority to an integer for sorting
func priorityToInt(p models.TaskPriority) int {
	switch p {
	case models.PriorityUrgent:
		return 4
	case models.PriorityHigh:
		return 3
	case models.PriorityMedium:
		return 2
	case models.PriorityLow:
		return 1
	default:
		return 0 // Should not happen with validated data
	}
}

// Helper to convert status to an integer for sorting (workflow order)
func statusToInt(s models.TaskStatus) int {
	switch s {
	case models.StatusTodo:
		return 1
	case models.StatusDoing:
		return 2
	case models.StatusReview:
		return 3
	case models.StatusDone:
		return 4
	default:
		return 0
	}
}
