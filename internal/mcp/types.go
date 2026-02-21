// Package mcp provides types and utilities for the MCP server.
package mcp

import (
	"encoding/json"
	"fmt"
)

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
//   - next: session_id (optional when MCP transport provides session identity)
//   - current: session_id (optional when MCP transport provides session identity)
//   - start: task_id, session_id (optional when MCP transport provides session identity)
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
	PlanID string `json:"plan_id,omitempty"`

	// SessionID is the unique AI session identifier.
	// Optional for: next, current, start when MCP transport session identity is available.
	// REQUIRED otherwise.
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

type taskToolParamsAlias TaskToolParams

// UnmarshalJSON enforces strict snake_case payloads for task tool params.
func (p *TaskToolParams) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if _, hasLegacyPlanID := raw["planId"]; hasLegacyPlanID {
		return fmt.Errorf("planId is no longer supported; use plan_id")
	}

	var aux taskToolParamsAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*p = TaskToolParams(aux)
	return nil
}

// === MCP Tool Parameters (non-unified) ===

// ProjectContextParams defines the parameters for the ask tool.
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

// ClarifyAnswerInput is a structured answer to a clarification question.
type ClarifyAnswerInput struct {
	Question string `json:"question,omitempty"`
	Answer   string `json:"answer"`
}

// PlanToolParams defines the parameters for the unified plan tool.
// Supports planning actions: clarify, decompose, expand, generate, finalize, audit
//
// Required fields by action:
//   - clarify first call: goal
//   - clarify follow-up: clarify_session_id (+ answers unless auto_answer=true)
//   - decompose: plan_id (with enriched_goal) OR enriched_goal (creates new plan)
//   - expand: plan_id, phase_id OR phase_index
//   - generate: goal, enriched_goal, clarify_session_id
//   - finalize: plan_id
//   - audit: none (defaults to active plan)
type PlanToolParams struct {
	// Action specifies which operation to perform.
	// Required. One of: clarify, decompose, expand, generate, finalize, audit
	Action PlanAction `json:"action"`

	// Goal is the user's development goal.
	// REQUIRED for: clarify first call, generate
	// Optional for: clarify follow-up calls (loaded from clarify_session_id)
	Goal string `json:"goal,omitempty"`

	// EnrichedGoal is the full technical specification from clarify.
	// REQUIRED for: generate, decompose (will error if empty; call clarify first to get this)
	EnrichedGoal string `json:"enriched_goal,omitempty"`

	// ClarifySessionID identifies an existing clarify session for follow-up rounds.
	// REQUIRED for: clarify follow-up calls, generate
	ClarifySessionID string `json:"clarify_session_id,omitempty"`

	// Answers are user responses for the previous clarification round.
	// REQUIRED for: clarify follow-up calls (unless auto_answer=true)
	Answers []ClarifyAnswerInput `json:"answers,omitempty"`

	// AutoAnswer uses knowledge graph to auto-answer clarifying questions.
	// Optional for: clarify (default: false)
	AutoAnswer bool `json:"auto_answer,omitempty"`

	// Save persists the generated plan to the database.
	// Optional for: generate (default: true)
	Save *bool `json:"save,omitempty"`

	// PlanID is the plan to operate on.
	// REQUIRED for: expand, finalize
	// Optional for: decompose (creates new plan if not provided), audit (defaults to active plan)
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

type planToolParamsAlias PlanToolParams

// UnmarshalJSON enforces strict snake_case payloads and rejects removed fields.
func (p *PlanToolParams) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if _, hasLegacyPlanID := raw["planId"]; hasLegacyPlanID {
		return fmt.Errorf("planId is no longer supported; use plan_id")
	}
	if _, hasHistory := raw["history"]; hasHistory {
		return fmt.Errorf("history is no longer supported; use clarify_session_id and answers")
	}

	var aux planToolParamsAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*p = PlanToolParams(aux)
	return nil
}
