// Package policy provides policy-as-code enforcement using OPA (Open Policy Agent).
// It allows enterprises to define guardrails in Rego that AI agents must obey.
package policy

import (
	"encoding/json"
	"time"
)

// PolicyDecision represents the outcome of evaluating a policy against some input.
// This is stored in the policy_decisions table for audit trail and compliance.
type PolicyDecision struct {
	ID          int64     `json:"id"`                   // Auto-increment primary key
	DecisionID  string    `json:"decisionId"`           // UUID for referencing
	PolicyPath  string    `json:"policyPath"`           // Rego package path (e.g., "taskwing.policy")
	Result      string    `json:"result"`               // "allow" or "deny"
	Violations  []string  `json:"violations,omitempty"` // Deny messages from OPA
	Input       any       `json:"input"`                // The input that was evaluated
	TaskID      string    `json:"taskId,omitempty"`     // Optional task context
	SessionID   string    `json:"sessionId,omitempty"`  // Optional session context
	EvaluatedAt time.Time `json:"evaluatedAt"`          // When the evaluation occurred
}

// PolicyResult constants.
const (
	PolicyResultAllow = "allow"
	PolicyResultDeny  = "deny"
)

// IsAllowed returns true if the policy decision was "allow".
func (d *PolicyDecision) IsAllowed() bool {
	return d.Result == PolicyResultAllow
}

// IsDenied returns true if the policy decision was "deny".
func (d *PolicyDecision) IsDenied() bool {
	return d.Result == PolicyResultDeny
}

// ViolationsJSON returns the violations as a JSON string for storage.
func (d *PolicyDecision) ViolationsJSON() string {
	if len(d.Violations) == 0 {
		return "[]"
	}
	b, err := json.Marshal(d.Violations)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// InputJSON returns the input as a JSON string for storage.
func (d *PolicyDecision) InputJSON() string {
	if d.Input == nil {
		return "{}"
	}
	b, err := json.Marshal(d.Input)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ParseViolations parses a JSON string into a slice of violations.
func ParseViolations(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	var v []string
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}

// PolicyInput represents the structured input provided to OPA policies.
// This is what Rego policies receive in the `input` variable.
type PolicyInput struct {
	Task    *TaskInput    `json:"task,omitempty"`
	Plan    *PlanInput    `json:"plan,omitempty"`
	Context *ContextInput `json:"context,omitempty"`
}

// TaskInput contains task-specific data for policy evaluation.
type TaskInput struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	FilesModified []string `json:"files_modified,omitempty"`
	FilesCreated  []string `json:"files_created,omitempty"`
}

// PlanInput contains plan-specific data for policy evaluation.
type PlanInput struct {
	ID   string `json:"id"`
	Goal string `json:"goal"`
}

// ContextInput contains project context for policy evaluation.
type ContextInput struct {
	ProtectedZones []string `json:"protected_zones,omitempty"`
	ProjectType    string   `json:"project_type,omitempty"`
}

// EvaluationResult represents the outcome of policy evaluation.
type EvaluationResult struct {
	Allowed    bool     `json:"allowed"`
	Denied     bool     `json:"denied"`
	Violations []string `json:"violations,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// IsBlocked returns true if any deny rules fired.
func (r *EvaluationResult) IsBlocked() bool {
	return r.Denied || len(r.Violations) > 0
}
