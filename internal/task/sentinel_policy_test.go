package task

import (
	"context"
	"testing"
)

// mockPolicyEvaluator implements PolicyEvaluator for testing.
type mockPolicyEvaluator struct {
	allowAll    bool
	violations  []string
	decisionID  string
	err         error
	policyCount int
}

func (m *mockPolicyEvaluator) EvaluateTaskPolicy(ctx context.Context, taskID, taskTitle, taskDescription string, filesModified, filesCreated []string, planID, planGoal string) (bool, []string, string, error) {
	if m.err != nil {
		return false, nil, "", m.err
	}
	if m.allowAll {
		return true, nil, m.decisionID, nil
	}
	// Check for protected files
	for _, f := range filesModified {
		if isProtectedFile(f) {
			return false, m.violations, m.decisionID, nil
		}
	}
	return true, nil, m.decisionID, nil
}

func (m *mockPolicyEvaluator) EvaluateFilesPolicy(ctx context.Context, filesModified, filesCreated []string) (bool, []string, string, error) {
	if m.err != nil {
		return false, nil, "", m.err
	}
	if m.allowAll {
		return true, nil, m.decisionID, nil
	}
	for _, f := range filesModified {
		if isProtectedFile(f) {
			return false, m.violations, m.decisionID, nil
		}
	}
	return true, nil, m.decisionID, nil
}

func (m *mockPolicyEvaluator) PolicyCount() int {
	return m.policyCount
}

// isProtectedFile checks if a file matches protected patterns.
func isProtectedFile(file string) bool {
	protectedPatterns := []string{".env", ".env.local", ".env.production", "secrets/"}
	for _, pattern := range protectedPatterns {
		if len(file) >= len(pattern) {
			// Simple contains check for testing
			for i := 0; i <= len(file)-len(pattern); i++ {
				if file[i:i+len(pattern)] == pattern {
					return true
				}
			}
		}
	}
	return false
}

func TestPolicyEnforcer_NoPolicies(t *testing.T) {
	enforcer := NewPolicyEnforcer(nil, "session-123")

	task := &Task{
		ID:            "task-123",
		Title:         "Test task",
		FilesModified: []string{".env"},
	}

	result := enforcer.Enforce(context.Background(), task, "Test plan goal")

	if !result.Allowed {
		t.Error("Expected task to be allowed when no policies are configured")
	}
	if enforcer.HasPolicies() {
		t.Error("HasPolicies should return false when evaluator is nil")
	}
}

func TestPolicyEnforcer_AllowNonProtectedFile(t *testing.T) {
	evaluator := &mockPolicyEvaluator{
		allowAll:    false,
		violations:  []string{"Cannot modify .env files"},
		decisionID:  "decision-456",
		policyCount: 1,
	}

	enforcer := NewPolicyEnforcer(evaluator, "session-123")

	task := &Task{
		ID:            "task-123",
		Title:         "Add feature",
		FilesModified: []string{"internal/app/main.go", "internal/app/handler.go"},
	}

	result := enforcer.Enforce(context.Background(), task, "Implement feature")

	if !result.Allowed {
		t.Error("Expected task to be allowed for non-protected files")
	}
}

func TestPolicyEnforcer_DenyEnvFile(t *testing.T) {
	evaluator := &mockPolicyEvaluator{
		allowAll:    false,
		violations:  []string{"Cannot modify .env files"},
		decisionID:  "decision-789",
		policyCount: 1,
	}

	enforcer := NewPolicyEnforcer(evaluator, "session-123")

	task := &Task{
		ID:            "task-123",
		Title:         "Update config",
		FilesModified: []string{".env"},
	}

	result := enforcer.Enforce(context.Background(), task, "Update configuration")

	if result.Allowed {
		t.Error("Expected task to be denied for .env file modification")
	}
	if len(result.Violations) == 0 {
		t.Error("Expected violations to be set")
	}
	if result.DecisionID != "decision-789" {
		t.Errorf("Expected decision ID 'decision-789', got '%s'", result.DecisionID)
	}
}

func TestPolicyEnforcer_DenyEnvLocalFile(t *testing.T) {
	evaluator := &mockPolicyEvaluator{
		allowAll:    false,
		violations:  []string{"Cannot modify environment files"},
		decisionID:  "decision-001",
		policyCount: 1,
	}

	enforcer := NewPolicyEnforcer(evaluator, "session-123")

	task := &Task{
		ID:            "task-123",
		Title:         "Update local config",
		FilesModified: []string{".env.local"},
	}

	result := enforcer.Enforce(context.Background(), task, "Update local configuration")

	if result.Allowed {
		t.Error("Expected task to be denied for .env.local file modification")
	}
}

