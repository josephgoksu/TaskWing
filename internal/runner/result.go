package runner

import (
	"encoding/json"
	"fmt"
	"strings"
)

// InvokeResult holds the output from an AI CLI invocation.
type InvokeResult struct {
	// RawOutput is the full stdout from the CLI subprocess.
	RawOutput string

	// ExitCode from the subprocess.
	ExitCode int

	// Stderr output (for debugging).
	Stderr string

	// Runner that produced this result.
	CLIType CLIType
}

// Decode unmarshals the JSON content from the result into the target struct.
// It uses ExtractJSON to handle various output formats.
// For Claude Code, it first unwraps the JSON envelope {"type":"result","result":"..."}.
func (r *InvokeResult) Decode(target any) error {
	raw := r.RawOutput

	// Claude Code wraps output in a JSON envelope: {"type":"result","result":"<content>"}
	// Unwrap before extracting the inner JSON.
	if r.CLIType == CLIClaude {
		raw = unwrapClaudeEnvelope(raw)
	}

	jsonStr, err := ExtractJSON(raw)
	if err != nil {
		return fmt.Errorf("extract JSON from %s output: %w", r.CLIType, err)
	}
	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return fmt.Errorf("unmarshal %s output: %w", r.CLIType, err)
	}
	return nil
}

// unwrapClaudeEnvelope extracts the inner content from Claude Code's JSON envelope.
// Claude Code outputs: {"type":"result","subtype":"success","cost_usd":...,"result":"<content>"}
// Returns the inner result string if envelope is detected, otherwise returns raw unchanged.
func unwrapClaudeEnvelope(raw string) string {
	trimmed := strings.TrimSpace(raw)
	var envelope struct {
		Type   string `json:"type"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
		return raw
	}
	if envelope.Type == "result" && envelope.Result != "" {
		return envelope.Result
	}
	return raw
}

// ExtractJSON attempts to extract a JSON object or array from mixed text output.
// It tries three strategies in order:
//  1. Full parse — entire string is valid JSON
//  2. Markdown code block — extract from ```json ... ``` fences
//  3. Brace matching — find the outermost { } or [ ] pair
func ExtractJSON(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)

	// Strategy 1: Full parse
	if json.Valid([]byte(trimmed)) {
		return trimmed, nil
	}

	// Strategy 2: Markdown code block
	if idx := strings.Index(trimmed, "```json"); idx >= 0 {
		start := idx + len("```json")
		end := strings.Index(trimmed[start:], "```")
		if end > 0 {
			candidate := strings.TrimSpace(trimmed[start : start+end])
			if json.Valid([]byte(candidate)) {
				return candidate, nil
			}
		}
	}

	// Also try generic code blocks
	if idx := strings.Index(trimmed, "```\n"); idx >= 0 {
		start := idx + len("```\n")
		end := strings.Index(trimmed[start:], "```")
		if end > 0 {
			candidate := strings.TrimSpace(trimmed[start : start+end])
			if json.Valid([]byte(candidate)) {
				return candidate, nil
			}
		}
	}

	// Strategy 3: Brace matching — find outermost { } or [ ]
	for _, pair := range [][2]byte{{'{', '}'}, {'[', ']'}} {
		if result, ok := extractBraceMatched(trimmed, pair[0], pair[1]); ok {
			return result, nil
		}
	}

	return "", fmt.Errorf("no valid JSON found in output (%d bytes)", len(raw))
}

// extractBraceMatched finds the outermost matched braces in a string.
func extractBraceMatched(s string, open, close byte) (string, bool) {
	start := strings.IndexByte(s, open)
	if start < 0 {
		return "", false
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		ch := s[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if ch == open {
			depth++
		} else if ch == close {
			depth--
			if depth == 0 {
				candidate := s[start : i+1]
				if json.Valid([]byte(candidate)) {
					return candidate, true
				}
				return "", false
			}
		}
	}
	return "", false
}

// BootstrapAnalysis is the expected JSON output from a bootstrap analysis invocation.
type BootstrapAnalysis struct {
	Findings      []BootstrapFinding      `json:"findings"`
	Relationships []BootstrapRelationship `json:"relationships,omitempty"`
}

// BootstrapFinding represents a single architectural finding from AI CLI analysis.
// Maps 1:1 to core.Finding fields to preserve full fidelity through the pipeline.
type BootstrapFinding struct {
	// Core fields
	Type            string  `json:"type"`             // "decision", "pattern", "constraint", "feature", "metadata", "documentation"
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Why             string  `json:"why,omitempty"`
	Tradeoffs       string  `json:"tradeoffs,omitempty"`
	ConfidenceScore float64 `json:"confidence_score"` // 0.0-1.0

	// Evidence with full fields
	Evidence []BootstrapEvidence `json:"evidence,omitempty"`

	// Metadata — agent-specific key-value data
	// Common keys: "component", "severity", "type", "trigger", "steps", "service"
	Metadata map[string]any `json:"metadata,omitempty"`

	// Debt classification — distinguishes essential from accidental complexity
	DebtScore    float64 `json:"debt_score,omitempty"`    // 0.0=clean, 1.0=pure technical debt
	DebtReason   string  `json:"debt_reason,omitempty"`   // Why this is considered debt
	RefactorHint string  `json:"refactor_hint,omitempty"` // How to eliminate the debt
}

// BootstrapEvidence represents verifiable proof for a finding.
type BootstrapEvidence struct {
	FilePath     string `json:"file_path"`
	StartLine    int    `json:"start_line,omitempty"`
	EndLine      int    `json:"end_line,omitempty"`
	Snippet      string `json:"snippet,omitempty"`
	GrepPattern  string `json:"grep_pattern,omitempty"`  // Pattern to find this evidence
	EvidenceType string `json:"evidence_type,omitempty"` // "file" (default) or "git"
}

// BootstrapRelationship represents a link between two findings.
type BootstrapRelationship struct {
	From     string `json:"from"`     // Title of source finding
	To       string `json:"to"`       // Title of target finding
	Relation string `json:"relation"` // depends_on, affects, extends, relates_to
	Reason   string `json:"reason"`   // Why they are related
}

// PlanOutput is the expected JSON output from a plan generation invocation.
type PlanOutput struct {
	GoalSummary         string           `json:"goal_summary"`
	Rationale           string           `json:"rationale"`
	Tasks               []PlanTaskOutput `json:"tasks"`
	EstimatedComplexity string           `json:"estimated_complexity"`
	Prerequisites       []string         `json:"prerequisites,omitempty"`
	RiskFactors         []string         `json:"risk_factors,omitempty"`
}

// PlanTaskOutput represents a single task in the plan output.
type PlanTaskOutput struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Priority           int      `json:"priority"`
	Complexity         string   `json:"complexity"`
	AssignedAgent      string   `json:"assigned_agent"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	ValidationSteps    []string `json:"validation_steps,omitempty"`
	DependsOn          []int    `json:"depends_on,omitempty"`
	ExpectedFiles      []string `json:"expected_files,omitempty"`
	Scope              string   `json:"scope,omitempty"`    // e.g., "auth", "api", "vectorsearch"
	Keywords           []string `json:"keywords,omitempty"` // Extracted from title/description
}

// ExecuteOutput is the expected JSON output from a task execution invocation.
type ExecuteOutput struct {
	Status        string   `json:"status"`          // "completed", "failed", "partial"
	Summary       string   `json:"summary"`         // What was done
	FilesModified []string `json:"files_modified"`   // Files that were changed
	Error         string   `json:"error,omitempty"` // Error message if failed
}
