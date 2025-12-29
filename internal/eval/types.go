/*
Package eval provides evaluation harness types, templates, and business logic.
This package contains all non-CLI logic for the TaskWing eval command.
*/
package eval

import "time"

// Config defines evaluation tasks and hard-fail rules.
type Config struct {
	Version       int    `yaml:"version"`
	Project       string `yaml:"project"`
	HardFailRules []Rule `yaml:"hard_fail_rules"`
	Tasks         []Task `yaml:"tasks"`
}

// Rule defines a hard-fail evaluation rule.
type Rule struct {
	ID         string   `yaml:"id"`
	TaskIDs    []string `yaml:"task_ids"`
	RequireAll []string `yaml:"require_all"`
	RequireAny []string `yaml:"require_any"`
	Forbid     []string `yaml:"forbid"`
	AllowIf    []string `yaml:"allow_if"`
}

// Task defines a single evaluation task.
type Task struct {
	ID             string `yaml:"id"`
	Title          string `yaml:"title"`
	Prompt         string `yaml:"prompt"`
	Expected       string `yaml:"expected"`        // For LLM judge: expected behavior
	FailureSignals string `yaml:"failure_signals"` // For LLM judge: signals of failure
	PassFail       struct {
		Pass string `yaml:"pass"` // Legacy: human-readable pass criteria
		Fail string `yaml:"fail"` // Legacy: human-readable fail criteria
	} `yaml:"pass_fail"`
}

// Results holds the output of an evaluation run.
type Results struct {
	GeneratedAt time.Time              `json:"generatedAt"`
	Label       string                 `json:"label,omitempty"`
	ContextMode string                 `json:"context_mode,omitempty"`
	Runner      string                 `json:"runner,omitempty"`
	TaskResults []TaskResult           `json:"results"`
	Summary     map[string]Summary     `json:"summary"`
	Costs       map[string]CostSummary `json:"costs,omitempty"`
}

// TaskResult holds the result of evaluating a single task.
type TaskResult struct {
	Task        string               `json:"task"`
	Model       string               `json:"model"`
	HardFail    bool                 `json:"hard_fail"`
	Checks      map[string]RuleCheck `json:"checks,omitempty"`
	JudgeReason string               `json:"judge_reason,omitempty"`
	OutputFile  string               `json:"output_file"`
}

// RuleCheck holds the result of evaluating a single rule.
type RuleCheck struct {
	RequireAll map[string]bool `json:"require_all"`
	RequireAny map[string]bool `json:"require_any"`
	Forbid     map[string]bool `json:"forbid"`
	AllowIf    map[string]bool `json:"allow_if"`
	Pass       bool            `json:"pass"`
}

// Summary holds aggregate results for a model.
type Summary struct {
	Total    int `json:"total"`
	HardFail int `json:"hard_fail"`
}

// CostSummary holds token usage and cost information.
type CostSummary struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// TokenUsage tracks token consumption for cost calculation.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// JudgeResult holds the result of LLM-based evaluation.
type JudgeResult struct {
	Pass   bool   `json:"pass"`
	Reason string `json:"reason"`
}
