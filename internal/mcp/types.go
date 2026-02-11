// Package mcp provides types and utilities for the MCP server.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// planIDMCPDeprecationWarned ensures we only log the MCP deprecation warning once per process.
var planIDMCPDeprecationWarned sync.Once

// warnPlanIDMCPDeprecation logs a deprecation warning once per process run.
func warnPlanIDMCPDeprecation() {
	planIDMCPDeprecationWarned.Do(func() {
		fmt.Fprintln(os.Stderr, "DEPRECATION WARNING: MCP parameter 'planId' is deprecated, use 'plan_id' instead")
	})
}

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
	PlanActionClarify   PlanAction = "clarify"   // Refine goal with questions (Stage 1)
	PlanActionDecompose PlanAction = "decompose" // Break goal into phases (Stage 2 - interactive)
	PlanActionExpand    PlanAction = "expand"    // Generate tasks for a phase (Stage 3 - interactive)
	PlanActionGenerate  PlanAction = "generate"  // Generate all tasks at once (batch mode)
	PlanActionFinalize  PlanAction = "finalize"  // Save completed interactive plan (Stage 4)
	PlanActionAudit     PlanAction = "audit"     // Verify plan implementation
)

// ValidPlanActions returns all valid plan actions.
func ValidPlanActions() []PlanAction {
	return []PlanAction{PlanActionClarify, PlanActionDecompose, PlanActionExpand, PlanActionGenerate, PlanActionFinalize, PlanActionAudit}
}

