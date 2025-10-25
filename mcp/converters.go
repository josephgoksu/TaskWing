package mcp

import "github.com/josephgoksu/TaskWing/models"
import "github.com/josephgoksu/TaskWing/types"

func taskToResponse(task models.Task) types.TaskResponse {
	var completedAt *string
	if task.CompletedAt != nil {
		completed := task.CompletedAt.Format("2006-01-02T15:04:05Z")
		completedAt = &completed
	}

	return types.TaskResponse{
		ID:                 task.ID,
		Title:              task.Title,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		Status:             string(task.Status),
		Priority:           string(task.Priority),
		ParentID:           task.ParentID,
		SubtaskIDs:         task.SubtaskIDs,
		Dependencies:       task.Dependencies,
		Dependents:         task.Dependents,
		CreatedAt:          task.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          task.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		CompletedAt:        completedAt,
	}
}

func taskToResponsePtr(task models.Task) *types.TaskResponse {
	resp := taskToResponse(task)
	return &resp
}
