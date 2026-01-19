package policy

import (
	"context"
	"testing"

	"github.com/spf13/afero"
)

func TestTestRunner_Run(t *testing.T) {
	// Create test filesystem
	fs := afero.NewMemMapFs()

	// Create a simple policy
	policy := `package taskwing.policy

import rego.v1

is_env_file(file) if startswith(file, ".env")

deny contains msg if {
    some file in input.task.files_modified
    is_env_file(file)
    msg := sprintf("BLOCKED: Environment file '%s' is protected", [file])
}
`

	// Create a test file
	testFile := `package taskwing.policy

import rego.v1

test_deny_env_file if {
    result := deny with input as {"task": {"files_modified": [".env"]}}
    count(result) > 0
}

test_allow_regular_file if {
    result := deny with input as {"task": {"files_modified": ["main.go"]}}
    count(result) == 0
}
`

	// Write files to test filesystem
	policiesDir := "/test/policies"
	_ = afero.WriteFile(fs, policiesDir+"/default.rego", []byte(policy), 0644)
	_ = afero.WriteFile(fs, policiesDir+"/default_test.rego", []byte(testFile), 0644)

	// Create runner and run tests
	runner := NewTestRunner(fs, policiesDir, "/test")
	ctx := context.Background()

	summary, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify results
	if summary.Total != 2 {
		t.Errorf("expected 2 tests, got %d", summary.Total)
	}
	if summary.Passed != 2 {
		t.Errorf("expected 2 passed, got %d passed (failed: %d, errored: %d)",
			summary.Passed, summary.Failed, summary.Errored)
	}
	if !summary.AllPassed() {
		t.Error("expected AllPassed() to return true")
	}
}

func TestTestRunner_Run_NoTests(t *testing.T) {
	// Create test filesystem with no test files
	fs := afero.NewMemMapFs()
	policiesDir := "/test/policies"

	// Only create a policy file (no test file)
	policy := `package taskwing.policy

import rego.v1

deny contains msg if {
    false
    msg := "never"
}
`
	_ = afero.WriteFile(fs, policiesDir+"/default.rego", []byte(policy), 0644)

	runner := NewTestRunner(fs, policiesDir, "/test")
	ctx := context.Background()

	summary, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Should have no tests
	if summary.Total != 0 {
		t.Errorf("expected 0 tests, got %d", summary.Total)
	}
}

func TestTestRunner_HasTests(t *testing.T) {
	fs := afero.NewMemMapFs()
	policiesDir := "/test/policies"

	// Initially no test files
	_ = afero.WriteFile(fs, policiesDir+"/default.rego", []byte("package test"), 0644)

	runner := NewTestRunner(fs, policiesDir, "/test")

	hasTests, err := runner.HasTests()
	if err != nil {
		t.Fatalf("HasTests() failed: %v", err)
	}
	if hasTests {
		t.Error("expected HasTests() to return false when no test files")
	}

	// Add a test file
	_ = afero.WriteFile(fs, policiesDir+"/default_test.rego", []byte("package test"), 0644)

	hasTests, err = runner.HasTests()
	if err != nil {
		t.Fatalf("HasTests() failed: %v", err)
	}
	if !hasTests {
		t.Error("expected HasTests() to return true after adding test file")
	}
}

func TestTestRunner_Run_FailingTest(t *testing.T) {
	fs := afero.NewMemMapFs()
	policiesDir := "/test/policies"

	// Create a policy
	policy := `package taskwing.policy
import rego.v1

deny := false
`

	// Create a failing test
	testFile := `package taskwing.policy
import rego.v1

test_should_fail if {
    1 == 2  # This will always fail
}
`

	_ = afero.WriteFile(fs, policiesDir+"/default.rego", []byte(policy), 0644)
	_ = afero.WriteFile(fs, policiesDir+"/default_test.rego", []byte(testFile), 0644)

	runner := NewTestRunner(fs, policiesDir, "/test")
	ctx := context.Background()

	summary, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if summary.Failed != 1 {
		t.Errorf("expected 1 failed test, got %d", summary.Failed)
	}
	if summary.AllPassed() {
		t.Error("expected AllPassed() to return false for failing test")
	}
}

func TestTestSummary_FormatSummary(t *testing.T) {
	summary := &TestSummary{
		Total:  5,
		Passed: 3,
		Failed: 1,
		Errored: 1,
	}

	output := summary.FormatSummary()
	if output == "" {
		t.Error("FormatSummary() returned empty string")
	}
	// Should contain the counts
	if !contains(output, "5 tests") {
		t.Errorf("expected output to contain '5 tests', got: %s", output)
	}
	if !contains(output, "3 passed") {
		t.Errorf("expected output to contain '3 passed', got: %s", output)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
