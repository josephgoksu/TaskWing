package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// TaskStatus represents the possible statuses of a task.
type TaskStatus string

const (
	StatusPending     TaskStatus = "pending"
	StatusInProgress  TaskStatus = "in-progress"
	StatusCompleted   TaskStatus = "completed"
	StatusCancelled   TaskStatus = "cancelled"
	StatusOnHold      TaskStatus = "on-hold"
	StatusBlocked     TaskStatus = "blocked"
	StatusNeedsReview TaskStatus = "needs-review"
)

// TaskPriority represents the priority levels of a task.
type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
	PriorityUrgent TaskPriority = "urgent"
)

// Task represents a unit of work.
type Task struct {
	ID           string       `json:"id" validate:"required,uuid4"`
	Title        string       `json:"title" validate:"required,min=3,max=255"`
	Description  string       `json:"description,omitempty"`
	Status       TaskStatus   `json:"status" validate:"required,oneof=pending in-progress completed cancelled on-hold blocked needs-review"`
	ParentID     *string      `json:"parentId,omitempty" validate:"omitempty,uuid4"` // ID of the parent task
	SubtaskIDs   []string     `json:"subtaskIds,omitempty" validate:"dive,uuid4"`    // IDs of direct children tasks
	Dependencies []string     `json:"dependencies,omitempty" validate:"dive,uuid4"`  // Slice of Task IDs this task depends on
	Dependents   []string     `json:"dependents,omitempty" validate:"dive,uuid4"`    // Slice of Task IDs that depend on this task (managed internally)
	Priority     TaskPriority `json:"priority" validate:"required,oneof=low medium high urgent"`
	Details      string       `json:"details,omitempty"`
	TestStrategy string       `json:"testStrategy,omitempty"`
	CreatedAt    time.Time    `json:"createdAt" validate:"required"`
	UpdatedAt    time.Time    `json:"updatedAt" validate:"required"`
	CompletedAt  *time.Time   `json:"completedAt,omitempty"` // Optional: pointer to allow null
}

// TaskList represents a collection of tasks.
type TaskList struct {
	Tasks []Task `json:"tasks" validate:"dive"`
	// Could add pagination metadata here in the future
	TotalCount int `json:"totalCount"`
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"perPage,omitempty"`
}

// Metadata could be used for various purposes, e.g., versioning, context.
// For now, it's a placeholder.
type Metadata struct {
	SchemaVersion string    `json:"schemaVersion" validate:"required,semver"`
	ExportedAt    time.Time `json:"exportedAt" validate:"required"`
	Source        string    `json:"source,omitempty"` // e.g., name of the application generating this
}

// global validator instance
var validate *validator.Validate

func init() {
	validate = validator.New()
	// Register custom validation functions here if needed
	// validate.RegisterValidation("tagformat", validateTagFormat)
	// e.g., validate.RegisterValidation("status_transition_valid", validateStatusTransition)
}

// ValidateStruct performs validation on any struct that has validation tags.
func ValidateStruct(s interface{}) error {
	if validate == nil {
		// This should ideally not happen if init() runs correctly,
		// but as a safeguard or for tests running in isolation.
		validate = validator.New()
	}
	err := validate.Struct(s)
	if err != nil {
		// Optionally, format validation errors for better readability
		validationErrors := err.(validator.ValidationErrors)
		var errorMessages []string
		for _, e := range validationErrors {
			// Customize error messages based on e.g. e.Tag(), e.Field(), e.Param()
			errorMessages = append(errorMessages, fmt.Sprintf("Validation failed on field '%s': rule '%s' (value: '%v')", e.StructNamespace(), e.Tag(), e.Value()))
		}
		return fmt.Errorf("%s", strings.Join(errorMessages, "; ")) // Simplified error return
	}
	return nil
}

// Example custom validation: validate task dependencies (e.g., check for circular dependencies - complex, placeholder for now)
// For this to be useful, we'd likely need access to all other tasks.
// A simpler validation here could be to ensure dependency IDs are valid UUIDs if not already covered by 'dive,uuid4'.
// The current 'dive,uuid4' on Dependencies field already validates this for each string in the slice.

// Example custom validation: validate status transitions
// This would require knowing the OLD status and the NEW status.
// This type of validation is typically done at the service/logic layer, not on the model itself during unmarshaling.
// However, we can register a custom validation tag if a specific field needs context-aware validation.

// For demonstration, let's assume a function that could be registered if needed:
// func validateStatusTransition(fl validator.FieldLevel) bool {
// 	 newStatus := fl.Field().String()
// 	 // Here, you'd need the old status. This is tricky with struct-level validation tags
// 	 // without passing the whole object or using struct-level validation.
// 	 // For now, let's say any of the defined statuses are valid individually (already handled by 'oneof').
// 	 // A true transition logic (e.g., cannot go from 'completed' to 'pending') is more complex here.
// 	 return true // Placeholder
// }

// Helper function to create a new task with default CreatedAt/UpdatedAt.
// This is not strictly part of struct definition but useful.
func NewTask(id, title string) *Task {
	now := time.Now()
	return &Task{
		ID:           id,
		Title:        title,
		Status:       StatusPending,
		Priority:     PriorityMedium,
		CreatedAt:    now,
		UpdatedAt:    now,
		ParentID:     nil,        // Initialize ParentID as nil
		SubtaskIDs:   []string{}, // Initialize SubtaskIDs as empty slice
		Dependencies: []string{},
		Dependents:   []string{},
	}
}

// validateTagFormat checks if the tag string contains only alphanumeric characters, hyphens, or underscores.
/*
func validateTagFormat(fl validator.FieldLevel) bool {
	// Regex to allow alphanumeric, hyphen, underscore. Min length 1 is handled by "min=1" tag.
	// Tag must not be empty, and must not contain spaces or other special characters.
	return regexp.MustCompile(`^[a-zA-Z0-9_ -]+$`).MatchString(fl.Field().String()) // Allow spaces
}
*/
