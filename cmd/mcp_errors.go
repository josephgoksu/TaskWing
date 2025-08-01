/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"errors"
	"fmt"
	"strings"
)

// MCP error types for better error categorization
var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrTaskNotFound      = errors.New("task not found")
	ErrDependencyConflict = errors.New("dependency conflict")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrResourceConflict  = errors.New("resource conflict")
)

// MCPError provides structured error information for MCP responses
type MCPError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewMCPError creates a new structured MCP error
func NewMCPError(code string, message string, details map[string]interface{}) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// ValidateTaskInput validates common task input parameters
func ValidateTaskInput(title, priority, status string) error {
	// Title validation
	if title != "" && len(strings.TrimSpace(title)) < 3 {
		return NewMCPError("INVALID_TITLE", "Task title must be at least 3 characters long", map[string]interface{}{
			"field": "title",
			"value": title,
		})
	}

	// Priority validation
	if priority != "" {
		validPriorities := map[string]bool{"low": true, "medium": true, "high": true, "urgent": true}
		if !validPriorities[strings.ToLower(priority)] {
			return NewMCPError("INVALID_PRIORITY", "Invalid priority value", map[string]interface{}{
				"field":           "priority",
				"value":           priority,
				"valid_values":    []string{"low", "medium", "high", "urgent"},
			})
		}
	}

	// Status validation
	if status != "" {
		validStatuses := map[string]bool{
			"pending": true, "in-progress": true, "completed": true,
			"cancelled": true, "on-hold": true, "blocked": true, "needs-review": true,
		}
		if !validStatuses[strings.ToLower(status)] {
			return NewMCPError("INVALID_STATUS", "Invalid status value", map[string]interface{}{
				"field":        "status",
				"value":        status,
				"valid_values": []string{"pending", "in-progress", "completed", "cancelled", "on-hold", "blocked", "needs-review"},
			})
		}
	}

	return nil
}

// WrapStoreError wraps store errors with more context
func WrapStoreError(err error, operation string, taskID string) error {
	if err == nil {
		return nil
	}

	// Check for common error patterns
	errStr := err.Error()
	
	if strings.Contains(errStr, "not found") {
		return NewMCPError("TASK_NOT_FOUND", fmt.Sprintf("Task %s not found", taskID), map[string]interface{}{
			"operation": operation,
			"task_id":   taskID,
		})
	}

	if strings.Contains(errStr, "circular dependency") {
		return NewMCPError("CIRCULAR_DEPENDENCY", "Operation would create a circular dependency", map[string]interface{}{
			"operation": operation,
			"task_id":   taskID,
		})
	}

	if strings.Contains(errStr, "dependents") {
		return NewMCPError("HAS_DEPENDENTS", "Cannot delete task with dependent tasks", map[string]interface{}{
			"operation": operation,
			"task_id":   taskID,
		})
	}

	// Generic error
	return NewMCPError("OPERATION_FAILED", fmt.Sprintf("%s operation failed: %v", operation, err), map[string]interface{}{
		"operation":     operation,
		"task_id":       taskID,
		"original_error": err.Error(),
	})
}