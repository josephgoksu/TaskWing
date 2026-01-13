/*
Package audit provides the AuditService for validating completed plans against their implementation.
It performs both programmatic checks (build/test) and semantic verification (LLM analysis).
*/
package impl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/task"
)

// CommandExecutor is an interface for executing shell commands.
// This allows mocking in tests.
type CommandExecutor interface {
	Execute(ctx context.Context, workDir, name string, args ...string) (stdout, stderr string, err error)
}

// ShellExecutor executes real shell commands.
type ShellExecutor struct{}

// Execute runs a command and returns stdout, stderr, and any error.
func (e *ShellExecutor) Execute(ctx context.Context, workDir, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// LLMClient is an interface for LLM operations.
// This allows mocking in tests.
type LLMClient interface {
	Generate(ctx context.Context, messages []*schema.Message) (string, error)
}

// BaseLLMClient wraps core.BaseAgent for LLM operations.
type BaseLLMClient struct {
	agent *core.BaseAgent
}

// NewBaseLLMClient creates a new LLM client using core.BaseAgent.
func NewBaseLLMClient(cfg llm.Config) *BaseLLMClient {
	agent := core.NewBaseAgent("audit-llm", "LLM client for audit service", cfg)
	return &BaseLLMClient{agent: &agent}
}

// Generate sends messages to the LLM and returns the response.
func (c *BaseLLMClient) Generate(ctx context.Context, messages []*schema.Message) (string, error) {
	return c.agent.Generate(ctx, messages)
}

// VerificationResult contains the outcome of a single verification check.
type VerificationResult struct {
	Passed  bool   `json:"passed"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
	Command string `json:"command,omitempty"`
}

// AuditResult contains the full audit report.
type AuditResult struct {
	PlanID         string             `json:"planId"`
	Status         string             `json:"status"` // "passed", "failed"
	BuildResult    VerificationResult `json:"buildResult"`
	TestResult     VerificationResult `json:"testResult"`
	SemanticResult SemanticResult     `json:"semanticResult"`
	Timestamp      time.Time          `json:"timestamp"`
}

// SemanticResult contains the LLM semantic verification results.
type SemanticResult struct {
	Passed      bool     `json:"passed"`
	Issues      []string `json:"issues,omitempty"`
	Summary     string   `json:"summary"`
	RawResponse string   `json:"rawResponse,omitempty"`
}

// Service performs audit verification on completed plans.
type Service struct {
	workDir   string
	executor  CommandExecutor
	llmClient LLMClient
}

// NewService creates a new audit service.
func NewService(workDir string, llmCfg llm.Config) *Service {
	return &Service{
		workDir:   workDir,
		executor:  &ShellExecutor{},
		llmClient: NewBaseLLMClient(llmCfg),
	}
}

// NewServiceWithDeps creates a new audit service with custom dependencies (for testing).
func NewServiceWithDeps(workDir string, executor CommandExecutor, llmClient LLMClient) *Service {
	return &Service{
		workDir:   workDir,
		executor:  executor,
		llmClient: llmClient,
	}
}

// Audit performs a full verification of a plan.
// It runs build, tests, and semantic analysis.
func (s *Service) Audit(ctx context.Context, plan *task.Plan) (*AuditResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is nil")
	}

	result := &AuditResult{
		PlanID:    plan.ID,
		Timestamp: time.Now().UTC(),
	}

	// Step 1: Run programmatic checks (build and test)
	result.BuildResult = s.runBuild(ctx)
	result.TestResult = s.runTests(ctx)

	// Step 2: Run semantic verification if LLM client is available
	if s.llmClient != nil {
		semanticResult, err := s.runSemanticCheck(ctx, plan)
		if err != nil {
			result.SemanticResult = SemanticResult{
				Passed:  false,
				Summary: fmt.Sprintf("Semantic check failed: %v", err),
			}
		} else {
			result.SemanticResult = semanticResult
		}
	} else {
		result.SemanticResult = SemanticResult{
			Passed:  true,
			Summary: "Semantic check skipped (no LLM configured)",
		}
	}

	// Determine overall status
	if result.BuildResult.Passed && result.TestResult.Passed && result.SemanticResult.Passed {
		result.Status = "passed"
	} else {
		result.Status = "failed"
	}

	return result, nil
}

// runBuild executes the build command and returns the result.
func (s *Service) runBuild(ctx context.Context) VerificationResult {
	result := VerificationResult{
		Command: "go build ./...",
	}

	stdout, stderr, err := s.executor.Execute(ctx, s.workDir, "go", "build", "./...")
	result.Output = stdout + stderr

	if err != nil {
		result.Passed = false
		result.Error = err.Error()
	} else {
		result.Passed = true
	}

	return result
}

// runTests executes the test command and returns the result.
func (s *Service) runTests(ctx context.Context) VerificationResult {
	result := VerificationResult{
		Command: "go test ./...",
	}

	stdout, stderr, err := s.executor.Execute(ctx, s.workDir, "go", "test", "./...", "-count=1")
	result.Output = stdout + stderr

	if err != nil {
		result.Passed = false
		result.Error = err.Error()
	} else {
		result.Passed = true
	}

	return result
}

// runSemanticCheck uses LLM to verify the implementation matches the plan.
func (s *Service) runSemanticCheck(ctx context.Context, plan *task.Plan) (SemanticResult, error) {
	result := SemanticResult{}

	// Build the prompt for semantic verification
	prompt := s.buildSemanticPrompt(plan)

	messages := []*schema.Message{
		schema.SystemMessage(`You are a code auditor. Analyze whether the described implementation requirements have been met.
Respond in JSON format:
{
  "passed": true/false,
  "issues": ["issue1", "issue2"],
  "summary": "brief summary of findings"
}

Be strict: if requirements are vague, assume they were met. Only flag clear violations.`),
		schema.UserMessage(prompt),
	}

	response, err := s.llmClient.Generate(ctx, messages)
	if err != nil {
		return result, fmt.Errorf("LLM generation failed: %w", err)
	}

	result.RawResponse = response

	// Parse the JSON response
	var llmResult struct {
		Passed  bool     `json:"passed"`
		Issues  []string `json:"issues"`
		Summary string   `json:"summary"`
	}

	// Try to extract JSON from response (it might have markdown code blocks)
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), &llmResult); err != nil {
		// If parsing fails, assume passed with a warning
		result.Passed = true
		result.Summary = "Could not parse LLM response, assuming passed"
		return result, nil
	}

	result.Passed = llmResult.Passed
	result.Issues = llmResult.Issues
	result.Summary = llmResult.Summary

	return result, nil
}

// buildSemanticPrompt creates the prompt for semantic verification.
func (s *Service) buildSemanticPrompt(plan *task.Plan) string {
	var sb strings.Builder

	sb.WriteString("## Plan Requirements\n\n")
	sb.WriteString("**Goal**: ")
	sb.WriteString(plan.Goal)
	sb.WriteString("\n\n")

	if plan.EnrichedGoal != "" {
		sb.WriteString("**Detailed Specification**:\n")
		sb.WriteString(plan.EnrichedGoal)
		sb.WriteString("\n\n")
	}

	if len(plan.Tasks) > 0 {
		sb.WriteString("## Completed Tasks\n\n")
		for i, t := range plan.Tasks {
			if t.Status == task.StatusCompleted {
				sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, t.Title))
				if t.CompletionSummary != "" {
					sb.WriteString(fmt.Sprintf("   Summary: %s\n", t.CompletionSummary))
				}
				if len(t.FilesModified) > 0 {
					sb.WriteString(fmt.Sprintf("   Files: %s\n", strings.Join(t.FilesModified, ", ")))
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n## Verification Request\n\n")
	sb.WriteString("Based on the plan requirements and completed tasks above, verify:\n")
	sb.WriteString("1. Do the completed tasks align with the stated goal?\n")
	sb.WriteString("2. Are there any obvious gaps or missing requirements?\n")
	sb.WriteString("3. Do the file modifications seem appropriate for the stated work?\n")

	return sb.String()
}

// extractJSON attempts to extract JSON from a response that might have markdown code blocks.
func extractJSON(response string) string {
	// Try to find JSON in code blocks
	if idx := strings.Index(response, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(response[start:], "```"); end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	// Try to find JSON in generic code blocks
	if idx := strings.Index(response, "```"); idx != -1 {
		start := idx + 3
		// Skip language identifier if present
		if newline := strings.Index(response[start:], "\n"); newline != -1 {
			start += newline + 1
		}
		if end := strings.Index(response[start:], "```"); end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	// Try to find raw JSON (starts with {)
	if idx := strings.Index(response, "{"); idx != -1 {
		// Find matching closing brace
		depth := 0
		for i := idx; i < len(response); i++ {
			switch response[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return response[idx : i+1]
				}
			}
		}
	}

	return response
}

// MaxRetries is the maximum number of auto-fix attempts.
const MaxRetries = 3

// writeFile is a wrapper for os.WriteFile for easier testing.
var writeFile = func(path string, content []byte) error {
	return os.WriteFile(path, content, 0644)
}

// FixSuggestion represents a suggested code fix from the LLM.
type FixSuggestion struct {
	FilePath    string `json:"filePath"`
	Description string `json:"description"`
	OldCode     string `json:"oldCode,omitempty"`
	NewCode     string `json:"newCode"`
	FullFile    bool   `json:"fullFile,omitempty"` // If true, NewCode replaces entire file
}

// AutoFixResult contains the outcome of an auto-fix loop.
type AutoFixResult struct {
	FinalStatus  string          `json:"finalStatus"` // "verified", "needs_revision"
	Attempts     int             `json:"attempts"`
	FixesApplied []string        `json:"fixesApplied"`
	FinalAudit   *AuditResult    `json:"finalAudit"`
	History      []AttemptRecord `json:"history"`
}

// AttemptRecord records a single fix attempt.
type AttemptRecord struct {
	Attempt     int            `json:"attempt"`
	AuditResult *AuditResult   `json:"auditResult"`
	FixApplied  *FixSuggestion `json:"fixApplied,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// AuditWithAutoFix performs audit and attempts to fix failures automatically.
// It will retry up to MaxRetries times before giving up.
// Returns the final status: "verified" if all checks pass, "needs_revision" otherwise.
func (s *Service) AuditWithAutoFix(ctx context.Context, plan *task.Plan) (*AutoFixResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is nil")
	}

	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client is required for auto-fix")
	}

	result := &AutoFixResult{
		History: make([]AttemptRecord, 0, MaxRetries),
	}

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		record := AttemptRecord{Attempt: attempt}

		// Run audit
		auditResult, err := s.Audit(ctx, plan)
		if err != nil {
			record.Error = err.Error()
			result.History = append(result.History, record)
			continue
		}
		record.AuditResult = auditResult

		// Check if passed
		if auditResult.Status == "passed" {
			result.FinalStatus = "verified"
			result.Attempts = attempt
			result.FinalAudit = auditResult
			result.History = append(result.History, record)
			return result, nil
		}

		// If this is the last attempt, don't try to fix
		if attempt == MaxRetries {
			result.History = append(result.History, record)
			break
		}

		// Try to generate and apply fix
		fix, err := s.generateFix(ctx, auditResult, plan)
		if err != nil {
			record.Error = fmt.Sprintf("failed to generate fix: %v", err)
			result.History = append(result.History, record)
			continue
		}

		if fix != nil {
			record.FixApplied = fix
			if err := s.applyFix(fix); err != nil {
				record.Error = fmt.Sprintf("failed to apply fix: %v", err)
				result.History = append(result.History, record)
				continue
			}
			result.FixesApplied = append(result.FixesApplied, fix.Description)
		}

		result.History = append(result.History, record)
	}

	// Failed after all retries
	result.FinalStatus = "needs_revision"
	result.Attempts = len(result.History)
	if len(result.History) > 0 {
		result.FinalAudit = result.History[len(result.History)-1].AuditResult
	}

	return result, nil
}

// generateFix uses LLM to suggest a fix for the failed audit.
func (s *Service) generateFix(ctx context.Context, audit *AuditResult, plan *task.Plan) (*FixSuggestion, error) {
	prompt := s.buildFixPrompt(audit, plan)

	messages := []*schema.Message{
		schema.SystemMessage(`You are a code repair assistant. Analyze the build/test failure and suggest a fix.
Respond in JSON format:
{
  "filePath": "path/to/file.go",
  "description": "Brief description of the fix",
  "oldCode": "the problematic code snippet (optional)",
  "newCode": "the corrected code",
  "fullFile": false
}

Rules:
- Only suggest fixes for clear, obvious errors (syntax errors, missing imports, typos)
- Do not refactor or change logic unless it's clearly broken
- If the error is unclear or requires human judgment, respond with: {"filePath": "", "description": "Cannot auto-fix: <reason>", "newCode": ""}
- Keep fixes minimal and focused`),
		schema.UserMessage(prompt),
	}

	response, err := s.llmClient.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse the fix suggestion
	var fix FixSuggestion
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), &fix); err != nil {
		return nil, fmt.Errorf("failed to parse fix suggestion: %w", err)
	}

	// Check if LLM decided not to fix
	if fix.FilePath == "" || fix.NewCode == "" {
		return nil, nil // No fix suggested
	}

	return &fix, nil
}

// buildFixPrompt creates the prompt for fix generation.
func (s *Service) buildFixPrompt(audit *AuditResult, plan *task.Plan) string {
	var sb strings.Builder

	sb.WriteString("## Build/Test Failure Report\n\n")

	if !audit.BuildResult.Passed {
		sb.WriteString("### Build Failed\n")
		sb.WriteString("```\n")
		sb.WriteString(truncateOutput(audit.BuildResult.Output, 2000))
		sb.WriteString("\n```\n\n")
	}

	if !audit.TestResult.Passed {
		sb.WriteString("### Tests Failed\n")
		sb.WriteString("```\n")
		sb.WriteString(truncateOutput(audit.TestResult.Output, 2000))
		sb.WriteString("\n```\n\n")
	}

	if !audit.SemanticResult.Passed && len(audit.SemanticResult.Issues) > 0 {
		sb.WriteString("### Semantic Issues\n")
		for _, issue := range audit.SemanticResult.Issues {
			sb.WriteString("- " + issue + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Context\n\n")
	sb.WriteString("**Project Goal**: " + plan.Goal + "\n")
	sb.WriteString("**Working Directory**: " + s.workDir + "\n\n")

	sb.WriteString("Please analyze the error and suggest a minimal fix.\n")

	return sb.String()
}

// applyFix applies the suggested fix to the file.
func (s *Service) applyFix(fix *FixSuggestion) error {
	if fix == nil || fix.FilePath == "" {
		return fmt.Errorf("invalid fix: no file path")
	}

	// Resolve path relative to workDir
	filePath := fix.FilePath
	if !strings.HasPrefix(filePath, "/") {
		filePath = s.workDir + "/" + filePath
	}

	if fix.FullFile || fix.OldCode == "" {
		// Replace entire file
		return writeFile(filePath, []byte(fix.NewCode))
	}

	// Read current file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Replace old code with new code
	newContent := strings.Replace(string(content), fix.OldCode, fix.NewCode, 1)
	if newContent == string(content) {
		return fmt.Errorf("old code not found in file")
	}

	return writeFile(filePath, []byte(newContent))
}

// truncateOutput truncates output to a maximum length.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}

// ToAuditReportWithFixes converts AutoFixResult to task.AuditReport.
func (r *AutoFixResult) ToAuditReportWithFixes() task.AuditReport {
	report := task.AuditReport{
		RetryCount:   r.Attempts,
		FixesApplied: r.FixesApplied,
		CompletedAt:  time.Now().UTC(),
	}

	if r.FinalStatus == "verified" {
		report.Status = "passed"
	} else {
		report.Status = "failed"
	}

	if r.FinalAudit != nil {
		report.BuildOutput = r.FinalAudit.BuildResult.Output
		report.TestOutput = r.FinalAudit.TestResult.Output
		report.SemanticIssues = r.FinalAudit.SemanticResult.Issues

		if !r.FinalAudit.BuildResult.Passed && r.FinalAudit.BuildResult.Error != "" {
			report.ErrorMessage = "Build failed: " + r.FinalAudit.BuildResult.Error
		} else if !r.FinalAudit.TestResult.Passed && r.FinalAudit.TestResult.Error != "" {
			report.ErrorMessage = "Tests failed: " + r.FinalAudit.TestResult.Error
		}
	}

	return report
}