func TestPolicyEnforcer_DenySecretsDirectory(t *testing.T) {
	evaluator := &mockPolicyEvaluator{
		allowAll:    false,
		violations:  []string{"Cannot modify secrets directory"},
		decisionID:  "decision-002",
		policyCount: 1,
	}

	enforcer := NewPolicyEnforcer(evaluator, "session-123")

	task := &Task{
		ID:            "task-123",
		Title:         "Update secrets",
		FilesModified: []string{"secrets/api_key.json"},
	}

	result := enforcer.Enforce(context.Background(), task, "Update secrets")

	if result.Allowed {
		t.Error("Expected task to be denied for secrets/ directory modification")
	}
}

func TestPolicyEnforcer_EnforceFiles(t *testing.T) {
	evaluator := &mockPolicyEvaluator{
		allowAll:    false,
		violations:  []string{"Cannot modify .env files"},
		decisionID:  "decision-003",
		policyCount: 1,
	}

	enforcer := NewPolicyEnforcer(evaluator, "session-123")

	// Test allowed files
	result := enforcer.EnforceFiles(context.Background(), []string{"main.go"}, nil)
	if !result.Allowed {
		t.Error("Expected main.go to be allowed")
	}

	// Test denied files
	result = enforcer.EnforceFiles(context.Background(), []string{".env.production"}, nil)
	if result.Allowed {
		t.Error("Expected .env.production to be denied")
	}
}

func TestPolicyEnforcer_HasPolicies(t *testing.T) {
	// No evaluator
	enforcer := NewPolicyEnforcer(nil, "session-123")
	if enforcer.HasPolicies() {
		t.Error("HasPolicies should return false with nil evaluator")
	}

	// Evaluator with no policies
	evaluator := &mockPolicyEvaluator{policyCount: 0}
	enforcer = NewPolicyEnforcer(evaluator, "session-123")
	if enforcer.HasPolicies() {
		t.Error("HasPolicies should return false with 0 policies")
	}

	// Evaluator with policies
	evaluator = &mockPolicyEvaluator{policyCount: 2}
	enforcer = NewPolicyEnforcer(evaluator, "session-123")
	if !enforcer.HasPolicies() {
		t.Error("HasPolicies should return true with policies loaded")
	}
}

func TestPolicyEnforcer_PolicyCount(t *testing.T) {
	enforcer := NewPolicyEnforcer(nil, "session-123")
	if enforcer.PolicyCount() != 0 {
		t.Errorf("PolicyCount should return 0 with nil evaluator, got %d", enforcer.PolicyCount())
	}

	evaluator := &mockPolicyEvaluator{policyCount: 5}
	enforcer = NewPolicyEnforcer(evaluator, "session-123")
	if enforcer.PolicyCount() != 5 {
		t.Errorf("PolicyCount should return 5, got %d", enforcer.PolicyCount())
	}
}

func TestPolicyEnforcer_EvaluationError(t *testing.T) {
	evaluator := &mockPolicyEvaluator{
		err:         context.DeadlineExceeded,
		policyCount: 1,
	}

	enforcer := NewPolicyEnforcer(evaluator, "session-123")

	task := &Task{
		ID:            "task-123",
		Title:         "Test task",
		FilesModified: []string{"main.go"},
	}

	result := enforcer.Enforce(context.Background(), task, "Test goal")

	if result.Allowed {
		t.Error("Expected task to be denied on evaluation error")
	}
	if result.Error == nil {
		t.Error("Expected error to be set on evaluation failure")
	}
}

func TestPolicyEnforcementResult_AllowedByDefault(t *testing.T) {
	// When no evaluator is configured, tasks should be allowed by default
	enforcer := NewPolicyEnforcer(nil, "test-session")

	task := &Task{
		ID:            "task-001",
		Title:         "Dangerous task",
		FilesModified: []string{".env", "secrets/key.json", "config/credentials.yaml"},
	}

	result := enforcer.Enforce(context.Background(), task, "Some goal")

	if !result.Allowed {
		t.Error("With no policy evaluator, all tasks should be allowed")
	}
	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}
}
