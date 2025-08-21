/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/josephgoksu/taskwing.app/types"
)

// MCP error types for better error categorization
var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrTaskNotFound       = errors.New("task not found")
	ErrDependencyConflict = errors.New("dependency conflict")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrResourceConflict   = errors.New("resource conflict")
)

// ValidateTaskInput validates common task input parameters
func ValidateTaskInput(title, priority, status string) error {
	// Title validation
	if title != "" && len(strings.TrimSpace(title)) < 3 {
		return types.NewMCPError("INVALID_TITLE", "Task title must be at least 3 characters long", map[string]interface{}{
			"field": "title",
			"value": title,
		})
	}

	// Priority validation
	if priority != "" {
		validPriorities := map[string]bool{"low": true, "medium": true, "high": true, "urgent": true}
		if !validPriorities[strings.ToLower(priority)] {
			return types.NewMCPError("INVALID_PRIORITY", "Invalid priority value", map[string]interface{}{
				"field":        "priority",
				"value":        priority,
				"valid_values": []string{"low", "medium", "high", "urgent"},
			})
		}
	}

	// Status validation
	if status != "" {
		validStatuses := map[string]bool{
			"todo": true, "doing": true, "review": true, "done": true,
		}
		if !validStatuses[strings.ToLower(status)] {
			return types.NewMCPError("INVALID_STATUS", "Invalid status value", map[string]interface{}{
				"field":        "status",
				"value":        status,
				"valid_values": []string{"todo", "doing", "review", "done", "pending", "in-progress", "completed", "cancelled", "on-hold", "blocked", "needs-review"},
			})
		}
	}

	return nil
}

// CreateUserFriendlyError creates errors with helpful context for AI tools
func CreateUserFriendlyError(code, message string, context map[string]interface{}) error {
	// Enhance context with helpful information
	enhancedContext := make(map[string]interface{})
	for k, v := range context {
		enhancedContext[k] = v
	}

	// Add general help based on error code
	switch code {
	case "TASK_NOT_FOUND":
		enhancedContext["tip"] = "💡 Use 'find-task' for smart task discovery with fuzzy matching"
	case "VALIDATION_FAILED":
		enhancedContext["tip"] = "💡 Check field formats: status (todo/doing/review/done), priority (low/medium/high/urgent)"
	case "FILTER_ERROR":
		enhancedContext["tip"] = "💡 Use 'query-tasks' for natural language filtering like 'high priority unfinished tasks'"
	case "MISSING_TITLE":
		enhancedContext["tip"] = "💡 Task titles should be descriptive and at least 3 characters long"
	}

	return types.NewMCPError(code, message, enhancedContext)
}

// ValidateAndSuggestTaskInput provides enhanced input validation with suggestions
func ValidateAndSuggestTaskInput(title, priority, status string, availableStatuses, availablePriorities []string) error {
	// Title validation with suggestions
	if title != "" {
		trimmedTitle := strings.TrimSpace(title)
		if len(trimmedTitle) < 3 {
			return CreateUserFriendlyError("INVALID_TITLE", "Task title must be at least 3 characters long", map[string]interface{}{
				"field":      "title",
				"value":      title,
				"min_length": 3,
				"suggestions": []string{
					"Make titles descriptive: 'Fix login bug' instead of 'Fix'",
					"Include context: 'Update user API documentation'",
					"Use action verbs: 'Implement', 'Fix', 'Add', 'Update'",
				},
			})
		}
		if len(trimmedTitle) > 255 {
			return CreateUserFriendlyError("TITLE_TOO_LONG", "Task title must be 255 characters or less", map[string]interface{}{
				"field":          "title",
				"current_length": len(trimmedTitle),
				"max_length":     255,
				"suggestion":     "Consider moving detailed information to the description field",
			})
		}
	}

	// Priority validation with available options
	if priority != "" {
		validPriorities := map[string]bool{"low": true, "medium": true, "high": true, "urgent": true}
		if !validPriorities[strings.ToLower(priority)] {
			errorContext := map[string]interface{}{
				"field":        "priority",
				"value":        priority,
				"valid_values": []string{"low", "medium", "high", "urgent"},
			}

			// Add suggestions based on input
			if suggestions := suggestPriorityCorrections(priority); len(suggestions) > 0 {
				errorContext["suggestions"] = suggestions
			}

			// Show available priorities from actual tasks
			if len(availablePriorities) > 0 {
				errorContext["available_in_project"] = availablePriorities
			}

			return CreateUserFriendlyError("INVALID_PRIORITY", "Invalid priority value", errorContext)
		}
	}

	// Status validation with available options
	if status != "" {
		validStatuses := map[string]bool{
			"todo": true, "doing": true, "review": true, "done": true,
		}
		if !validStatuses[strings.ToLower(status)] {
			errorContext := map[string]interface{}{
				"field":           "status",
				"value":           status,
				"core_statuses":   []string{"todo", "doing", "review", "done"},
				"legacy_statuses": []string{"pending", "in-progress", "completed", "cancelled", "on-hold", "blocked", "needs-review"},
			}

			// Add suggestions based on input
			if suggestions := suggestStatusCorrections(status); len(suggestions) > 0 {
				errorContext["suggestions"] = suggestions
			}

			// Show available statuses from actual tasks
			if len(availableStatuses) > 0 {
				errorContext["available_in_project"] = availableStatuses
			}

			return CreateUserFriendlyError("INVALID_STATUS", "Invalid status value", errorContext)
		}
	}

	return nil
}