// IsValid checks if the action is a valid plan action.
func (a PlanAction) IsValid() bool {
	switch a {
	case PlanActionClarify, PlanActionDecompose, PlanActionExpand, PlanActionGenerate, PlanActionFinalize, PlanActionAudit:
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
// Supports lifecycle actions: next, current, start, complete
//
// Required fields by action:
//   - next: session_id
//   - current: session_id
//   - start: task_id, session_id
//   - complete: task_id
type TaskToolParams struct {
	// Action specifies which operation to perform.
	// Required. One of: next, current, start, complete
	Action TaskAction `json:"action"`

	// TaskID is the task identifier.
	// REQUIRED for: start, complete (will error if empty for these actions)
	TaskID string `json:"task_id,omitempty"`

	// PlanID is the plan identifier.
	// Optional for: next, current (defaults to active plan)
	// Deprecated alias: planId (still accepted but plan_id is preferred)
	PlanID string `json:"plan_id,omitempty"`

	// SessionID is the unique AI session identifier.
	// REQUIRED for: next, current, start (will error if empty for these actions)
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
	// Optional for: next (default: true)
	CreateBranch *bool `json:"create_branch,omitempty"`

	// SkipUnpushedCheck proceeds despite unpushed commits.
	// Optional for: next (only if create_branch=true)
	SkipUnpushedCheck bool `json:"skip_unpushed_check,omitempty"`
}

// taskToolParamsAlias is used for JSON unmarshaling to accept deprecated planId field.
type taskToolParamsAlias TaskToolParams

// taskToolParamsWithAlias includes both plan_id and deprecated planId for backward compatibility.
type taskToolParamsWithAlias struct {
	taskToolParamsAlias
	PlanIDAlias string `json:"planId,omitempty"` // Deprecated: use plan_id
}

// UnmarshalJSON implements custom unmarshaling to accept deprecated planId field.
func (p *TaskToolParams) UnmarshalJSON(data []byte) error {
	var aux taskToolParamsWithAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*p = TaskToolParams(aux.taskToolParamsAlias)

	// If plan_id is empty but planId alias was provided, use the alias
	if p.PlanID == "" && aux.PlanIDAlias != "" {
		p.PlanID = aux.PlanIDAlias
		warnPlanIDMCPDeprecation()
	}

	return nil
}

// === MCP Tool Parameters (non-unified) ===

// ProjectContextParams defines the parameters for the recall tool.
type ProjectContextParams struct {
	Query     string `json:"query,omitempty"`
	Answer    bool   `json:"answer,omitempty"`    // If true, generate RAG answer using LLM
	Workspace string `json:"workspace,omitempty"` // Filter by workspace (e.g., 'osprey'). Empty = all workspaces.
	All       bool   `json:"all,omitempty"`       // Explicitly search all workspaces (ignore auto-detection)
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

// PhaseInput represents user-provided phase data for interactive mode.
type PhaseInput struct {
	Title         string `json:"title"`
	Description   string `json:"description,omitempty"`
	Rationale     string `json:"rationale,omitempty"`
	ExpectedTasks int    `json:"expected_tasks,omitempty"`
}

// TaskInput represents user-provided task data for interactive mode.
type TaskInput struct {
	Title              string   `json:"title"`
	Description        string   `json:"description,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	ValidationSteps    []string `json:"validation_steps,omitempty"`
	Priority           int      `json:"priority,omitempty"`
	Complexity         string   `json:"complexity,omitempty"`
}

// PlanToolParams defines the parameters for the unified plan tool.
// Supports planning actions: clarify, decompose, expand, generate, finalize, audit
//
// Required fields by action:
//   - clarify: goal
//   - decompose: plan_id (with enriched_goal) OR enriched_goal (creates new plan)
//   - expand: plan_id, phase_id OR phase_index
//   - generate: goal, enriched_goal (call clarify first to get enriched_goal)
//   - finalize: plan_id
//   - audit: none (defaults to active plan)
type PlanToolParams struct {
	// Action specifies which operation to perform.
	// Required. One of: clarify, decompose, expand, generate, finalize, audit
	Action PlanAction `json:"action"`

	// Goal is the user's development goal.
	// REQUIRED for: clarify, generate (will error if empty for these actions)
	Goal string `json:"goal,omitempty"`

	// EnrichedGoal is the full technical specification from clarify.
	// REQUIRED for: generate, decompose (will error if empty; call clarify first to get this)
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

	// PlanID is the plan to operate on.
	// REQUIRED for: expand, finalize
	// Optional for: decompose (creates new plan if not provided), audit (defaults to active plan)
	// Deprecated alias: planId (still accepted but plan_id is preferred)
	PlanID string `json:"plan_id,omitempty"`

	// AutoFix attempts to automatically fix failures.
	// Optional for: audit (default: true)
	AutoFix *bool `json:"auto_fix,omitempty"`

	// === Interactive Mode Fields ===

	// Mode specifies the generation mode.
	// Optional. One of: "interactive", "batch" (default: "batch" for backward compatibility)
	Mode string `json:"mode,omitempty"`

	// PhaseID is the ID of the phase to expand.
	// REQUIRED for: expand (if phase_index not provided)
	PhaseID string `json:"phase_id,omitempty"`

	// PhaseIndex is the 0-based index of the phase to expand.
	// Optional for: expand (alternative to phase_id)
	PhaseIndex *int `json:"phase_index,omitempty"`

	// Phases is user-edited phase data for decompose feedback.
	// Optional for: decompose (allows user modifications)
	Phases []PhaseInput `json:"phases,omitempty"`

	// Tasks is user-edited task data for expand feedback.
	// Optional for: expand (allows user modifications)
	Tasks []TaskInput `json:"tasks,omitempty"`

	// Feedback is a regeneration hint when user wants changes.
	// Optional for: decompose, expand (e.g., "split phase 2 into smaller chunks")
	Feedback string `json:"feedback,omitempty"`
}

// planToolParamsAlias is used for JSON unmarshaling to accept deprecated planId field.
type planToolParamsAlias PlanToolParams

// planToolParamsWithAlias includes both plan_id and deprecated planId for backward compatibility.
type planToolParamsWithAlias struct {
	planToolParamsAlias
	PlanIDAlias string `json:"planId,omitempty"` // Deprecated: use plan_id
}

// UnmarshalJSON implements custom unmarshaling to accept deprecated planId field.
func (p *PlanToolParams) UnmarshalJSON(data []byte) error {
	var aux planToolParamsWithAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*p = PlanToolParams(aux.planToolParamsAlias)

	// If plan_id is empty but planId alias was provided, use the alias
	if p.PlanID == "" && aux.PlanIDAlias != "" {
		p.PlanID = aux.PlanIDAlias
		warnPlanIDMCPDeprecation()
	}

	return nil
}
