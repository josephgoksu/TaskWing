package taskutil

import (
	"fmt"
	"sort"
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

// ResolveTaskReference attempts to find a task in a list by partial ID or fuzzy title match.
// Returns the resolved task, or an error if not found or ambiguous.
func ResolveTaskReference(reference string, tasks []models.Task) (*models.Task, error) {
	// First pass: Exact ID check
	for _, t := range tasks {
		if t.ID == reference {
			return &t, nil
		}
	}

	// Try partial ID match (minimum 8 characters for meaningful UUID portion)
	if len(reference) >= 8 {
		for _, task := range tasks {
			if strings.HasPrefix(strings.ToLower(task.ID), strings.ToLower(reference)) {
				return &task, nil
			}
		}
	}

	// Try fuzzy title matching
	type match struct {
		task  models.Task
		score float64
	}

	var matches []match
	refLower := strings.ToLower(reference)

	for _, task := range tasks {
		titleLower := strings.ToLower(task.Title)

		// Exact title match
		if titleLower == refLower {
			return &task, nil
		}

		// Substring match in title
		if strings.Contains(titleLower, refLower) {
			score := 0.9 - (float64(len(titleLower)-len(refLower)) / float64(len(titleLower)) * 0.3)
			matches = append(matches, match{task: task, score: score})
		}
	}

	// Sort matches by score and return best match if confidence is high enough
	if len(matches) > 0 {
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].score > matches[j].score
		})

		// If we have a high confidence match (>80%) and it's the only good match
		if matches[0].score > 0.8 && (len(matches) == 1 || matches[0].score > matches[1].score+0.2) {
			return &matches[0].task, nil
		}

		// If we have multiple similar matches, provide helpful error
		if len(matches) > 1 {
			var suggestions []string
			for i, m := range matches {
				if i >= 3 { // Limit to top 3 suggestions
					break
				}
				suggestions = append(suggestions, fmt.Sprintf("  %s - %s",
					m.task.ID[:8], m.task.Title))
			}

			return nil, fmt.Errorf("multiple matches found for '%s'. Did you mean:\n%s\n\nUse a more specific reference or full task ID",
				reference, strings.Join(suggestions, "\n"))
		}
	}

	return nil, fmt.Errorf("no task found matching '%s'", reference)
}
