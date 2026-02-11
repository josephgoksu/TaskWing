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
	defer func() { _ = os.RemoveAll(tmpDir) }()

	dbPath := filepath.Join(tmpDir, "memory.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer func() { _ = store.Close() }()

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

func TestGetNextTask_SelectsLowestNumericPriority(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "memory.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	plan := &task.Plan{
		ID:           "plan-priority-next",
		Goal:         "Priority ordering test",
		EnrichedGoal: "Priority ordering test",
		Status:       task.PlanStatusActive,
	}
	if err := store.CreatePlan(plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	highUrgency := &task.Task{
		ID:          "task-pri-10",
		PlanID:      plan.ID,
		Title:       "Priority 10",
		Description: "Should be selected first",
		Status:      task.StatusPending,
		Priority:    10,
	}
	lowUrgency := &task.Task{
		ID:          "task-pri-90",
		PlanID:      plan.ID,
		Title:       "Priority 90",
		Description: "Should be selected later",
		Status:      task.StatusPending,
		Priority:    90,
	}

	if err := store.CreateTask(lowUrgency); err != nil {
		t.Fatalf("create task priority 90: %v", err)
	}
	if err := store.CreateTask(highUrgency); err != nil {
		t.Fatalf("create task priority 10: %v", err)
	}

	next, err := store.GetNextTask(plan.ID)
	if err != nil {
		t.Fatalf("get next task: %v", err)
	}
	if next == nil {
		t.Fatal("expected next task, got nil")
	}
	if next.ID != highUrgency.ID {
		t.Fatalf("expected next task %q, got %q", highUrgency.ID, next.ID)
	}
}

func TestListTasksByPhase_OrdersByAscendingPriority(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "memory.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	plan := &task.Plan{
		ID:           "plan-phase-order",
		Goal:         "Phase order test",
		EnrichedGoal: "Phase order test",
		Status:       task.PlanStatusActive,
	}
	if err := store.CreatePlan(plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	phase := &task.Phase{
		ID:         "phase-1",
		PlanID:     plan.ID,
		Title:      "Phase 1",
		Status:     task.PhaseStatusExpanded,
		OrderIndex: 0,
	}
	if err := store.CreatePhase(phase); err != nil {
		t.Fatalf("create phase: %v", err)
	}

	tasks := []*task.Task{
		{ID: "task-p90", PlanID: plan.ID, PhaseID: phase.ID, Title: "P90", Description: "P90", Status: task.StatusPending, Priority: 90},
		{ID: "task-p10", PlanID: plan.ID, PhaseID: phase.ID, Title: "P10", Description: "P10", Status: task.StatusPending, Priority: 10},
		{ID: "task-p50", PlanID: plan.ID, PhaseID: phase.ID, Title: "P50", Description: "P50", Status: task.StatusPending, Priority: 50},
	}
	for _, tt := range tasks {
		if err := store.CreateTask(tt); err != nil {
			t.Fatalf("create task %s: %v", tt.ID, err)
		}
	}

	phaseTasks, err := store.ListTasksByPhase(phase.ID)
	if err != nil {
		t.Fatalf("list tasks by phase: %v", err)
	}
	if len(phaseTasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(phaseTasks))
	}

	wantOrder := []string{"task-p10", "task-p50", "task-p90"}
	for i, wantID := range wantOrder {
		if phaseTasks[i].ID != wantID {
			t.Fatalf("phase task order mismatch at index %d: got %s, want %s", i, phaseTasks[i].ID, wantID)
		}
	}
}
