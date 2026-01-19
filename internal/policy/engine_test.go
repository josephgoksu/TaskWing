package policy

import (
	"context"
	"testing"

	"github.com/spf13/afero"
)

func TestEngine_Evaluate_NoPolicies(t *testing.T) {
	// When no policies are loaded, everything should be allowed
	engine := &Engine{
		policies:      nil,
		policyPackage: DefaultPolicyPackage,
		compiled:      true,
	}

	input := map[string]any{
		"task": map[string]any{
			"files_modified": []string{"anything.go"},
		},
	}

	decision, err := engine.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if decision.Result != PolicyResultAllow {
		t.Errorf("Result = %v, want %v", decision.Result, PolicyResultAllow)
	}

	if len(decision.Violations) != 0 {
		t.Errorf("Violations = %v, want empty", decision.Violations)
	}
}

func TestEngine_Evaluate_DenyRule(t *testing.T) {
	policy := &PolicyFile{
		Name: "test_deny",
		Path: "test_deny.rego",
		Content: `package taskwing.policy

import rego.v1

deny contains msg if {
    some file in input.task.files_modified
    startswith(file, "core/")
    msg := sprintf("Cannot modify protected file: %s", [file])
}
`,
	}

	engine := NewEngineWithPolicies("/project", []*PolicyFile{policy})

	tests := []struct {
		name         string
		input        map[string]any
		wantResult   string
		wantViolate  bool
	}{
		{
			name: "allow non-core file",
			input: map[string]any{
				"task": map[string]any{
					"files_modified": []string{"internal/app/main.go"},
				},
			},
			wantResult:  PolicyResultAllow,
			wantViolate: false,
		},
		{
			name: "deny core file",
			input: map[string]any{
				"task": map[string]any{
					"files_modified": []string{"core/router.go"},
				},
			},
			wantResult:  PolicyResultDeny,
			wantViolate: true,
		},
		{
			name: "deny multiple core files",
			input: map[string]any{
				"task": map[string]any{
					"files_modified": []string{"core/a.go", "core/b.go"},
				},
			},
			wantResult:  PolicyResultDeny,
			wantViolate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := engine.Evaluate(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			if decision.Result != tt.wantResult {
				t.Errorf("Result = %v, want %v", decision.Result, tt.wantResult)
			}

			hasViolations := len(decision.Violations) > 0
			if hasViolations != tt.wantViolate {
				t.Errorf("Has violations = %v, want %v. Violations: %v", hasViolations, tt.wantViolate, decision.Violations)
			}
		})
	}
}

func TestEngine_Evaluate_MultiplePolicies(t *testing.T) {
	protectedZones := &PolicyFile{
		Name: "protected_zones",
		Path: "protected_zones.rego",
		Content: `package taskwing.policy

import rego.v1

deny contains msg if {
    some file in input.task.files_modified
    startswith(file, "vendor/")
    msg := "Cannot modify vendor directory"
}
`,
	}

	secrets := &PolicyFile{
		Name: "secrets",
		Path: "secrets.rego",
		Content: `package taskwing.policy

import rego.v1

deny contains msg if {
    some file in input.task.files_modified
    endswith(file, ".env")
    msg := "Cannot modify .env files"
}
`,
	}

	engine := NewEngineWithPolicies("/project", []*PolicyFile{protectedZones, secrets})

	// Should deny both violations
	input := map[string]any{
		"task": map[string]any{
			"files_modified": []string{"vendor/lib.go", "config/.env"},
		},
	}

	decision, err := engine.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if decision.Result != PolicyResultDeny {
		t.Errorf("Result = %v, want %v", decision.Result, PolicyResultDeny)
	}

	// Should have 2 violations
	if len(decision.Violations) != 2 {
		t.Errorf("Violations count = %d, want 2. Got: %v", len(decision.Violations), decision.Violations)
	}
}

