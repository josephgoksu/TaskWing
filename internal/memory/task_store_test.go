package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/task"
)

func TestListPlans_TaskCountNotPlaceholderSlice(t *testing.T) {
	// Create a temporary database
	tmpDir, err := os.MkdirTemp("", "taskwing-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "memory.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	// Create a plan
	plan := &task.Plan{
		ID:           "plan-test-123",
		Goal:         "Test goal",
		EnrichedGoal: "Enriched test goal",
		Status:       task.PlanStatusActive,
	}
	if err := store.CreatePlan(plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	// Create some tasks for this plan
	tasks := []task.Task{
		{ID: "task-1", PlanID: plan.ID, Title: "Task 1", Description: "Desc 1", Status: task.StatusPending, Priority: 80},
		{ID: "task-2", PlanID: plan.ID, Title: "Task 2", Description: "Desc 2", Status: task.StatusCompleted, Priority: 70},
		{ID: "task-3", PlanID: plan.ID, Title: "Task 3", Description: "Desc 3", Status: task.StatusInProgress, Priority: 60},
	}
	for _, tsk := range tasks {
		if err := store.CreateTask(&tsk); err != nil {
			t.Fatalf("create task %s: %v", tsk.ID, err)
		}
	}

	// List plans
	plans, err := store.ListPlans()
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}

	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}

	p := plans[0]

	// Verify TaskCount is set correctly
	if p.TaskCount != 3 {
		t.Errorf("expected TaskCount=3, got %d", p.TaskCount)
	}

	// Verify Tasks slice is nil (not placeholder slice)
	if p.Tasks != nil {
		t.Errorf("expected Tasks to be nil, got slice of length %d", len(p.Tasks))
	}

	// Verify GetTaskCount() returns correct value
	if p.GetTaskCount() != 3 {
		t.Errorf("expected GetTaskCount()=3, got %d", p.GetTaskCount())
	}

	// Verify iterating over Tasks doesn't yield misleading zero-value structs
	for i, tsk := range p.Tasks {
		// This loop should not execute since Tasks is nil
		t.Errorf("unexpected task at index %d: %+v", i, tsk)
	}
}

func TestGetTaskCount_FallsBackToTasksLength(t *testing.T) {
	// When TaskCount is 0 but Tasks are populated, use len(Tasks)
	plan := &task.Plan{
		ID:        "test-plan",
		TaskCount: 0, // Not set
		Tasks: []task.Task{
			{ID: "t1", Title: "Task 1"},
			{ID: "t2", Title: "Task 2"},
		},
	}

	if plan.GetTaskCount() != 2 {
		t.Errorf("expected GetTaskCount()=2 (from len(Tasks)), got %d", plan.GetTaskCount())
	}
}

func TestGetTaskCount_UsesTaskCountIfSet(t *testing.T) {
	// When TaskCount is set, use it regardless of Tasks
	plan := &task.Plan{
		ID:        "test-plan",
		TaskCount: 5,
		Tasks:     nil, // Not loaded
	}

	if plan.GetTaskCount() != 5 {
		t.Errorf("expected GetTaskCount()=5 (from TaskCount), got %d", plan.GetTaskCount())
	}
}