// suggestPriorityCorrections suggests likely intended priorities
func suggestPriorityCorrections(input string) []string {
	input = strings.ToLower(input)
	var suggestions []string

	// Common typos and alternatives
	corrections := map[string]string{
		"lo": "low", "l": "low", "minor": "low",
		"med": "medium", "m": "medium", "normal": "medium", "regular": "medium",
		"hi": "high", "h": "high", "important": "high",
		"urg": "urgent", "u": "urgent", "critical": "urgent", "asap": "urgent", "emergency": "urgent",
	}

	if correction, exists := corrections[input]; exists {
		suggestions = append(suggestions, fmt.Sprintf("Did you mean '%s'?", correction))
	}

	// Fuzzy matching
	for _, priority := range []string{"low", "medium", "high", "urgent"} {
		if strings.Contains(priority, input) || strings.Contains(input, priority) {
			suggestions = append(suggestions, fmt.Sprintf("Maybe '%s'?", priority))
		}
	}

	return suggestions
}

// suggestStatusCorrections suggests likely intended statuses
func suggestStatusCorrections(input string) []string {
	input = strings.ToLower(input)
	var suggestions []string

	// Common typos and alternatives
	corrections := map[string]string{
		"to-do": "todo", "to_do": "todo", "pending": "todo", "new": "todo",
		"in-progress": "doing", "in_progress": "doing", "working": "doing", "active": "doing",
		"reviewing": "review", "needs-review": "review", "needs_review": "review",
		"complete": "done", "completed": "done", "finished": "done", "closed": "done",
	}

	if correction, exists := corrections[input]; exists {
		suggestions = append(suggestions, fmt.Sprintf("Did you mean '%s'?", correction))
	}

	// Fuzzy matching
	for _, status := range []string{"todo", "doing", "review", "done"} {
		if strings.Contains(status, input) || strings.Contains(input, status) {
			suggestions = append(suggestions, fmt.Sprintf("Maybe '%s'?", status))
		}
	}

	return suggestions
}

// WrapStoreError wraps store errors with more context and actionable suggestions
func WrapStoreError(err error, operation string, taskID string) error {
	if err == nil {
		return nil
	}

	// Check for common error patterns
	errStr := err.Error()

	if strings.Contains(errStr, "not found") {
		return types.NewMCPError("TASK_NOT_FOUND", fmt.Sprintf("Task %s not found", taskID), map[string]interface{}{
			"operation": operation,
			"task_id":   taskID,
			"suggestions": []string{
				"Use 'list-tasks' to see all available tasks",
				"Try 'find-task' with partial ID or title for fuzzy matching",
				"Check if the task was deleted or moved",
			},
			"help_commands": []string{
				"list-tasks",
				"find-task",
				"suggest-tasks",
			},
		})
	}

	if strings.Contains(errStr, "circular dependency") {
		return types.NewMCPError("CIRCULAR_DEPENDENCY", "Operation would create a circular dependency", map[string]interface{}{
			"operation":   operation,
			"task_id":     taskID,
			"explanation": "Dependencies must form a directed acyclic graph (DAG)",
			"suggestions": []string{
				"Remove conflicting dependencies first",
				"Use 'dependency-health' tool to check for dependency issues",
				"Consider breaking the task into smaller independent subtasks",
			},
		})
	}

	if strings.Contains(errStr, "dependents") {
		return types.NewMCPError("HAS_DEPENDENTS", "Cannot delete task with dependent tasks", map[string]interface{}{
			"operation": operation,
			"task_id":   taskID,
			"suggestions": []string{
				"Delete dependent tasks first, or remove their dependencies",
				"Use 'get-task' to see which tasks depend on this one",
				"Consider completing the task instead of deleting it",
			},
		})
	}

	// Generic error with contextual help
	return types.NewMCPError("OPERATION_FAILED", fmt.Sprintf("%s operation failed: %v", operation, err), map[string]interface{}{
		"operation":      operation,
		"task_id":        taskID,
		"original_error": err.Error(),
		"suggestions": []string{
			"Check if the task exists using 'get-task' or 'list-tasks'",
			"Verify input parameters and try again",
			"Use 'task-summary' to check project health",
		},
	})
}
