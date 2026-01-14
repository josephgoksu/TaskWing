package task

import (
	"testing"
)

func TestVerifyDAG_NoCycle(t *testing.T) {
	// A -> B -> C (linear, no cycle)
	tasks := []Task{
		{ID: "task-A", Title: "Task A", Dependencies: nil},
		{ID: "task-B", Title: "Task B", Dependencies: []string{"task-A"}},
		{ID: "task-C", Title: "Task C", Dependencies: []string{"task-B"}},
	}

	if err := VerifyDAG(tasks); err != nil {
		t.Errorf("VerifyDAG() returned error for valid DAG: %v", err)
	}
}

func TestVerifyDAG_WithCycle(t *testing.T) {
	// A -> B -> C -> A (cycle)
	tasks := []Task{
		{ID: "task-A", Title: "Task A", Dependencies: []string{"task-C"}},
		{ID: "task-B", Title: "Task B", Dependencies: []string{"task-A"}},
		{ID: "task-C", Title: "Task C", Dependencies: []string{"task-B"}},
	}

	err := VerifyDAG(tasks)
	if err == nil {
		t.Error("VerifyDAG() should return error for cycle, got nil")
	}
}

func TestVerifyDAG_EmptyID(t *testing.T) {
	tasks := []Task{
		{ID: "", Title: "Task with no ID"},
	}

	err := VerifyDAG(tasks)
	if err == nil {
		t.Error("VerifyDAG() should return error for empty ID, got nil")
	}
}

func TestTopologicalSort_LinearDependencies(t *testing.T) {
	// C depends on B, B depends on A
	// Expected order: A, B, C
	tasks := []Task{
		{ID: "task-C", Title: "Task C", Dependencies: []string{"task-B"}},
		{ID: "task-A", Title: "Task A", Dependencies: nil},
		{ID: "task-B", Title: "Task B", Dependencies: []string{"task-A"}},
	}

	sorted, err := TopologicalSort(tasks)
	if err != nil {
		t.Fatalf("TopologicalSort() error: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("Expected 3 tasks, got %d", len(sorted))
	}

	// A must come before B, B must come before C
	posA, posB, posC := -1, -1, -1
	for i, task := range sorted {
		switch task.ID {
		case "task-A":
			posA = i
		case "task-B":
			posB = i
		case "task-C":
			posC = i
		}
	}

	if posA >= posB {
		t.Errorf("Task A (pos %d) should come before Task B (pos %d)", posA, posB)
	}
	if posB >= posC {
		t.Errorf("Task B (pos %d) should come before Task C (pos %d)", posB, posC)
	}
}

func TestTopologicalSort_DiamondDependencies(t *testing.T) {
	// Diamond: D depends on B and C, B and C both depend on A
	//     A
	//    / \
	//   B   C
	//    \ /
	//     D
	tasks := []Task{
		{ID: "task-D", Title: "Task D", Dependencies: []string{"task-B", "task-C"}},
		{ID: "task-B", Title: "Task B", Dependencies: []string{"task-A"}},
		{ID: "task-C", Title: "Task C", Dependencies: []string{"task-A"}},
		{ID: "task-A", Title: "Task A", Dependencies: nil},
	}

	sorted, err := TopologicalSort(tasks)
	if err != nil {
		t.Fatalf("TopologicalSort() error: %v", err)
	}

	if len(sorted) != 4 {
		t.Fatalf("Expected 4 tasks, got %d", len(sorted))
	}

	// A must come first, D must come last
	if sorted[0].ID != "task-A" {
		t.Errorf("Expected first task to be A, got %s", sorted[0].ID)
	}
	if sorted[3].ID != "task-D" {
		t.Errorf("Expected last task to be D, got %s", sorted[3].ID)
	}
}

func TestTopologicalSort_WithCycle(t *testing.T) {
	tasks := []Task{
		{ID: "task-A", Title: "Task A", Dependencies: []string{"task-B"}},
		{ID: "task-B", Title: "Task B", Dependencies: []string{"task-A"}},
	}

	_, err := TopologicalSort(tasks)
	if err == nil {
		t.Error("TopologicalSort() should return error for cycle, got nil")
	}
}
