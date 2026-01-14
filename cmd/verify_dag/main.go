package main

import (
	"fmt"
	"log"
	"os"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/task"
)

func main() {
	// Setup temporary DB
	tmpDir, err := os.MkdirTemp("", "taskwing-test")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := memory.NewSQLiteStore(tmpDir)
	if err != nil {
		log.Fatal(err)
	}

	// Create Plan
	p := &task.Plan{
		Goal:         "Build a Rocket",
		EnrichedGoal: "Build a Falcon 9 replica",
		Status:       task.PlanStatusDraft,
	}

	// Create Tasks (A -> B -> C)
	taskA := task.Task{
		Title:       "Task A (Engine)",
		Description: "Build engine",
		Complexity:  "high",
		Status:      task.StatusPending,
		Priority:    100,
	}

	taskB := task.Task{
		Title:       "Task B (Fuselage)",
		Description: "Build body",
		Complexity:  "medium",
		Status:      task.StatusPending,
		Priority:    90,
	}

	taskC := task.Task{
		Title:       "Task C (Assembly)",
		Description: "Put it together",
		Complexity:  "low",
		Status:      task.StatusPending,
		Priority:    80,
	}

	// Save Plan (generates Plan ID)
	// Note: Store.CreatePlan expects tasks in the slice, but we want to wire dependencies manually for this test
	// to verify the logic. Actually, let's use CreatePlan with empty tasks first, then add them.
	// Or define them in the slice. But we need IDs to define dependencies.
	// Better: Generate IDs manually first.
	p.Tasks = []task.Task{taskA, taskB, taskC}
	// CreatePlan assigns IDs if empty.

	// Let's create proper IDs first so we can link them
	taskA.ID = "task-A"
	taskB.ID = "task-B"
	taskC.ID = "task-C"

	// C depends on A and B
	taskC.Dependencies = []string{"task-A", "task-B"}
	// B depends on A
	taskB.Dependencies = []string{"task-A"}

	p.Tasks = []task.Task{taskA, taskB, taskC}

	fmt.Println("Creating Plan with Dependencies: C->(A,B), B->A")
	if err := store.CreatePlan(p); err != nil {
		log.Fatalf("CreatePlan failed: %v", err)
	}

	// Retrieve and Verify
	fetchedPlan, err := store.GetPlan(p.ID)
	if err != nil {
		log.Fatalf("GetPlan failed: %v", err)
	}

	fmt.Printf("Fetched Plan ID: %s\n", fetchedPlan.ID)

	taskMap := make(map[string]task.Task)
	for _, t := range fetchedPlan.Tasks {
		taskMap[t.ID] = t
		fmt.Printf("Task %s (%s): Deps=%v Complexity=%s\n", t.ID, t.Title, t.Dependencies, t.Complexity)
	}

	// Verify A has no deps
	if len(taskMap["task-A"].Dependencies) != 0 {
		log.Fatalf("Task A should have 0 deps, got %d", len(taskMap["task-A"].Dependencies))
	}

	// Verify B depends on A
	if len(taskMap["task-B"].Dependencies) != 1 || taskMap["task-B"].Dependencies[0] != "task-A" {
		log.Fatalf("Task B should depend on A, got %v", taskMap["task-B"].Dependencies)
	}

	// Verify C depends on A and B
	depsC := taskMap["task-C"].Dependencies
	if len(depsC) != 2 {
		log.Fatalf("Task C should have 2 deps, got %d", len(depsC))
	}

	// Verify Logic: Topological Sort
	fmt.Println("Running Topological Sort...")
	sorted, err := task.TopologicalSort(fetchedPlan.Tasks)
	if err != nil {
		log.Fatalf("TopologicalSort failed: %v", err)
	}

	for i, t := range sorted {
		fmt.Printf("%d: %s\n", i, t.Title)
	}

	// Expected Order: A, B, C (Since B depends on A, and C depends on both)
	if sorted[0].ID != "task-A" {
		log.Fatalf("Expected first task to be A, got %s", sorted[0].ID)
	}
	if sorted[1].ID != "task-B" {
		log.Fatalf("Expected second task to be B, got %s", sorted[1].ID)
	}
	if sorted[2].ID != "task-C" {
		log.Fatalf("Expected third task to be C, got %s", sorted[2].ID)
	}

	fmt.Println("SUCCESS: DAG Support Verified!")
}
