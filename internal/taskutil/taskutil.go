package taskutil

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
)

// NormalizePriorityString maps common inputs and typos to canonical priorities.
// Returns one of: low, medium, high, urgent. Empty input stays empty.
func NormalizePriorityString(input string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return "", nil
	}

	switch s {
	case "low", "medium", "high", "urgent":
		return s, nil
	case "lo", "l", "minor":
		return "low", nil
	case "med", "m", "normal", "regular":
		return "medium", nil
	case "hi", "h", "important", "importantn", "imp", "prio-high", "high-priority", "p1":
		return "high", nil
	case "urg", "u", "critical", "asap", "emergency", "prio-urgent", "urgent!":
		return "urgent", nil
	case "p2", "p3", "p4":
		return "medium", nil
	case "p5", "p0":
		return "low", nil
	}

	return "", fmt.Errorf("unknown priority '%s'", input)
}

// ShortID returns the first 8 characters of a UUID-like string for display purposes.
func ShortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// PriorityToInt maps priorities to sortable integer weights (higher = more urgent).
func PriorityToInt(p models.TaskPriority) int {
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
		return 0
	}
}

// StatusToInt maps statuses to workflow order.
func StatusToInt(s models.TaskStatus) int {
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
