package store

import (
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/models"
)

func setupTestStore(t *testing.T) *FileTaskStore {
	t.Helper()

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "tasks.json")

	store := NewFileTaskStore()
	config := map[string]string{
		"dataFile":       filePath,
		"dataFileFormat": "json",
	}

	err := store.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	return store
}

func TestFileTaskStore_BasicOperations(t *testing.T) {
	store := setupTestStore(t)
	defer func() { _ = store.Close() }()

	// Test CreateTask
	task := models.Task{
		Title:       "Test Task",
		Description: "Test Description",
		Status:      models.StatusTodo,
		Priority:    models.PriorityMedium,
	}

	created, err := store.CreateTask(task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if created.ID == "" {
		t.Error("Created task should have an ID")
	}
	if created.Title != task.Title {
		t.Errorf("Title mismatch: got %q, want %q", created.Title, task.Title)
	}

	// Test GetTask
	retrieved, err := store.GetTask(created.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, created.ID)
	}
	if retrieved.Title != created.Title {
		t.Errorf("Title mismatch: got %q, want %q", retrieved.Title, created.Title)
	}

	// Test UpdateTask
	updates := map[string]interface{}{
		"title":    "Updated Task",
		"priority": "high",
		"status":   "doing",
	}

	updated, err := store.UpdateTask(created.ID, updates)
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	if updated.Title != "Updated Task" {
		t.Errorf("Title not updated: got %q, want %q", updated.Title, "Updated Task")
	}
	if updated.Priority != models.PriorityHigh {
		t.Errorf("Priority not updated: got %q, want %q", updated.Priority, models.PriorityHigh)
	}
	if updated.Status != models.StatusDoing {
		t.Errorf("Status not updated: got %q, want %q", updated.Status, models.StatusDoing)
	}

	// Test ListTasks
	tasks, err := store.ListTasks(nil, nil)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	if tasks[0].ID != updated.ID {
		t.Errorf("Listed task ID mismatch: got %q, want %q", tasks[0].ID, updated.ID)
	}

	// Test MarkTaskDone
	done, err := store.MarkTaskDone(updated.ID)
	if err != nil {
		t.Fatalf("MarkTaskDone failed: %v", err)
	}

	if done.Status != models.StatusDone {
		t.Errorf("Task not marked done: got %q, want %q", done.Status, models.StatusDone)
	}
	if done.CompletedAt == nil {
		t.Error("CompletedAt should be set when task is marked done")
	}

	// Test DeleteTask
	err = store.DeleteTask(done.ID)
	if err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// Verify task is deleted
	_, err = store.GetTask(done.ID)
	if err == nil {
		t.Error("Expected error when getting deleted task")
	}
}

func TestFileTaskStore_ParentChildRelationship(t *testing.T) {
	store := setupTestStore(t)
	defer func() { _ = store.Close() }()

	// Create parent task
	parent := models.Task{
		Title:    "Parent Task",
		Status:   models.StatusTodo,
		Priority: models.PriorityMedium,
	}

	createdParent, err := store.CreateTask(parent)
	if err != nil {
		t.Fatalf("Failed to create parent task: %v", err)
	}

	// Create child task
	child := models.Task{
		Title:    "Child Task",
		Status:   models.StatusTodo,
		Priority: models.PriorityMedium,
		ParentID: &createdParent.ID,
	}

	createdChild, err := store.CreateTask(child)
	if err != nil {
		t.Fatalf("Failed to create child task: %v", err)
	}

	// Verify parent-child relationship
	if createdChild.ParentID == nil || *createdChild.ParentID != createdParent.ID {
		t.Errorf("Child parent ID incorrect: got %v, want %s", createdChild.ParentID, createdParent.ID)
	}

	// Verify parent has child in SubtaskIDs
	updatedParent, err := store.GetTask(createdParent.ID)
	if err != nil {
		t.Fatalf("Failed to get updated parent: %v", err)
	}

	if len(updatedParent.SubtaskIDs) != 1 {
		t.Errorf("Expected 1 subtask, got %d", len(updatedParent.SubtaskIDs))
	}

	if updatedParent.SubtaskIDs[0] != createdChild.ID {
		t.Errorf("Subtask ID incorrect: got %s, want %s", updatedParent.SubtaskIDs[0], createdChild.ID)
	}
}

