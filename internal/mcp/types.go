// Package mcp provides types and utilities for the MCP server.
package mcp

// === Action Constants ===

// CodeAction defines the valid actions for the unified code tool.
type CodeAction string

const (
	CodeActionFind    CodeAction = "find"
	CodeActionSearch  CodeAction = "search"
	CodeActionExplain CodeAction = "explain"
	CodeActionCallers CodeAction = "callers"
	CodeActionImpact  CodeAction = "impact"
)

// ValidCodeActions returns all valid code actions.
func ValidCodeActions() []CodeAction {
	return []CodeAction{CodeActionFind, CodeActionSearch, CodeActionExplain, CodeActionCallers, CodeActionImpact}
}

// IsValid checks if the action is a valid code action.
func (a CodeAction) IsValid() bool {
	switch a {
	case CodeActionFind, CodeActionSearch, CodeActionExplain, CodeActionCallers, CodeActionImpact:
		return true
	}
	return false
}

// TaskAction defines the valid actions for the unified task tool.
type TaskAction string

const (
	TaskActionNext     TaskAction = "next"
	TaskActionCurrent  TaskAction = "current"
	TaskActionStart    TaskAction = "start"
	TaskActionComplete TaskAction = "complete"
)

// ValidTaskActions returns all valid task actions.
func ValidTaskActions() []TaskAction {
	return []TaskAction{TaskActionNext, TaskActionCurrent, TaskActionStart, TaskActionComplete}
}

// IsValid checks if the action is a valid task action.
func (a TaskAction) IsValid() bool {
	switch a {
	case TaskActionNext, TaskActionCurrent, TaskActionStart, TaskActionComplete:
		return true
	}
	return false
}

// PlanAction defines the valid actions for the unified plan tool.
type PlanAction string

const (
	PlanActionClarify  PlanAction = "clarify"
	PlanActionGenerate PlanAction = "generate"
	PlanActionAudit    PlanAction = "audit"
)

// ValidPlanActions returns all valid plan actions.
func ValidPlanActions() []PlanAction {
	return []PlanAction{PlanActionClarify, PlanActionGenerate, PlanActionAudit}
}

// IsValid checks if the action is a valid plan action.
func (a PlanAction) IsValid() bool {
	switch a {
	case PlanActionClarify, PlanActionGenerate, PlanActionAudit:
		return true
	}
	return false
}

// === Unified Tool Parameters ===

// CodeToolParams defines the parameters for the unified code tool.
// Consolidates: find_symbol, semantic_search_code, explain_symbol, get_callers, analyze_impact
type CodeToolParams struct {
	// Action specifies which operation to perform.
	// Required. One of: find, search, explain, callers, impact
	Action CodeAction `json:"action"`

	// Query is the symbol name or search query.
	// Required for: search, explain (if symbol_id not provided)
	// Optional for: find (alternative to symbol_id), callers, impact
	Query string `json:"query,omitempty"`

	// SymbolID is the direct symbol ID for precise lookups.
	// Optional. If provided, takes precedence over query for find/explain/callers/impact.
	SymbolID uint32 `json:"symbol_id,omitempty"`

	// FilePath filters results to a specific file or directory.
	// Optional for: find, search
	FilePath string `json:"file_path,omitempty"`

	// Language filters results by programming language (e.g., "go", "typescript").
	// Optional for: find
	Language string `json:"language,omitempty"`

	// Kind filters by symbol kind (function, struct, interface, etc.).
	// Optional for: search
	Kind string `json:"kind,omitempty"`

	// Limit is the maximum number of results to return.
	// Optional for: search (default: 20)
	Limit int `json:"limit,omitempty"`

	// Direction specifies call graph direction for callers action.
	// One of: callers, callees, both (default: both)
	// Optional for: callers
	Direction string `json:"direction,omitempty"`

	// MaxDepth is the maximum recursion depth for impact analysis.
	// Optional for: impact (default: 5)
	MaxDepth int `json:"max_depth,omitempty"`

	// Depth is the call graph depth for explain action.
	// Optional for: explain (default: 2, range: 1-5)
	Depth int `json:"depth,omitempty"`
}

// TaskToolParams defines the parameters for the unified task tool.
// Consolidates: task_next, task_current, task_start, task_complete
type TaskToolParams struct {
	// Action specifies which operation to perform.
	// Required. One of: next, current, start, complete
	Action TaskAction `json:"action"`

	// TaskID is the task identifier.
	// Required for: start, complete
	TaskID string `json:"task_id,omitempty"`

	// PlanID is the plan identifier.
	// Optional for: next, current (defaults to active plan)
	PlanID string `json:"plan_id,omitempty"`

	// SessionID is the unique AI session identifier.
	// Required for: next, current, start
	SessionID string `json:"session_id,omitempty"`

	// Summary describes what was accomplished.
	// Optional for: complete
	Summary string `json:"summary,omitempty"`

	// FilesModified lists files that were changed.
	// Optional for: complete
	FilesModified []string `json:"files_modified,omitempty"`

	// AutoStart automatically claims the next task.
	// Optional for: next (default: false)
	AutoStart bool `json:"auto_start,omitempty"`

	// CreateBranch creates a new git branch for this plan.
	// Optional for: next (default: false)
	CreateBranch bool `json:"create_branch,omitempty"`

	// SkipUnpushedCheck proceeds despite unpushed commits.
	// Optional for: next (only if create_branch=true)
	SkipUnpushedCheck bool `json:"skip_unpushed_check,omitempty"`
}

// === MCP Tool Parameters (non-unified) ===

// ProjectContextParams defines the parameters for the recall tool.
type ProjectContextParams struct {
	Query  string `json:"query,omitempty"`
	Answer bool   `json:"answer,omitempty"` // If true, generate RAG answer using LLM
}

// RememberParams defines the parameters for the remember tool.
type RememberParams struct {
	Content string `json:"content"`        // Required: knowledge to store
	Type    string `json:"type,omitempty"` // Optional: decision, feature, plan, note
}

// PlanToolParams defines the parameters for the unified plan tool.
// Consolidates: plan_clarify, plan_generate, audit_plan
type PlanToolParams struct {
	// Action specifies which operation to perform.
	// Required. One of: clarify, generate, audit
	Action PlanAction `json:"action"`

	// Goal is the user's development goal.
	// Required for: clarify, generate
	Goal string `json:"goal,omitempty"`

	// EnrichedGoal is the full technical specification from clarify.
	// Required for: generate
	EnrichedGoal string `json:"enriched_goal,omitempty"`

	// History is a JSON array of previous Q&A from clarify loop.
	// Optional for: clarify (format: [{"q": "...", "a": "..."}, ...])
	History string `json:"history,omitempty"`

	// AutoAnswer uses knowledge graph to auto-answer clarifying questions.
	// Optional for: clarify (default: false)
	AutoAnswer bool `json:"auto_answer,omitempty"`

	// Save persists the generated plan to the database.
	// Optional for: generate (default: true)
	Save *bool `json:"save,omitempty"`

	// PlanID is the plan to audit.
	// Optional for: audit (defaults to active plan)
	PlanID string `json:"plan_id,omitempty"`

	// AutoFix attempts to automatically fix failures.
	// Optional for: audit (default: true)
	AutoFix *bool `json:"auto_fix,omitempty"`
}
