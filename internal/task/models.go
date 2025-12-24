package task

import (
	"time"
)

// TaskStatus represents the lifecycle state of a task
type TaskStatus string

const (
	StatusDraft      TaskStatus = "draft"       // Initial creation, not ready for execution
	StatusPending    TaskStatus = "pending"     // Ready to be picked up by an agent
	StatusInProgress TaskStatus = "in_progress" // Agent is actively working
	StatusVerifying  TaskStatus = "verifying"   // Work done, running validation
	StatusCompleted  TaskStatus = "completed"   // Successfully verified
	StatusFailed     TaskStatus = "failed"      // Execution or verification failed
)

// PlanStatus represents the lifecycle state of a plan
type PlanStatus string

const (
	PlanStatusDraft     PlanStatus = "draft"     // Initial creation
	PlanStatusActive    PlanStatus = "active"    // Currently being executed
	PlanStatusCompleted PlanStatus = "completed" // All tasks done
	PlanStatusArchived  PlanStatus = "archived"  // No longer active
)

// Task represents a discrete unit of work to be executed by an agent
type Task struct {
	ID                 string     `json:"id"`
	PlanID             string     `json:"planId"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Status             TaskStatus `json:"status"`
	Priority           int        `json:"priority"` // 0-100 (High to Low)
	AssignedAgent      string     `json:"assignedAgent"`
	ParentTaskID       string     `json:"parentTaskId,omitempty"`
	ContextSummary     string     `json:"contextSummary"` // AI-generated summary of linked nodes
	AcceptanceCriteria []string   `json:"acceptanceCriteria"`
	ValidationSteps    []string   `json:"validationSteps"` // CLI commands

	// Computed/Joined fields (not in tasks table directly)
	Dependencies []string `json:"dependencies"` // IDs of tasks
	ContextNodes []string `json:"contextNodes"` // IDs of knowledge nodes

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Plan represents a collection of tasks to achieve a high-level goal
type Plan struct {
	ID           string     `json:"id"`
	Goal         string     `json:"goal"`         // Initial user intent
	EnrichedGoal string     `json:"enrichedGoal"` // Refined by Clarifying Agent
	Status       PlanStatus `json:"status"`       // draft, active, completed, archived
	Tasks        []Task     `json:"tasks"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}
