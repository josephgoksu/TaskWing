package mcp

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/taskutil"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/types"
)

type taskRelationshipMap struct {
	tempParentToChildren map[string][]string
	tempChildToParent    map[string]string
	tempTaskToDeps       map[string][]string
	flattenedTasks       map[string]models.Task
	tempIDToInputID      map[int]string
	taskOrder            []string
}

const tempIDPrefix = "tmp_"

func resolveAndBuildTaskCandidates(outputs []types.TaskOutput) ([]models.Task, taskRelationshipMap, error) {
	rel := taskRelationshipMap{
		tempParentToChildren: map[string][]string{},
		tempChildToParent:    map[string]string{},
		tempTaskToDeps:       map[string][]string{},
		flattenedTasks:       map[string]models.Task{},
		tempIDToInputID:      map[int]string{},
		taskOrder:            []string{},
	}
	counter := 0
	var walk func(items []types.TaskOutput)
	walk = func(items []types.TaskOutput) {
		for _, it := range items {
			counter++
			tid := fmt.Sprintf("%s%d", tempIDPrefix, counter)
			if strings.TrimSpace(it.Title) == "" {
				continue
			}
			t := models.Task{
				Title:              it.Title,
				Description:        it.Description,
				AcceptanceCriteria: it.GetAcceptanceCriteriaAsString(),
				Status:             models.StatusTodo,
				Priority:           mapLLMPriority(it.Priority),
			}
			rel.flattenedTasks[tid] = t
			rel.taskOrder = append(rel.taskOrder, tid)
			if len(it.Subtasks) > 0 {
				walk(it.Subtasks)
			}
		}
	}
	walk(outputs)
	list := make([]models.Task, 0, len(rel.taskOrder))
	for _, id := range rel.taskOrder {
		list = append(list, rel.flattenedTasks[id])
	}
	return list, rel, nil
}

func mapLLMPriority(p string) models.TaskPriority {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "urgent":
		return models.PriorityUrgent
	case "high":
		return models.PriorityHigh
	case "low":
		return models.PriorityLow
	default:
		return models.PriorityMedium
	}
}

func mapPriorityOrDefault(p string) models.TaskPriority {
	if canon, err := taskutil.NormalizePriorityString(p); err == nil && canon != "" {
		return models.TaskPriority(canon)
	}
	return models.PriorityMedium
}