func TestFileTaskStore_Dependencies(t *testing.T) {
	store := setupTestStore(t)
	defer func() { _ = store.Close() }()

	// Create first task
	task1 := models.Task{
		Title:    "Task 1",
		Status:   models.StatusTodo,
		Priority: models.PriorityMedium,
	}

	created1, err := store.CreateTask(task1)
	if err != nil {
		t.Fatalf("Failed to create task1: %v", err)
	}

	// Create second task that depends on first
	task2 := models.Task{
		Title:        "Task 2",
		Status:       models.StatusTodo,
		Priority:     models.PriorityMedium,
		Dependencies: []string{created1.ID},
	}

	created2, err := store.CreateTask(task2)
	if err != nil {
		t.Fatalf("Failed to create task2: %v", err)
	}

	// Verify dependency was set
	if len(created2.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(created2.Dependencies))
	}

	if created2.Dependencies[0] != created1.ID {
		t.Errorf("Dependency incorrect: got %s, want %s", created2.Dependencies[0], created1.ID)
	}

	// Verify dependent was set on task1
	updated1, err := store.GetTask(created1.ID)
	if err != nil {
		t.Fatalf("Failed to get updated task1: %v", err)
	}

	if len(updated1.Dependents) != 1 {
		t.Errorf("Expected 1 dependent, got %d", len(updated1.Dependents))
	}

	if updated1.Dependents[0] != created2.ID {
		t.Errorf("Dependent incorrect: got %s, want %s", updated1.Dependents[0], created2.ID)
	}
}

func TestFileTaskStore_FilterAndSort(t *testing.T) {
	store := setupTestStore(t)
	defer func() { _ = store.Close() }()

	// Create multiple tasks with different statuses and priorities
	tasks := []models.Task{
		{Title: "High Priority Task", Status: models.StatusTodo, Priority: models.PriorityHigh},
		{Title: "Medium Priority Task", Status: models.StatusDoing, Priority: models.PriorityMedium},
		{Title: "Low Priority Task", Status: models.StatusDone, Priority: models.PriorityLow},
	}

	for _, task := range tasks {
		_, err := store.CreateTask(task)
		if err != nil {
			t.Fatalf("Failed to create task %s: %v", task.Title, err)
		}
	}

	// Test filter by status
	todoFilter := func(task models.Task) bool {
		return task.Status == models.StatusTodo
	}

	todoTasks, err := store.ListTasks(todoFilter, nil)
	if err != nil {
		t.Fatalf("ListTasks with filter failed: %v", err)
	}

	if len(todoTasks) != 1 {
		t.Errorf("Expected 1 todo task, got %d", len(todoTasks))
	}

	if todoTasks[0].Title != "High Priority Task" {
		t.Errorf("Wrong task filtered: got %s, want High Priority Task", todoTasks[0].Title)
	}
}

func TestFileTaskStore_DeleteAllTasks(t *testing.T) {
	store := setupTestStore(t)
	defer func() { _ = store.Close() }()

	// Create some tasks
	for i := 0; i < 3; i++ {
		task := models.Task{
			Title:    "Task " + string(rune('A'+i)),
			Status:   models.StatusTodo,
			Priority: models.PriorityMedium,
		}
		_, err := store.CreateTask(task)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}
	}

	// Verify tasks exist
	tasks, err := store.ListTasks(nil, nil)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	// Delete all tasks
	err = store.DeleteAllTasks()
	if err != nil {
		t.Fatalf("DeleteAllTasks failed: %v", err)
	}

	// Verify all tasks are deleted
	tasks, err = store.ListTasks(nil, nil)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks after DeleteAllTasks, got %d", len(tasks))
	}
}
