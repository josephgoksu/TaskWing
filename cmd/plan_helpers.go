package cmd

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/task"
)

func resolveLatestPlanID(repo *memory.Repository) (string, error) {
	plans, err := repo.ListPlans()
	if err != nil {
		return "", fmt.Errorf("list plans: %w", err)
	}
	if len(plans) == 0 {
		return "", fmt.Errorf("no plans found. Create one with: tw plan new \"Your goal\"")
	}
	return plans[0].ID, nil
}

func isValidPlanStatus(status task.PlanStatus) bool {
	switch status {
	case task.PlanStatusDraft, task.PlanStatusActive, task.PlanStatusCompleted, task.PlanStatusArchived:
		return true
	default:
		return false
	}
}

func isValidTaskStatus(status task.TaskStatus) bool {
	switch status {
	case task.StatusDraft, task.StatusPending, task.StatusInProgress, task.StatusBlocked, task.StatusVerifying, task.StatusCompleted, task.StatusFailed:
		return true
	default:
		return false
	}
}