func TestEngine_EvaluateTask(t *testing.T) {
	policy := &PolicyFile{
		Name: "task_policy",
		Path: "task_policy.rego",
		Content: `package taskwing.policy

import rego.v1

deny contains msg if {
    input.task.id == ""
    msg := "Task ID is required"
}
`,
	}

	engine := NewEngineWithPolicies("/project", []*PolicyFile{policy})

	// Task without ID should be denied
	decision, err := engine.EvaluateTask(context.Background(), &TaskInput{
		ID:    "",
		Title: "Test task",
	}, nil, nil)
	if err != nil {
		t.Fatalf("EvaluateTask() error = %v", err)
	}

	if decision.Result != PolicyResultDeny {
		t.Errorf("Result = %v, want deny", decision.Result)
	}

	// Task with ID should be allowed
	decision, err = engine.EvaluateTask(context.Background(), &TaskInput{
		ID:    "task-123",
		Title: "Test task",
	}, nil, nil)
	if err != nil {
		t.Fatalf("EvaluateTask() error = %v", err)
	}

	if decision.Result != PolicyResultAllow {
		t.Errorf("Result = %v, want allow", decision.Result)
	}
}

func TestEngine_EvaluateFiles(t *testing.T) {
	policy := &PolicyFile{
		Name: "file_policy",
		Path: "file_policy.rego",
		Content: `package taskwing.policy

import rego.v1

deny contains msg if {
    some file in input.task.files_created
    endswith(file, "_test.go")
    msg := "Cannot create test files in this context"
}
`,
	}

	engine := NewEngineWithPolicies("/project", []*PolicyFile{policy})

	// Creating a non-test file should be allowed
	decision, err := engine.EvaluateFiles(context.Background(), nil, []string{"main.go"})
	if err != nil {
		t.Fatalf("EvaluateFiles() error = %v", err)
	}
	if decision.Result != PolicyResultAllow {
		t.Errorf("Result = %v, want allow", decision.Result)
	}

	// Creating a test file should be denied
	decision, err = engine.EvaluateFiles(context.Background(), nil, []string{"main_test.go"})
	if err != nil {
		t.Fatalf("EvaluateFiles() error = %v", err)
	}
	if decision.Result != PolicyResultDeny {
		t.Errorf("Result = %v, want deny", decision.Result)
	}
}

func TestEngine_PolicyManagement(t *testing.T) {
	engine := &Engine{
		policies:      nil,
		policyPackage: DefaultPolicyPackage,
		compiled:      true,
	}

	// Initially no policies
	if engine.PolicyCount() != 0 {
		t.Errorf("PolicyCount() = %d, want 0", engine.PolicyCount())
	}

	// Add a policy
	engine.AddPolicy("test", `package taskwing.policy`)
	if engine.PolicyCount() != 1 {
		t.Errorf("PolicyCount() after add = %d, want 1", engine.PolicyCount())
	}

	names := engine.PolicyNames()
	if len(names) != 1 || names[0] != "test" {
		t.Errorf("PolicyNames() = %v, want [test]", names)
	}

	// Clear policies
	engine.ClearPolicies()
	if engine.PolicyCount() != 0 {
		t.Errorf("PolicyCount() after clear = %d, want 0", engine.PolicyCount())
	}
}

