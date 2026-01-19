// Package mcp provides types and utilities for the MCP server.
package mcp

// === Action Constants ===

// CodeAction defines the valid actions for the unified code tool.
type CodeAction string

const (
	CodeActionFind     CodeAction = "find"
	CodeActionSearch   CodeAction = "search"
	CodeActionExplain  CodeAction = "explain"
	CodeActionCallers  CodeAction = "callers"
	CodeActionImpact   CodeAction = "impact"
	CodeActionSimplify CodeAction = "simplify"
)

// ValidCodeActions returns all valid code actions.
func ValidCodeActions() []CodeAction {
	return []CodeAction{CodeActionFind, CodeActionSearch, CodeActionExplain, CodeActionCallers, CodeActionImpact, CodeActionSimplify}
}

// IsValid checks if the action is a valid code action.
func (a CodeAction) IsValid() bool {
	switch a {
	case CodeActionFind, CodeActionSearch, CodeActionExplain, CodeActionCallers, CodeActionImpact, CodeActionSimplify:
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
// Consolidates: find_symbol, semantic_search_code, explain_symbol, get_callers, analyze_impact, simplify
type CodeToolParams struct {
	// Action specifies which operation to perform.
	// Required. One of: find, search, explain, callers, impact, simplify
	Action CodeAction `json:"action"`

	// Query is the symbol name or search query.
	// Required for: search, explain (if symbol_id not provided)
	// Optional for: find (alternative to symbol_id), callers, impact
	Query string `json:"query,omitempty"`

	// SymbolID is the direct symbol ID for precise lookups.
	// Optional. If provided, takes precedence over query for find/explain/callers/impact.
	SymbolID uint32 `json:"symbol_id,omitempty"`

	// FilePath filters results to a specific file or directory.
	// Required for: simplify (specifies file to simplify)
	// Optional for: find, search
	FilePath string `json:"file_path,omitempty"`

	// Code is the source code to process.
	// Required for: simplify (if file_path not provided)
	Code string `json:"code,omitempty"`

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

// DebugToolParams defines the parameters for the debug tool.
type DebugToolParams struct {
	// Problem is the description of the issue.
	// Required.
	Problem string `json:"problem"`

	// Error is the error message (if available).
	// Optional.
	Error string `json:"error,omitempty"`

	// StackTrace is the stack trace (if available).
	// Optional.
	StackTrace string `json:"stack_trace,omitempty"`

	// FilePath is a file related to the issue.
	// Optional. If provided, context will be fetched for that file.
	FilePath string `json:"file_path,omitempty"`
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

// PolicyAction defines the valid actions for the unified policy tool.
type PolicyAction string

const (
	PolicyActionCheck   PolicyAction = "check"
	PolicyActionList    PolicyAction = "list"
	PolicyActionExplain PolicyAction = "explain"
)

// ValidPolicyActions returns all valid policy actions.
func ValidPolicyActions() []PolicyAction {
	return []PolicyAction{PolicyActionCheck, PolicyActionList, PolicyActionExplain}
}

// IsValid checks if the action is a valid policy action.
func (a PolicyAction) IsValid() bool {
	switch a {
	case PolicyActionCheck, PolicyActionList, PolicyActionExplain:
		return true
	}
	return false
}

// PolicyToolParams defines the parameters for the unified policy tool.
// Consolidates: policy check, policy list, policy explain
type PolicyToolParams struct {
	// Action specifies which operation to perform.
	// Required. One of: check, list, explain
	Action PolicyAction `json:"action"`

	// Files is a list of file paths to check against policies.
	// Required for: check
	Files []string `json:"files,omitempty"`

	// TaskID is the task context for policy evaluation.
	// Optional for: check (provides task metadata to policies)
	TaskID string `json:"task_id,omitempty"`

	// TaskTitle is the task title for policy evaluation.
	// Optional for: check
	TaskTitle string `json:"task_title,omitempty"`

	// PlanID is the plan context for policy evaluation.
	// Optional for: check
	PlanID string `json:"plan_id,omitempty"`

	// PlanGoal is the plan goal for policy evaluation.
	// Optional for: check
	PlanGoal string `json:"plan_goal,omitempty"`

	// PolicyName is the name of a specific policy to explain.
	// Optional for: explain (if not provided, lists all rules)
	PolicyName string `json:"policy_name,omitempty"`
}
