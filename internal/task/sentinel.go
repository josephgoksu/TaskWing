package task

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// DeviationType classifies the type of deviation between expected and actual files.
type DeviationType string

const (
	// DeviationDrift means files were modified that weren't in the plan
	DeviationDrift DeviationType = "drift"
	// DeviationMissing means expected files weren't touched
	DeviationMissing DeviationType = "missing"
	// DeviationUnreported means git found modifications not reported by the agent
	DeviationUnreported DeviationType = "unreported"
	// DeviationOverReported means agent claimed modifications git doesn't show
	DeviationOverReported DeviationType = "over_reported"
)

// SeverityLevel indicates how serious a deviation is.
type SeverityLevel string

const (
	SeverityInfo    SeverityLevel = "info"    // Minor deviations, informational
	SeverityWarning SeverityLevel = "warning" // Notable deviations, review recommended
	SeverityError   SeverityLevel = "error"   // Significant deviations, intervention needed
)

// Deviation represents a single deviation between plan and execution.
type Deviation struct {
	Type     DeviationType `json:"type"`     // drift or missing
	File     string        `json:"file"`     // The file path
	Severity SeverityLevel `json:"severity"` // How serious this is
	Reason   string        `json:"reason"`   // Human-readable explanation
}

// VerificationStatus indicates the state of git verification.
type VerificationStatus string

const (
	// VerificationStatusVerified means git verification succeeded
	VerificationStatusVerified VerificationStatus = "verified"
	// VerificationStatusUnavailable means git verification could not run
	VerificationStatusUnavailable VerificationStatus = "unavailable"
	// VerificationStatusSkipped means git verification was not attempted
	VerificationStatusSkipped VerificationStatus = "skipped"
)

// SentinelReport contains the results of comparing expected vs actual files.
type SentinelReport struct {
	TaskID        string      `json:"task_id"`
	TaskTitle     string      `json:"task_title"`
	ExpectedFiles []string    `json:"expected_files"` // What plan said
	ActualFiles   []string    `json:"actual_files"`   // What happened
	Deviations    []Deviation `json:"deviations"`
	DeviationRate float64     `json:"deviation_rate"` // 0.0 = perfect match, 1.0 = complete mismatch
	Summary       string      `json:"summary"`        // Human-readable summary

	// Git verification fields
	GitVerification    *VerificationResult `json:"git_verification,omitempty"`    // Git-based verification results
	VerificationStatus VerificationStatus  `json:"verification_status,omitempty"` // verified, unavailable, or skipped
}

// Sentinel analyzes task execution for plan deviations.
type Sentinel struct {
	// Threshold for severity classification
	DriftWarningThreshold int // Number of drift files before warning (default: 2)
	DriftErrorThreshold   int // Number of drift files before error (default: 5)
}

// NewSentinel creates a new Sentinel analyzer with default thresholds.
func NewSentinel() *Sentinel {
	return &Sentinel{
		DriftWarningThreshold: 2,
		DriftErrorThreshold:   5,
	}
}

// Analyze compares expected files from the plan with actually modified files.
func (s *Sentinel) Analyze(t *Task) *SentinelReport {
	report := &SentinelReport{
		TaskID:        t.ID,
		TaskTitle:     t.Title,
		ExpectedFiles: t.ExpectedFiles,
		ActualFiles:   t.FilesModified,
		Deviations:    []Deviation{},
	}

	// Normalize paths for comparison
	expectedSet := make(map[string]bool)
	for _, f := range t.ExpectedFiles {
		expectedSet[normalizePath(f)] = true
	}

	actualSet := make(map[string]bool)
	for _, f := range t.FilesModified {
		actualSet[normalizePath(f)] = true
	}

	// Find drift: files modified but not expected
	driftCount := 0
	for f := range actualSet {
		if !expectedSet[f] {
			deviation := Deviation{
				Type:   DeviationDrift,
				File:   f,
				Reason: "File was modified but not in plan",
			}
			driftCount++
			deviation.Severity = s.classifyDriftSeverity(driftCount, f)
			report.Deviations = append(report.Deviations, deviation)
		}
	}

	// Find missing: files expected but not modified
	for f := range expectedSet {
		if !actualSet[f] {
			report.Deviations = append(report.Deviations, Deviation{
				Type:     DeviationMissing,
				File:     f,
				Severity: SeverityWarning,
				Reason:   "Expected file was not modified",
			})
		}
	}

	// Calculate deviation rate
	report.DeviationRate = s.calculateDeviationRate(expectedSet, actualSet)

	// Generate summary
	report.Summary = s.generateSummary(report)

	return report
}