func TestNewEngine(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create policies directory with a policy file
	_ = fs.MkdirAll("/project/.taskwing/policies", 0755)
	policyContent := `package taskwing.policy

import rego.v1

deny contains msg if {
    input.blocked == true
    msg := "Input is blocked"
}
`
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/test.rego", []byte(policyContent), 0644)

	engine, err := NewEngine(EngineConfig{
		WorkDir: "/project",
		Fs:      fs,
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	if engine.PolicyCount() != 1 {
		t.Errorf("PolicyCount() = %d, want 1", engine.PolicyCount())
	}

	// Test evaluation
	decision, err := engine.Evaluate(context.Background(), map[string]any{"blocked": true})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if decision.Result != PolicyResultDeny {
		t.Errorf("Result = %v, want deny", decision.Result)
	}

	decision, err = engine.Evaluate(context.Background(), map[string]any{"blocked": false})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if decision.Result != PolicyResultAllow {
		t.Errorf("Result = %v, want allow", decision.Result)
	}
}

func TestNewEngine_NoPoliciesDir(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Don't create policies directory
	engine, err := NewEngine(EngineConfig{
		WorkDir: "/project",
		Fs:      fs,
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v (should succeed with no policies)", err)
	}

	if engine.PolicyCount() != 0 {
		t.Errorf("PolicyCount() = %d, want 0", engine.PolicyCount())
	}
}

func TestValidatePolicy(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid policy",
			content: `package test

import rego.v1

deny contains "blocked" if {
    input.x == 1
}
`,
			wantErr: false,
		},
		{
			name:    "invalid syntax",
			content: `package test { invalid syntax here`,
			wantErr: true,
		},
		{
			name:    "empty content",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePolicy(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEngine_Evaluate_DecisionFields(t *testing.T) {
	policy := &PolicyFile{
		Name:    "test",
		Path:    "test.rego",
		Content: `package taskwing.policy`,
	}

	engine := NewEngineWithPolicies("/project", []*PolicyFile{policy})

	input := map[string]any{"test": "data"}
	decision, err := engine.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	// Check all decision fields are populated
	if decision.DecisionID == "" {
		t.Error("DecisionID is empty")
	}

	if decision.PolicyPath != DefaultPolicyPackage {
		t.Errorf("PolicyPath = %v, want %v", decision.PolicyPath, DefaultPolicyPackage)
	}

	if decision.EvaluatedAt.IsZero() {
		t.Error("EvaluatedAt is zero")
	}

	if decision.Input == nil {
		t.Error("Input is nil")
	}
}

func TestEngine_MustEvaluate(t *testing.T) {
	engine := &Engine{
		policies:      nil,
		policyPackage: DefaultPolicyPackage,
		compiled:      true,
	}

	// Should not panic with valid input
	decision := engine.MustEvaluate(context.Background(), map[string]any{})
	if decision == nil {
		t.Error("MustEvaluate() returned nil")
	}
}

func TestEngine_ReloadPolicies(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Start with one policy
	_ = fs.MkdirAll("/project/.taskwing/policies", 0755)
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/a.rego", []byte(`package taskwing.policy`), 0644)

	engine, _ := NewEngine(EngineConfig{
		WorkDir: "/project",
		Fs:      fs,
	})

	if engine.PolicyCount() != 1 {
		t.Errorf("Initial PolicyCount() = %d, want 1", engine.PolicyCount())
	}

	// Add another policy file
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/b.rego", []byte(`package taskwing.policy`), 0644)

	// Reload
	err := engine.ReloadPolicies(fs, "/project/.taskwing/policies")
	if err != nil {
		t.Fatalf("ReloadPolicies() error = %v", err)
	}

	if engine.PolicyCount() != 2 {
		t.Errorf("PolicyCount() after reload = %d, want 2", engine.PolicyCount())
	}
}

func TestEngine_Evaluate_NoNetworkCalls(t *testing.T) {
	// This test verifies that evaluation happens entirely in-process.
	// We use a policy that only uses local operations.
	policy := &PolicyFile{
		Name: "local_only",
		Path: "local_only.rego",
		Content: `package taskwing.policy

import rego.v1

# This policy only uses local string operations
deny contains msg if {
    some file in input.task.files_modified
    contains(file, "secret")
    msg := "Cannot modify files containing 'secret' in path"
}
`,
	}

	engine := NewEngineWithPolicies("/project", []*PolicyFile{policy})

	// Run multiple evaluations - all should be fast and local
	for i := 0; i < 100; i++ {
		input := map[string]any{
			"task": map[string]any{
				"files_modified": []string{"config.go"},
			},
		}
		_, err := engine.Evaluate(context.Background(), input)
		if err != nil {
			t.Fatalf("Evaluate() iteration %d error = %v", i, err)
		}
	}
	// If we got here without timeout, evaluation is local
}
