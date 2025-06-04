package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskSerialization(t *testing.T) {
	id := uuid.New().String()
	now := time.Now().UTC().Truncate(time.Second) // Truncate for stable comparison

	task := Task{
		ID:           id,
		Title:        "Test Task Title",
		Description:  "This is a test description.",
		Status:       StatusInProgress,
		Dependencies: []string{uuid.New().String(), uuid.New().String()},
		Priority:     PriorityHigh,
		Details:      "Some specific details here.",
		TestStrategy: "Unit tests and integration tests.",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Test Marshaling
	jsonData, err := json.Marshal(task)
	require.NoError(t, err, "JSON marshaling should not fail")

	// Basic check: is it valid JSON?
	assert.True(t, json.Valid(jsonData), "Marshaled data should be valid JSON")

	// Test Unmarshaling
	var unmarshaledTask Task
	err = json.Unmarshal(jsonData, &unmarshaledTask)
	require.NoError(t, err, "JSON unmarshaling should not fail")

	// Compare fields
	// For time.Time, ensure they are compared in UTC and potentially with truncation if sub-second precision is an issue.
	unmarshaledTask.CreatedAt = unmarshaledTask.CreatedAt.UTC()
	unmarshaledTask.UpdatedAt = unmarshaledTask.UpdatedAt.UTC()

	assert.Equal(t, task.ID, unmarshaledTask.ID)
	assert.Equal(t, task.Title, unmarshaledTask.Title)
	assert.Equal(t, task.Description, unmarshaledTask.Description)
	assert.Equal(t, task.Status, unmarshaledTask.Status)
	assert.Equal(t, task.Dependencies, unmarshaledTask.Dependencies)
	assert.Equal(t, task.Priority, unmarshaledTask.Priority)
	assert.Equal(t, task.Details, unmarshaledTask.Details)
	assert.Equal(t, task.TestStrategy, unmarshaledTask.TestStrategy)
	assert.True(t, task.CreatedAt.Equal(unmarshaledTask.CreatedAt), "CreatedAt should match")
	assert.True(t, task.UpdatedAt.Equal(unmarshaledTask.UpdatedAt), "UpdatedAt should match")
}

func TestTaskValidation(t *testing.T) {
	now := time.Now()
	// due := now.Add(24 * time.Hour) // DueDate field removed from Task struct, so 'due' is unused.

	t.Run("Valid Task", func(t *testing.T) {
		task := Task{
			ID:           uuid.New().String(),
			Title:        "Valid Task",
			Status:       StatusPending,
			Priority:     PriorityMedium,
			CreatedAt:    now,
			UpdatedAt:    now,
			Dependencies: []string{uuid.New().String()},
		}
		err := ValidateStruct(&task)
		assert.NoError(t, err, "Validation should pass for a valid task")
	})

	t.Run("Invalid Task - Missing Required Fields", func(t *testing.T) {
		task := Task{ // Missing Title, Status, Priority, CreatedAt, UpdatedAt
			ID: uuid.New().String(),
		}
		err := ValidateStruct(&task)
		assert.Error(t, err, "Validation should fail for missing required fields")
		t.Log("Validation error:", err) // Log error for inspection
		assert.Contains(t, err.Error(), "Title", "Error message should mention Title")
		assert.Contains(t, err.Error(), "Status", "Error message should mention Status")
		assert.Contains(t, err.Error(), "Priority", "Error message should mention Priority")
	})

	t.Run("Invalid Task - Bad Field Values", func(t *testing.T) {
		task := Task{
			ID:           "not-a-uuid",
			Title:        "T", // Too short
			Status:       "invalid-status",
			Priority:     "super-low",
			CreatedAt:    now,
			UpdatedAt:    now,
			Dependencies: []string{"not-a-uuid-either"},
		}
		err := ValidateStruct(&task)
		assert.Error(t, err, "Validation should fail for bad field values")
		t.Log("Validation error:", err) // Log error for inspection
		assert.Contains(t, err.Error(), "ID", "Error message should mention ID for uuid")
		assert.Contains(t, err.Error(), "Title", "Error message should mention Title for min length")
		assert.Contains(t, err.Error(), "Status", "Error message should mention Status for oneof")
		assert.Contains(t, err.Error(), "Priority", "Error message should mention Priority for oneof")
		assert.Contains(t, err.Error(), "Dependencies", "Error message should mention Dependencies for dive uuid4")
	})

	t.Run("Valid TaskList and Metadata", func(t *testing.T) {
		taskList := TaskList{
			Tasks: []Task{
				{
					ID:        uuid.New().String(),
					Title:     "Sub Task",
					Status:    StatusPending,
					Priority:  PriorityLow,
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
			TotalCount: 1,
		}
		err := ValidateStruct(&taskList)
		assert.NoError(t, err, "Validation should pass for a valid task list")

		metadata := Metadata{
			SchemaVersion: "1.0.0",
			ExportedAt:    time.Now(),
		}
		err = ValidateStruct(&metadata)
		assert.NoError(t, err, "Validation should pass for valid metadata")
	})

}

func TestNewTaskDefaults(t *testing.T) {
	id := uuid.New().String()
	title := "Newly Created Task"
	task := NewTask(id, title)

	assert.Equal(t, id, task.ID)
	assert.Equal(t, title, task.Title)
	assert.Equal(t, StatusPending, task.Status, "Default status should be pending")
	assert.Equal(t, PriorityMedium, task.Priority, "Default priority should be medium")
	assert.NotZero(t, task.CreatedAt, "CreatedAt should be set")
	assert.NotZero(t, task.UpdatedAt, "UpdatedAt should be set")
	assert.True(t, task.CreatedAt.Equal(task.UpdatedAt), "CreatedAt and UpdatedAt should be equal on creation")

	err := ValidateStruct(task)
	assert.NoError(t, err, "Newly created task should be valid")
}