// classifyDriftSeverity determines severity based on drift count and file type.
func (s *Sentinel) classifyDriftSeverity(driftCount int, file string) SeverityLevel {
	// High severity for certain file types
	if isHighRiskFile(file) {
		return SeverityError
	}

	// Based on thresholds
	if driftCount >= s.DriftErrorThreshold {
		return SeverityError
	}
	if driftCount >= s.DriftWarningThreshold {
		return SeverityWarning
	}
	return SeverityInfo
}

// isHighRiskFile checks if a file is high-risk (config, security, etc.).
func isHighRiskFile(file string) bool {
	lowerFile := strings.ToLower(file)
	highRiskPatterns := []string{
		"config", ".env", "secret", "credential",
		"auth", "security", "password", "token",
		"migration", "schema",
	}
	for _, pattern := range highRiskPatterns {
		if strings.Contains(lowerFile, pattern) {
			return true
		}
	}
	return false
}

// calculateDeviationRate computes how much the execution deviated from plan.
// 0.0 = perfect match, 1.0 = complete mismatch
func (s *Sentinel) calculateDeviationRate(expected, actual map[string]bool) float64 {
	if len(expected) == 0 && len(actual) == 0 {
		return 0.0 // No files expected, none modified = perfect
	}
	if len(expected) == 0 {
		return 1.0 // No files expected but some modified = complete drift
	}

	// Count matches
	matches := 0
	for f := range expected {
		if actual[f] {
			matches++
		}
	}

	// Count total unique files
	union := make(map[string]bool)
	for f := range expected {
		union[f] = true
	}
	for f := range actual {
		union[f] = true
	}

	if len(union) == 0 {
		return 0.0
	}

	// Deviation = 1 - (matches / union)
	return 1.0 - (float64(matches) / float64(len(union)))
}

// generateSummary creates a human-readable summary of the report.
func (s *Sentinel) generateSummary(report *SentinelReport) string {
	if len(report.Deviations) == 0 {
		return "Execution matched plan exactly"
	}

	driftCount := 0
	missingCount := 0
	errorCount := 0

	for _, d := range report.Deviations {
		switch d.Type {
		case DeviationDrift:
			driftCount++
		case DeviationMissing:
			missingCount++
		}
		if d.Severity == SeverityError {
			errorCount++
		}
	}

	var parts []string
	if driftCount > 0 {
		parts = append(parts, pluralize(driftCount, "unexpected file", "unexpected files"))
	}
	if missingCount > 0 {
		parts = append(parts, pluralize(missingCount, "missing file", "missing files"))
	}

	summary := strings.Join(parts, ", ")
	if errorCount > 0 {
		summary += " (requires review)"
	}

	return summary
}

// pluralize returns singular or plural form based on count.
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", count, plural)
}

// normalizePath normalizes a file path for comparison.
func normalizePath(p string) string {
	// Clean the path
	p = filepath.Clean(p)
	// Remove leading ./
	p = strings.TrimPrefix(p, "./")
	// Normalize to forward slashes
	p = filepath.ToSlash(p)
	return p
}

