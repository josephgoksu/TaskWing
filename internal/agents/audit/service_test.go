package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/task"
)

// MockExecutor is a mock implementation of CommandExecutor for testing.
type MockExecutor struct {
	BuildStdout string
	BuildStderr string
	BuildErr    error
	TestStdout  string
	TestStderr  string
	TestErr     error
}

func (m *MockExecutor) Execute(ctx context.Context, workDir, name string, args ...string) (string, string, error) {
	// Determine which command is being run
	if name == "go" && len(args) > 0 {
		if args[0] == "build" {
			return m.BuildStdout, m.BuildStderr, m.BuildErr
		}
		if args[0] == "test" {
			return m.TestStdout, m.TestStderr, m.TestErr
		}
	}
	return "", "", nil
}

// MockLLMClient is a mock implementation of LLMClient for testing.
type MockLLMClient struct {
	Response string
	Err      error
}

func (m *MockLLMClient) Generate(ctx context.Context, messages []*schema.Message) (string, error) {
	return m.Response, m.Err
}

func TestAudit_AllPass(t *testing.T) {
	executor := &MockExecutor{
		BuildStdout: "",
		BuildErr:    nil,
		TestStdout:  "ok  	github.com/test/pkg	0.001s\n",
		TestErr:     nil,
	}

	llmClient := &MockLLMClient{
		Response: `{"passed": true, "issues": [], "summary": "All requirements met"}`,
	}

	service := NewServiceWithDeps("/tmp", executor, llmClient)

	plan := &task.Plan{
		ID:   "plan-123",
		Goal: "Test plan",
		Tasks: []task.Task{
			{
				Title:             "Task 1",
				Status:            task.StatusCompleted,
				CompletionSummary: "Done",
			},
		},
	}

	result, err := service.Audit(context.Background(), plan)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Status != "passed" {
		t.Errorf("Expected status 'passed', got '%s'", result.Status)
	}

	if !result.BuildResult.Passed {
		t.Error("Expected build to pass")
	}

	if !result.TestResult.Passed {
		t.Error("Expected tests to pass")
	}

	if !result.SemanticResult.Passed {
		t.Error("Expected semantic check to pass")
	}
}

func TestAudit_BuildFails(t *testing.T) {
	executor := &MockExecutor{
		BuildStderr: "undefined: someFunction",
		BuildErr:    errors.New("exit status 1"),
		TestStdout:  "",
		TestErr:     nil,
	}

	llmClient := &MockLLMClient{
		Response: `{"passed": true, "issues": [], "summary": "OK"}`,
	}

	service := NewServiceWithDeps("/tmp", executor, llmClient)

	plan := &task.Plan{
		ID:   "plan-123",
		Goal: "Test plan",
	}

	result, err := service.Audit(context.Background(), plan)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", result.Status)
	}

	if result.BuildResult.Passed {
		t.Error("Expected build to fail")
	}

	if result.BuildResult.Error != "exit status 1" {
		t.Errorf("Expected build error 'exit status 1', got '%s'", result.BuildResult.Error)
	}
}

func TestAudit_TestsFail(t *testing.T) {
	executor := &MockExecutor{
		BuildStdout: "",
		BuildErr:    nil,
		TestStderr:  "--- FAIL: TestSomething\n",
		TestErr:     errors.New("exit status 1"),
	}

	llmClient := &MockLLMClient{
		Response: `{"passed": true, "issues": [], "summary": "OK"}`,
	}

	service := NewServiceWithDeps("/tmp", executor, llmClient)

	plan := &task.Plan{
		ID:   "plan-123",
		Goal: "Test plan",
	}

	result, err := service.Audit(context.Background(), plan)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", result.Status)
	}

	if !result.BuildResult.Passed {
		t.Error("Expected build to pass")
	}

	if result.TestResult.Passed {
		t.Error("Expected tests to fail")
	}
}

