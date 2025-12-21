/*
Package spec provides data models and storage for feature specifications.
*/
package spec

import (
	"time"
)

// Status represents the state of a spec or task
type Status string

const (
	StatusDraft      Status = "draft"
	StatusApproved   Status = "approved"
	StatusInProgress Status = "in-progress"
	StatusDone       Status = "done"
)

// Spec represents a feature specification
type Spec struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Agent outputs (stored as markdown)
	PMAnalysis           string `json:"pm_analysis,omitempty"`
	ArchitectAnalysis    string `json:"architect_analysis,omitempty"`
	EngineerAnalysis     string `json:"engineer_analysis,omitempty"`
	QAAnalysis           string `json:"qa_analysis,omitempty"`
	MonetizationAnalysis string `json:"monetization_analysis,omitempty"`
	UXAnalysis           string `json:"ux_analysis,omitempty"`

	// Tasks extracted from engineer analysis
	Tasks []Task `json:"tasks,omitempty"`
}

// Task represents a single development task
type Task struct {
	ID          string    `json:"id"`
	SpecID      string    `json:"spec_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Priority    int       `json:"priority"`
	Estimate    string    `json:"estimate"`
	Files       []string  `json:"files,omitempty"`
	DependsOn   []string  `json:"depends_on,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// SpecSummary is a lightweight view for listings
type SpecSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    Status    `json:"status"`
	TaskCount int       `json:"task_count"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskSummary is a lightweight view for listings
type TaskSummary struct {
	ID       string `json:"id"`
	SpecID   string `json:"spec_id"`
	Title    string `json:"title"`
	Status   Status `json:"status"`
	Priority int    `json:"priority"`
	Estimate string `json:"estimate"`
}

// TaskContext provides full context for AI tools when starting a task
type TaskContext struct {
	Task         Task     `json:"task"`
	SpecTitle    string   `json:"spec_title"`
	SpecContent  string   `json:"spec_content"`
	RelevantCode []string `json:"relevant_code,omitempty"`
	Patterns     []string `json:"patterns,omitempty"`
	Decisions    []string `json:"decisions,omitempty"`
}
