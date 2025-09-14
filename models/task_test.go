package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTask_ValidateStruct(t *testing.T) {
	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name: "valid task",
			task: Task{
				ID:        uuid.New().String(),
				Title:     "Valid Task Title",
				Status:    StatusTodo,
				Priority:  PriorityMedium,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty title",
			task: Task{
				ID:        uuid.New().String(),
				Title:     "",
				Status:    StatusTodo,
				Priority:  PriorityMedium,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "title too short",
			task: Task{
				ID:        uuid.New().String(),
				Title:     "ab", // Less than 3 characters
				Status:    StatusTodo,
				Priority:  PriorityMedium,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			task: Task{
				ID:        uuid.New().String(),
				Title:     "Valid Task Title",
				Status:    "invalid-status",
				Priority:  PriorityMedium,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid priority",
			task: Task{
				ID:        uuid.New().String(),
				Title:     "Valid Task Title",
				Status:    StatusTodo,
				Priority:  "invalid-priority",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid UUID",
			task: Task{
				ID:        "not-a-uuid",
				Title:     "Valid Task Title",
				Status:    StatusTodo,
				Priority:  PriorityMedium,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStruct(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStruct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCoreStatuses(t *testing.T) {
	statuses := CoreStatuses()
	expected := []TaskStatus{StatusTodo, StatusDoing, StatusReview, StatusDone}

	if len(statuses) != len(expected) {
		t.Errorf("expected %d statuses, got %d", len(expected), len(statuses))
	}

	for i, status := range statuses {
		if status != expected[i] {
			t.Errorf("expected status %q at index %d, got %q", expected[i], i, status)
		}
	}
}

func TestTask_JSONSerialization(t *testing.T) {
	now := time.Now()
	original := Task{
		ID:          uuid.New().String(),
		Title:       "Test Task",
		Description: "Test Description",
		Status:      StatusDoing,
		Priority:    PriorityHigh,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Serialize to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal task: %v", err)
	}

	// Deserialize from JSON
	var restored Task
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal task: %v", err)
	}

	// Verify all fields match
	if restored.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", restored.ID, original.ID)
	}
	if restored.Title != original.Title {
		t.Errorf("Title mismatch: got %q, want %q", restored.Title, original.Title)
	}
	if restored.Description != original.Description {
		t.Errorf("Description mismatch: got %q, want %q", restored.Description, original.Description)
	}
	if restored.Status != original.Status {
		t.Errorf("Status mismatch: got %q, want %q", restored.Status, original.Status)
	}
	if restored.Priority != original.Priority {
		t.Errorf("Priority mismatch: got %q, want %q", restored.Priority, original.Priority)
	}
}

func TestValidateStatusTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     TaskStatus
		to       TaskStatus
		expected bool
	}{
		{"todo to doing", StatusTodo, StatusDoing, true},
		{"todo to done", StatusTodo, StatusDone, true},
		{"doing to review", StatusDoing, StatusReview, true},
		{"doing to todo", StatusDoing, StatusTodo, true},
		{"review to done", StatusReview, StatusDone, true},
		{"done to todo", StatusDone, StatusTodo, true},
		{"todo to review", StatusTodo, StatusReview, false},
		{"review to doing", StatusReview, StatusDoing, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateStatusTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("ValidateStatusTransition(%q, %q) = %v, want %v",
					tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestTaskList_Validation(t *testing.T) {
	validTask := Task{
		ID:        uuid.New().String(),
		Title:     "Valid Task",
		Status:    StatusTodo,
		Priority:  PriorityMedium,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	taskList := TaskList{
		Tasks:      []Task{validTask},
		TotalCount: 1,
	}

	err := ValidateStruct(taskList)
	if err != nil {
		t.Errorf("ValidateStruct() error = %v, expected no error", err)
	}

	// Test with invalid task
	invalidTask := Task{
		ID:     "invalid-uuid",
		Title:  "", // Empty title should fail validation
		Status: StatusTodo,
	}

	taskListInvalid := TaskList{
		Tasks:      []Task{invalidTask},
		TotalCount: 1,
	}

	err = ValidateStruct(taskListInvalid)
	if err == nil {
		t.Error("expected validation error for task list with invalid task")
	}
}