func TestAudit_SemanticFails(t *testing.T) {
	executor := &MockExecutor{
		BuildStdout: "",
		BuildErr:    nil,
		TestStdout:  "ok\n",
		TestErr:     nil,
	}

	llmClient := &MockLLMClient{
		Response: `{"passed": false, "issues": ["Missing error handling", "No tests added"], "summary": "Requirements not fully met"}`,
	}

	service := NewServiceWithDeps("/tmp", executor, llmClient)

	plan := &task.Plan{
		ID:   "plan-123",
		Goal: "Add error handling",
	}

	result, err := service.Audit(context.Background(), plan)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", result.Status)
	}

	if result.SemanticResult.Passed {
		t.Error("Expected semantic check to fail")
	}

	if len(result.SemanticResult.Issues) != 2 {
		t.Errorf("Expected 2 issues, got %d", len(result.SemanticResult.Issues))
	}
}

func TestAudit_LLMError(t *testing.T) {
	executor := &MockExecutor{
		BuildStdout: "",
		BuildErr:    nil,
		TestStdout:  "ok\n",
		TestErr:     nil,
	}

	llmClient := &MockLLMClient{
		Err: errors.New("API rate limit exceeded"),
	}

	service := NewServiceWithDeps("/tmp", executor, llmClient)

	plan := &task.Plan{
		ID:   "plan-123",
		Goal: "Test plan",
	}

	result, err := service.Audit(context.Background(), plan)
	if err != nil {
		t.Fatalf("Audit failed unexpectedly: %v", err)
	}

	// LLM error should result in semantic check failing, not the whole audit
	if result.SemanticResult.Passed {
		t.Error("Expected semantic check to fail when LLM errors")
	}
}

func TestAudit_NilPlan(t *testing.T) {
	service := NewServiceWithDeps("/tmp", &MockExecutor{}, &MockLLMClient{})

	_, err := service.Audit(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil plan")
	}
}

