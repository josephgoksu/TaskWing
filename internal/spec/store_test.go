package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Check that specs directory was created
	specsDir := filepath.Join(tmpDir, ".taskwing", "specs")
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Error("specs directory was not created")
	}

	if store.basePath != specsDir {
		t.Errorf("basePath = %q, want %q", store.basePath, specsDir)
	}
}

func TestCreateAndGetSpec(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Create a spec
	spec, err := store.CreateSpec("Add Stripe Integration", "Implement payment processing")
	if err != nil {
		t.Fatalf("CreateSpec failed: %v", err)
	}

	if spec.ID == "" {
		t.Error("spec.ID is empty")
	}
	if spec.Title != "Add Stripe Integration" {
		t.Errorf("spec.Title = %q, want %q", spec.Title, "Add Stripe Integration")
	}
	if spec.Status != StatusDraft {
		t.Errorf("spec.Status = %q, want %q", spec.Status, StatusDraft)
	}

	// Get by slug
	loaded, err := store.GetSpec("add-stripe-integration")
	if err != nil {
		t.Fatalf("GetSpec by slug failed: %v", err)
	}
	if loaded.ID != spec.ID {
		t.Errorf("loaded.ID = %q, want %q", loaded.ID, spec.ID)
	}

	// Get by ID
	loaded2, err := store.GetSpec(spec.ID)
	if err != nil {
		t.Fatalf("GetSpec by ID failed: %v", err)
	}
	if loaded2.Title != spec.Title {
		t.Errorf("loaded2.Title = %q, want %q", loaded2.Title, spec.Title)
	}
}

func TestListSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Empty list
	specs, err := store.ListSpecs()
	if err != nil {
		t.Fatalf("ListSpecs failed: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("len(specs) = %d, want 0", len(specs))
	}

	// Create specs
	_, err = store.CreateSpec("Feature A", "")
	if err != nil {
		t.Fatalf("CreateSpec failed: %v", err)
	}
	_, err = store.CreateSpec("Feature B", "")
	if err != nil {
		t.Fatalf("CreateSpec failed: %v", err)
	}

	specs, err = store.ListSpecs()
	if err != nil {
		t.Fatalf("ListSpecs failed: %v", err)
	}
	if len(specs) != 2 {
		t.Errorf("len(specs) = %d, want 2", len(specs))
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Add Stripe Integration", "add-stripe-integration"},
		{"Fix Bug #123", "fix-bug-123"},
		{"  Spaces  ", "spaces"},
		{"Special!@#$%Chars", "specialchars"},
		{"A very long title that exceeds the maximum allowed character limit for a slug name", "a-very-long-title-that-exceeds-the-maximum-allowed"},
	}

	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSpecNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	_, err = store.GetSpec("nonexistent")
	if err == nil {
		t.Error("GetSpec should return error for nonexistent spec")
	}
}

func TestSaveSpecWithTasks(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	spec, err := store.CreateSpec("Test Feature", "Description")
	if err != nil {
		t.Fatalf("CreateSpec failed: %v", err)
	}

	// Add tasks
	spec.Tasks = []Task{
		{ID: "task-001", SpecID: spec.ID, Title: "Task 1", Status: StatusDraft, Priority: 1, Estimate: "2h"},
		{ID: "task-002", SpecID: spec.ID, Title: "Task 2", Status: StatusDraft, Priority: 2, Estimate: "4h"},
	}

	err = store.SaveSpec(spec)
	if err != nil {
		t.Fatalf("SaveSpec failed: %v", err)
	}

	// Verify tasks.json was created
	tasksPath := filepath.Join(store.basePath, "test-feature", "tasks.json")
	if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
		t.Error("tasks.json was not created")
	}

	// Reload and verify
	loaded, err := store.GetSpec("test-feature")
	if err != nil {
		t.Fatalf("GetSpec failed: %v", err)
	}
	if len(loaded.Tasks) != 2 {
		t.Errorf("len(loaded.Tasks) = %d, want 2", len(loaded.Tasks))
	}
}

func TestListTasks(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	spec, _ := store.CreateSpec("Feature X", "")
	spec.Tasks = []Task{
		{ID: "task-a", SpecID: spec.ID, Title: "A", Priority: 2, Estimate: "1h"},
		{ID: "task-b", SpecID: spec.ID, Title: "B", Priority: 1, Estimate: "1h"},
	}
	store.SaveSpec(spec)

	tasks, err := store.ListTasks("")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("len(tasks) = %d, want 2", len(tasks))
	}

	// Should be sorted by priority
	if tasks[0].Priority > tasks[1].Priority {
		t.Error("tasks not sorted by priority")
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	spec, _ := store.CreateSpec("Feature Y", "")
	spec.Tasks = []Task{
		{ID: "task-xyz", SpecID: spec.ID, Title: "Task", Status: StatusDraft, Priority: 1, Estimate: "1h"},
	}
	store.SaveSpec(spec)

	// Update status
	err = store.UpdateTaskStatus("task-xyz", StatusInProgress)
	if err != nil {
		t.Fatalf("UpdateTaskStatus failed: %v", err)
	}

	// Reload and verify
	loaded, _ := store.GetSpec("feature-y")
	if loaded.Tasks[0].Status != StatusInProgress {
		t.Errorf("task status = %q, want %q", loaded.Tasks[0].Status, StatusInProgress)
	}
}

func TestGetTaskContext(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	spec, _ := store.CreateSpec("Context Test", "Test description")
	spec.Tasks = []Task{
		{ID: "task-ctx", SpecID: spec.ID, Title: "Context Task", Status: StatusDraft, Priority: 1, Estimate: "2h"},
	}
	store.SaveSpec(spec)

	ctx, err := store.GetTaskContext("task-ctx")
	if err != nil {
		t.Fatalf("GetTaskContext failed: %v", err)
	}

	if ctx.Task.ID != "task-ctx" {
		t.Errorf("ctx.Task.ID = %q, want %q", ctx.Task.ID, "task-ctx")
	}
	if ctx.SpecTitle != "Context Test" {
		t.Errorf("ctx.SpecTitle = %q, want %q", ctx.SpecTitle, "Context Test")
	}
	if ctx.SpecContent == "" {
		t.Error("ctx.SpecContent is empty")
	}
}
