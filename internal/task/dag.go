package task

import (
	"errors"
	"fmt"
)

// VerifyDAG checks if the given tasks form a valid Directed Acyclic Graph.
// It detects cycles and ensures dependencies exist within the set or pre-exist.
func VerifyDAG(tasks []Task) error {
	taskMap := make(map[string]Task)
	for _, t := range tasks {
		if t.ID == "" {
			return errors.New("task ID cannot be empty")
		}
		taskMap[t.ID] = t
	}

	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	var checkCycle func(taskID string) error
	checkCycle = func(taskID string) error {
		visited[taskID] = true
		recursionStack[taskID] = true

		task, exists := taskMap[taskID]
		// If task is not in the map, it might be an existing task in DB.
		// For plan creation, we assume all relevant tasks are in the slice.
		// If strict validation against DB is needed, that belongs in service layer.
		// Here we just skip if not in slice (assuming it's external/valid).
		if !exists {
			recursionStack[taskID] = false
			return nil
		}

		for _, depID := range task.Dependencies {
			if !visited[depID] {
				if err := checkCycle(depID); err != nil {
					return err
				}
			} else if recursionStack[depID] {
				return fmt.Errorf("cycle detected involving task %s -> %s", taskID, depID)
			}
		}

		recursionStack[taskID] = false
		return nil
	}

	for _, t := range tasks {
		if !visited[t.ID] {
			if err := checkCycle(t.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// TopologicalSort returns tasks in dependency order (dependencies first).
// Returns error if cycle detected.
func TopologicalSort(tasks []Task) ([]Task, error) {
	if err := VerifyDAG(tasks); err != nil {
		return nil, err
	}

	taskMap := make(map[string]Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	var sorted []Task
	visited := make(map[string]bool)

	var visit func(taskID string)
	visit = func(taskID string) {
		if visited[taskID] {
			return
		}
		visited[taskID] = true

		t, exists := taskMap[taskID]
		if !exists {
			return
		}

		for _, depID := range t.Dependencies {
			visit(depID)
		}
		sorted = append(sorted, t)
	}

	for _, t := range tasks {
		visit(t.ID)
	}

	return sorted, nil
}