func TestAudit_NoLLMClient(t *testing.T) {
	executor := &MockExecutor{
		BuildStdout: "",
		BuildErr:    nil,
		TestStdout:  "ok\n",
		TestErr:     nil,
	}

	service := NewServiceWithDeps("/tmp", executor, nil)

	plan := &task.Plan{
		ID:   "plan-123",
		Goal: "Test plan",
	}

	result, err := service.Audit(context.Background(), plan)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	// Without LLM client, semantic check should be skipped (passed)
	if result.Status != "passed" {
		t.Errorf("Expected status 'passed' when no LLM, got '%s'", result.Status)
	}

	if !result.SemanticResult.Passed {
		t.Error("Expected semantic check to pass when skipped")
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "raw json",
			input:    `{"passed": true}`,
			expected: `{"passed": true}`,
		},
		{
			name:     "json in code block",
			input:    "```json\n{\"passed\": true}\n```",
			expected: `{"passed": true}`,
		},
		{
			name:     "json in generic code block",
			input:    "```\n{\"passed\": false}\n```",
			expected: `{"passed": false}`,
		},
		{
			name:     "json with text before",
			input:    "Here is the result:\n{\"passed\": true, \"summary\": \"ok\"}",
			expected: `{"passed": true, "summary": "ok"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractJSON(tc.input)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestToAuditReport(t *testing.T) {
	result := &AuditResult{
		PlanID: "plan-123",
		Status: "failed",
		BuildResult: VerificationResult{
			Passed: false,
			Output: "build failed",
			Error:  "exit 1",
		},
		TestResult: VerificationResult{
			Passed: true,
			Output: "ok",
		},
		SemanticResult: SemanticResult{
			Passed: false,
			Issues: []string{"Missing tests"},
		},
		Timestamp: time.Now(),
	}

	report := result.ToAuditReport()

	if report.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", report.Status)
	}

	if report.BuildOutput != "build failed" {
		t.Errorf("Expected BuildOutput 'build failed', got '%s'", report.BuildOutput)
	}

	if report.ErrorMessage != "Build failed: exit 1" {
		t.Errorf("Expected ErrorMessage to contain build error, got '%s'", report.ErrorMessage)
	}

	if len(report.SemanticIssues) != 1 {
		t.Errorf("Expected 1 semantic issue, got %d", len(report.SemanticIssues))
	}
}

func TestBuildSemanticPrompt(t *testing.T) {
	service := NewServiceWithDeps("/tmp", &MockExecutor{}, nil)

	plan := &task.Plan{
		ID:           "plan-123",
		Goal:         "Add user authentication",
		EnrichedGoal: "Implement JWT-based authentication with login and logout endpoints",
		Tasks: []task.Task{
			{
				Title:             "Create login endpoint",
				Status:            task.StatusCompleted,
				CompletionSummary: "Added /api/login endpoint",
				FilesModified:     []string{"api/auth.go"},
			},
			{
				Title:  "Pending task",
				Status: task.StatusPending,
			},
		},
	}

	prompt := service.buildSemanticPrompt(plan)

	if !contains(prompt, "Add user authentication") {
		t.Error("Prompt should contain goal")
	}

	if !contains(prompt, "JWT-based authentication") {
		t.Error("Prompt should contain enriched goal")
	}

	if !contains(prompt, "Create login endpoint") {
		t.Error("Prompt should contain completed task")
	}

	if contains(prompt, "Pending task") {
		t.Error("Prompt should not contain pending tasks")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// === Auto-Fix Loop Tests ===

// StatefulMockExecutor changes behavior based on call count.
type StatefulMockExecutor struct {
	CallCount   int
	PassOnRetry int // Which retry to pass on (0 = never pass)
}

func (m *StatefulMockExecutor) Execute(ctx context.Context, workDir, name string, args ...string) (string, string, error) {
	m.CallCount++
	if name == "go" && len(args) > 0 {
		if args[0] == "build" {
			if m.PassOnRetry > 0 && m.CallCount >= m.PassOnRetry {
				return "", "", nil
			}
			return "", "undefined: someFunc", errors.New("exit status 1")
		}
		if args[0] == "test" {
			if m.PassOnRetry > 0 && m.CallCount >= m.PassOnRetry {
				return "ok", "", nil
			}
			return "", "FAIL", errors.New("exit status 1")
		}
	}
	return "", "", nil
}

// StatefulMockLLM provides different responses for audit vs fix.
type StatefulMockLLM struct {
	AuditResponse string
	FixResponse   string
	CallCount     int
}

func (m *StatefulMockLLM) Generate(ctx context.Context, messages []*schema.Message) (string, error) {
	m.CallCount++
	// Determine if this is an audit or fix call based on system message
	for _, msg := range messages {
		if msg.Role == schema.System && containsHelper(msg.Content, "code repair") {
			return m.FixResponse, nil
		}
	}
	return m.AuditResponse, nil
}

func TestAuditWithAutoFix_PassesOnFirstTry(t *testing.T) {
	executor := &MockExecutor{
		BuildStdout: "",
		BuildErr:    nil,
		TestStdout:  "ok",
		TestErr:     nil,
	}

	llmClient := &MockLLMClient{
		Response: `{"passed": true, "issues": [], "summary": "OK"}`,
	}

	service := NewServiceWithDeps("/tmp", executor, llmClient)

	plan := &task.Plan{
		ID:   "plan-123",
		Goal: "Test plan",
	}

	result, err := service.AuditWithAutoFix(context.Background(), plan)
	if err != nil {
		t.Fatalf("AuditWithAutoFix failed: %v", err)
	}

	if result.FinalStatus != "verified" {
		t.Errorf("Expected status 'verified', got '%s'", result.FinalStatus)
	}

	if result.Attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", result.Attempts)
	}

	if len(result.FixesApplied) != 0 {
		t.Errorf("Expected no fixes, got %d", len(result.FixesApplied))
	}
}

func TestAuditWithAutoFix_FailsAfterMaxRetries(t *testing.T) {
	executor := &MockExecutor{
		BuildStderr: "undefined: someFunc",
		BuildErr:    errors.New("exit status 1"),
		TestStdout:  "",
		TestErr:     nil,
	}

	// LLM suggests no fix (cannot auto-fix)
	llmClient := &StatefulMockLLM{
		AuditResponse: `{"passed": false, "issues": ["Build failed"], "summary": "Build error"}`,
		FixResponse:   `{"filePath": "", "description": "Cannot auto-fix: unclear error", "newCode": ""}`,
	}

	service := NewServiceWithDeps("/tmp", executor, llmClient)

	plan := &task.Plan{
		ID:   "plan-123",
		Goal: "Test plan",
	}

	result, err := service.AuditWithAutoFix(context.Background(), plan)
	if err != nil {
		t.Fatalf("AuditWithAutoFix failed: %v", err)
	}

	if result.FinalStatus != "needs_revision" {
		t.Errorf("Expected status 'needs_revision', got '%s'", result.FinalStatus)
	}

	if result.Attempts != MaxRetries {
		t.Errorf("Expected %d attempts, got %d", MaxRetries, result.Attempts)
	}
}

func TestAuditWithAutoFix_NilPlan(t *testing.T) {
	service := NewServiceWithDeps("/tmp", &MockExecutor{}, &MockLLMClient{})

	_, err := service.AuditWithAutoFix(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil plan")
	}
}

func TestAuditWithAutoFix_NoLLMClient(t *testing.T) {
	service := NewServiceWithDeps("/tmp", &MockExecutor{}, nil)

	plan := &task.Plan{
		ID:   "plan-123",
		Goal: "Test plan",
	}

	_, err := service.AuditWithAutoFix(context.Background(), plan)
	if err == nil {
		t.Error("Expected error when no LLM client")
	}
}

func TestAuditWithAutoFix_HistoryTracking(t *testing.T) {
	executor := &MockExecutor{
		BuildStderr: "error",
		BuildErr:    errors.New("exit status 1"),
	}

	llmClient := &StatefulMockLLM{
		AuditResponse: `{"passed": false, "issues": [], "summary": "Failed"}`,
		FixResponse:   `{"filePath": "", "description": "Cannot fix", "newCode": ""}`,
	}

	service := NewServiceWithDeps("/tmp", executor, llmClient)

	plan := &task.Plan{
		ID:   "plan-123",
		Goal: "Test plan",
	}

	result, err := service.AuditWithAutoFix(context.Background(), plan)
	if err != nil {
		t.Fatalf("AuditWithAutoFix failed: %v", err)
	}

	if len(result.History) != MaxRetries {
		t.Errorf("Expected %d history records, got %d", MaxRetries, len(result.History))
	}

	// Verify each attempt is recorded
	for i, record := range result.History {
		if record.Attempt != i+1 {
			t.Errorf("Expected attempt %d, got %d", i+1, record.Attempt)
		}
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 100, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is too long", 10, "this is to\n... (truncated)"},
	}

	for _, tc := range tests {
		result := truncateOutput(tc.input, tc.maxLen)
		if result != tc.expected {
			t.Errorf("truncateOutput(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.expected)
		}
	}
}

func TestToAuditReportWithFixes(t *testing.T) {
	result := &AutoFixResult{
		FinalStatus:  "verified",
		Attempts:     2,
		FixesApplied: []string{"Fixed import"},
		FinalAudit: &AuditResult{
			BuildResult: VerificationResult{Passed: true, Output: "ok"},
			TestResult:  VerificationResult{Passed: true, Output: "ok"},
		},
	}

	report := result.ToAuditReportWithFixes()

	if report.Status != "passed" {
		t.Errorf("Expected status 'passed', got '%s'", report.Status)
	}

	if report.RetryCount != 2 {
		t.Errorf("Expected RetryCount 2, got %d", report.RetryCount)
	}

	if len(report.FixesApplied) != 1 {
		t.Errorf("Expected 1 fix, got %d", len(report.FixesApplied))
	}
}

func TestToAuditReportWithFixes_NeedsRevision(t *testing.T) {
	result := &AutoFixResult{
		FinalStatus: "needs_revision",
		Attempts:    3,
		FinalAudit: &AuditResult{
			BuildResult: VerificationResult{Passed: false, Error: "exit 1"},
		},
	}

	report := result.ToAuditReportWithFixes()

	if report.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", report.Status)
	}
}
