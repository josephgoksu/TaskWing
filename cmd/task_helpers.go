package cmd

import (
	"github.com/josephgoksu/TaskWing/internal/task"
)

func isValidTaskStatus(status task.TaskStatus) bool {
	switch status {
	case task.StatusDraft, task.StatusPending, task.StatusInProgress, task.StatusVerifying, task.StatusCompleted, task.StatusFailed:
		return true
	default:
		return false
	}
}