// HasCriticalDeviations returns true if the report contains any error-level deviations.
func (r *SentinelReport) HasCriticalDeviations() bool {
	for _, d := range r.Deviations {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasDeviations returns true if there are any deviations.
func (r *SentinelReport) HasDeviations() bool {
	return len(r.Deviations) > 0
}

// GetDeviationsByType returns deviations filtered by type.
func (r *SentinelReport) GetDeviationsByType(dtype DeviationType) []Deviation {
	var result []Deviation
	for _, d := range r.Deviations {
		if d.Type == dtype {
			result = append(result, d)
		}
	}
	return result
}

// AnalyzeWithVerification runs Sentinel analysis with git-based verification.
// This catches cases where an agent lies about what files it modified.
// If repoRoot is empty or not a git repo, falls back to standard analysis.
func (s *Sentinel) AnalyzeWithVerification(ctx context.Context, t *Task, repoRoot string) *SentinelReport {
	// Run standard analysis first
	report := s.Analyze(t)

	// Skip git verification if no repo root provided
	if repoRoot == "" {
		report.VerificationStatus = VerificationStatusSkipped
		return report
	}

	// Check if it's a git repo
	if !IsGitRepo(repoRoot) {
		report.VerificationStatus = VerificationStatusUnavailable
		return report
	}

	// Attempt git verification with baseline exclusion
	// GitBaseline contains files already modified before task started
	verifier := NewGitVerifier(repoRoot)
	vResult := verifier.VerifyWithBaseline(ctx, t.FilesModified, t.GitBaseline)
	report.GitVerification = vResult

	if !vResult.IsVerified {
		report.VerificationStatus = VerificationStatusUnavailable
		// Log warning but continue with unverified result
		return report
	}

	report.VerificationStatus = VerificationStatusVerified

	// Add unreported files as deviations - these are files git found that the agent didn't report
	// This is the critical bypass detection: agent modified files but didn't tell us
	for _, f := range vResult.UnreportedFiles {
		severity := SeverityError // Always error - this is potential bypass attempt
		reason := "File was modified (per git) but not reported by agent"

		if isHighRiskFile(f) {
			reason = "HIGH RISK: " + reason
		}

		report.Deviations = append(report.Deviations, Deviation{
			Type:     DeviationUnreported,
			File:     f,
			Severity: severity,
			Reason:   reason,
		})
	}

	// Add over-reported files as warnings (agent claimed to modify but didn't)
	// This is less critical - could be hallucination or stale info
	for _, f := range vResult.OverReported {
		report.Deviations = append(report.Deviations, Deviation{
			Type:     DeviationOverReported,
			File:     f,
			Severity: SeverityWarning,
			Reason:   "File was reported as modified but git shows no changes",
		})
	}

	// Regenerate summary to include verification results
	report.Summary = s.generateSummaryWithVerification(report)

	return report
}

// generateSummaryWithVerification creates a summary that includes git verification info.
func (s *Sentinel) generateSummaryWithVerification(report *SentinelReport) string {
	if len(report.Deviations) == 0 {
		if report.VerificationStatus == VerificationStatusVerified {
			return "Execution matched plan exactly (git verified)"
		}
		return "Execution matched plan exactly"
	}

	driftCount := 0
	missingCount := 0
	unreportedCount := 0
	overReportedCount := 0
	errorCount := 0

	for _, d := range report.Deviations {
		switch d.Type {
		case DeviationDrift:
			driftCount++
		case DeviationMissing:
			missingCount++
		case DeviationUnreported:
			unreportedCount++
		case DeviationOverReported:
			overReportedCount++
		}
		if d.Severity == SeverityError {
			errorCount++
		}
	}

	var parts []string
	if driftCount > 0 {
		parts = append(parts, pluralize(driftCount, "unexpected file", "unexpected files"))
	}
	if missingCount > 0 {
		parts = append(parts, pluralize(missingCount, "missing file", "missing files"))
	}
	if unreportedCount > 0 {
		parts = append(parts, pluralize(unreportedCount, "unreported modification", "unreported modifications"))
	}
	if overReportedCount > 0 {
		parts = append(parts, pluralize(overReportedCount, "over-reported file", "over-reported files"))
	}

	summary := strings.Join(parts, ", ")

	if report.VerificationStatus == VerificationStatusVerified {
		summary += " [git verified]"
	}

	if errorCount > 0 {
		summary += " (requires review)"
	}

	return summary
}

// PolicyEnforcementResult contains the result of policy enforcement.
type PolicyEnforcementResult struct {
	Allowed    bool     `json:"allowed"`
	Violations []string `json:"violations,omitempty"`
	DecisionID string   `json:"decision_id,omitempty"`
	Error      error    `json:"error,omitempty"`
}

// PolicyTaskInput is the data structure passed to policy evaluators.
type PolicyTaskInput struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	FilesModified []string `json:"files_modified"`
	FilesCreated  []string `json:"files_created"`
}

// PolicyPlanInput is optional plan context for policy evaluation.
type PolicyPlanInput struct {
	ID   string `json:"id"`
	Goal string `json:"goal"`
}

// PolicyEvaluator is the interface for evaluating tasks against policies.
// This is implemented by policy.PolicyEvaluatorAdapter but defined here to avoid import cycles.
// Uses primitive types to prevent import cycles between task and policy packages.
type PolicyEvaluator interface {
	// EvaluateTaskPolicy evaluates a task against loaded policies.
	// Returns whether allowed, any violations, and the decision ID.
	EvaluateTaskPolicy(ctx context.Context, taskID, taskTitle, taskDescription string, filesModified, filesCreated []string, planID, planGoal string) (allowed bool, violations []string, decisionID string, err error)
	// EvaluateFilesPolicy checks if file modifications are allowed.
	EvaluateFilesPolicy(ctx context.Context, filesModified, filesCreated []string) (allowed bool, violations []string, decisionID string, err error)
	// PolicyCount returns the number of loaded policies.
	PolicyCount() int
}

// PolicyEnforcer enforces OPA policies on task completion.
// It evaluates tasks against loaded policies and records decisions.
type PolicyEnforcer struct {
	evaluator PolicyEvaluator
	sessionID string
}

// NewPolicyEnforcer creates a new PolicyEnforcer with the given evaluator.
// If evaluator is nil, all tasks will be allowed by default.
func NewPolicyEnforcer(evaluator PolicyEvaluator, sessionID string) *PolicyEnforcer {
	return &PolicyEnforcer{
		evaluator: evaluator,
		sessionID: sessionID,
	}
}

// Enforce evaluates the task against loaded policies.
// Returns the enforcement result indicating whether the task is allowed to proceed.
// If the result indicates denial, the task should be transitioned to 'failed' status.
func (pe *PolicyEnforcer) Enforce(ctx context.Context, t *Task, planGoal string) *PolicyEnforcementResult {
	if pe.evaluator == nil {
		// No policy evaluator configured - allow by default
		return &PolicyEnforcementResult{
			Allowed: true,
		}
	}

	// Evaluate against policies using primitives (avoids import cycle)
	allowed, violations, decisionID, err := pe.evaluator.EvaluateTaskPolicy(
		ctx,
		t.ID,
		t.Title,
		t.Description,
		t.FilesModified,
		[]string{}, // filesCreated - we don't track this separately
		t.PlanID,
		planGoal,
	)
	if err != nil {
		return &PolicyEnforcementResult{
			Allowed: false,
			Error:   fmt.Errorf("policy evaluation failed: %w", err),
		}
	}

	return &PolicyEnforcementResult{
		Allowed:    allowed,
		Violations: violations,
		DecisionID: decisionID,
	}
}

// EnforceFiles is a convenience method for checking if file modifications are allowed.
// This can be called during task execution to pre-validate file changes.
func (pe *PolicyEnforcer) EnforceFiles(ctx context.Context, filesModified, filesCreated []string) *PolicyEnforcementResult {
	if pe.evaluator == nil {
		return &PolicyEnforcementResult{
			Allowed: true,
		}
	}

	allowed, violations, decisionID, err := pe.evaluator.EvaluateFilesPolicy(ctx, filesModified, filesCreated)
	if err != nil {
		return &PolicyEnforcementResult{
			Allowed: false,
			Error:   fmt.Errorf("policy evaluation failed: %w", err),
		}
	}

	return &PolicyEnforcementResult{
		Allowed:    allowed,
		Violations: violations,
		DecisionID: decisionID,
	}
}

// HasPolicies returns true if the enforcer has policies loaded.
func (pe *PolicyEnforcer) HasPolicies() bool {
	return pe.evaluator != nil && pe.evaluator.PolicyCount() > 0
}

// PolicyCount returns the number of loaded policies.
func (pe *PolicyEnforcer) PolicyCount() int {
	if pe.evaluator == nil {
		return 0
	}
	return pe.evaluator.PolicyCount()
}
