package models

import (
	"fmt"
	"strings"
	"time"

	validator "github.com/go-playground/validator/v10"
)

// TaskStatus represents the possible statuses of a task.
type TaskStatus string

const (
	// New minimal status system
	StatusTodo   TaskStatus = "todo"
	StatusDoing  TaskStatus = "doing"
	StatusReview TaskStatus = "review"
	StatusDone   TaskStatus = "done"
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
	ID                 string       `json:"id" validate:"required,uuid4"`
	Title              string       `json:"title" validate:"required,min=3,max=255"`
	Description        string       `json:"description,omitempty"`
	AcceptanceCriteria string       `json:"acceptanceCriteria,omitempty"`
	Status             TaskStatus   `json:"status" validate:"required,oneof=todo doing review done"`
	ParentID           *string      `json:"parentId,omitempty" validate:"omitempty,uuid4"` // ID of the parent task
	SubtaskIDs         []string     `json:"subtaskIds,omitempty" validate:"dive,uuid4"`    // IDs of direct children tasks
	Dependencies       []string     `json:"dependencies,omitempty" validate:"dive,uuid4"`  // Slice of Task IDs this task depends on
	Dependents         []string     `json:"dependents,omitempty" validate:"dive,uuid4"`    // Slice of Task IDs that depend on this task (managed internally)
	Priority           TaskPriority `json:"priority" validate:"required,oneof=low medium high urgent"`
	CreatedAt          time.Time    `json:"createdAt" validate:"required"`
	UpdatedAt          time.Time    `json:"updatedAt" validate:"required"`
	CompletedAt        *time.Time   `json:"completedAt,omitempty"` // Optional: pointer to allow null
}

// TaskList represents a collection of tasks.
type TaskList struct {
	Tasks []Task `json:"tasks" validate:"dive"`
	// Could add pagination metadata here in the future
	TotalCount int `json:"totalCount"`
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"perPage,omitempty"`
}

// CoreStatuses returns the 4 core statuses in workflow order
func CoreStatuses() []TaskStatus {
	return []TaskStatus{StatusTodo, StatusDoing, StatusReview, StatusDone}
}

// ValidateStatusTransition checks if a status transition is valid
func ValidateStatusTransition(from, to TaskStatus) bool {
	// Define valid transitions
	validTransitions := map[TaskStatus][]TaskStatus{
		StatusTodo:   {StatusDoing, StatusDone},
		StatusDoing:  {StatusReview, StatusTodo, StatusDone},
		StatusReview: {StatusDoing, StatusDone, StatusTodo},
		StatusDone:   {StatusTodo}, // Allow reopening tasks
	}

	allowedTransitions, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, allowed := range allowedTransitions {
		if allowed == to {
			return true
		}
	}
	return false
}

// global validator instance
var validate *validator.Validate

func init() {
	validate = validator.New()
	// Register custom validation functions here if needed
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
