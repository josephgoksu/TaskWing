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
	StatusBlocked    TaskStatus = "blocked"     // Blocked by external dependency or issue
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

	// AI integration fields - for MCP tool context fetching
	Scope                  string   `json:"scope,omitempty"`                  // e.g., "auth", "api", "vectorsearch"
	Keywords               []string `json:"keywords,omitempty"`               // Extracted from title/description
	SuggestedRecallQueries []string `json:"suggestedRecallQueries,omitempty"` // Pre-computed queries for recall tool

	// Session tracking - for AI tool state management
	ClaimedBy   string    `json:"claimedBy,omitempty"`   // Session ID that claimed this task
	ClaimedAt   time.Time `json:"claimedAt,omitempty"`   // When the task was claimed
	CompletedAt time.Time `json:"completedAt,omitempty"` // When the task was completed

	// Completion tracking
	CompletionSummary string   `json:"completionSummary,omitempty"` // AI-generated summary on completion
	FilesModified     []string `json:"filesModified,omitempty"`     // Files touched during task

	// Block tracking
	BlockReason string `json:"blockReason,omitempty"` // Reason if task is blocked

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
