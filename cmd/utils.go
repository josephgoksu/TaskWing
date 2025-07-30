package cmd

import "github.com/josephgoksu/taskwing.app/models"

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

// Helper to convert status to an integer for sorting (example order)
func statusToInt(s models.TaskStatus) int {
	switch s {
	case models.StatusPending:
		return 1
	case models.StatusInProgress:
		return 2
	case models.StatusBlocked:
		return 3
	case models.StatusNeedsReview:
		return 4
	case models.StatusOnHold:
		return 5
	case models.StatusCompleted:
		return 6
	case models.StatusCancelled:
		return 7
	default:
		return 0 // Should not happen
	}
}
