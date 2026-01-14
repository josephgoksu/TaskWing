package planner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSemanticMiddleware_ValidPlan(t *testing.T) {
	// Create a valid plan with no file references
	plan := &LLMPlanResponse{
		GoalSummary:         "Test plan",
		Rationale:           "This is a test plan with valid tasks",
		EstimatedComplexity: "low",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Implement feature",
				Description:        "Add new feature to the system",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"Feature works correctly"},
				ValidationSteps:    []string{"echo 'test'"},
			},
		},
	}

	middleware := NewSemanticMiddleware(MiddlewareConfig{
		SkipFileValidation: true, // No file paths in this test
	})

	result := middleware.Validate(plan)

	if !result.Valid {
		t.Errorf("Expected valid plan, got errors: %s", result.ErrorSummary())
	}
}

func TestSemanticMiddleware_MissingFile(t *testing.T) {
	plan := &LLMPlanResponse{
		GoalSummary:         "Test plan",
		Rationale:           "Plan references non-existent file",
		EstimatedComplexity: "low",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Fix bug in handler",
				Description:        "Update the handler in /nonexistent/path/handler.go",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"Bug is fixed"},
				ValidationSteps:    []string{"go test ./..."},
			},
		},
	}

	middleware := NewSemanticMiddleware(MiddlewareConfig{})

	result := middleware.Validate(plan)

	if result.Valid {
		t.Error("Expected validation to fail for missing file")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected at least one error")
	}

	foundMissingFile := false
	for _, e := range result.Errors {
		if e.Type == "missing_file" {
			foundMissingFile = true
			break
		}
	}
	if !foundMissingFile {
		t.Error("Expected missing_file error type")
	}
}

func TestSemanticMiddleware_MissingFileAsWarning(t *testing.T) {
	plan := &LLMPlanResponse{
		GoalSummary:         "Test plan",
		Rationale:           "Plan references non-existent file",
		EstimatedComplexity: "low",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Fix bug in handler",
				Description:        "Update the handler in /nonexistent/path/handler.go",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"Bug is fixed"},
				ValidationSteps:    []string{"echo done"},
			},
		},
	}

	middleware := NewSemanticMiddleware(MiddlewareConfig{
		AllowMissingFiles: true,
	})

	result := middleware.Validate(plan)

	if !result.Valid {
		t.Error("Expected valid when AllowMissingFiles is true")
	}

	if len(result.Warnings) == 0 {
		t.Error("Expected at least one warning")
	}
}

func TestSemanticMiddleware_ExistingFile(t *testing.T) {
	// Create a temp file with a known structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "internal")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(subDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package test"), 0644); err != nil {
		t.Fatal(err)
	}

	plan := &LLMPlanResponse{
		GoalSummary:         "Test plan",
		Rationale:           "Plan references existing file",
		EstimatedComplexity: "low",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Update test file",
				Description:        "Modify internal/test.go for the fix",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"File is updated"},
				ValidationSteps:    []string{"echo done"},
			},
		},
	}

	middleware := NewSemanticMiddleware(MiddlewareConfig{
		BasePath: tmpDir,
	})

	result := middleware.Validate(plan)

	if !result.Valid {
		t.Errorf("Expected valid plan for existing file, got: %s", result.ErrorSummary())
	}
}

func TestSemanticMiddleware_InvalidCommand(t *testing.T) {
	plan := &LLMPlanResponse{
		GoalSummary:         "Test plan",
		Rationale:           "Plan with invalid shell command",
		EstimatedComplexity: "low",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Run tests",
				Description:        "Execute test suite",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "qa",
				AcceptanceCriteria: []string{"Tests pass"},
				ValidationSteps:    []string{"echo 'unclosed quote"}, // Invalid syntax
			},
		},
	}

	middleware := NewSemanticMiddleware(MiddlewareConfig{
		SkipFileValidation: true,
	})

	result := middleware.Validate(plan)

	if result.Valid {
		t.Error("Expected validation to fail for invalid command")
	}

	foundInvalidCommand := false
	for _, e := range result.Errors {
		if e.Type == "invalid_command" {
			foundInvalidCommand = true
			break
		}
	}
	if !foundInvalidCommand {
		t.Error("Expected invalid_command error type")
	}
}

func TestSemanticMiddleware_ValidCommand(t *testing.T) {
	plan := &LLMPlanResponse{
		GoalSummary:         "Test plan",
		Rationale:           "Plan with valid shell commands",
		EstimatedComplexity: "low",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Run tests",
				Description:        "Execute test suite",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "qa",
				AcceptanceCriteria: []string{"Tests pass"},
				ValidationSteps: []string{
					"go test ./...",
					"echo 'done'",
					"ls -la && pwd",
				},
			},
		},
	}

	middleware := NewSemanticMiddleware(MiddlewareConfig{
		SkipFileValidation: true,
	})

	result := middleware.Validate(plan)

	if !result.Valid {
		t.Errorf("Expected valid plan, got: %s", result.ErrorSummary())
	}

	if result.Stats.CommandsValidated != 3 {
		t.Errorf("Expected 3 commands validated, got %d", result.Stats.CommandsValidated)
	}
}

func TestSemanticMiddleware_CreationContext(t *testing.T) {
	plan := &LLMPlanResponse{
		GoalSummary:         "Test plan",
		Rationale:           "Plan that creates a new file",
		EstimatedComplexity: "low",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Create new handler",
				Description:        "Create a new file internal/handlers/new_handler.go",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"File is created"},
				ValidationSteps:    []string{"echo done"},
			},
		},
	}

	middleware := NewSemanticMiddleware(MiddlewareConfig{})

	result := middleware.Validate(plan)

	// Should be valid because the file is mentioned in a "create" context
	if !result.Valid {
		t.Errorf("Expected valid plan for creation context, got: %s", result.ErrorSummary())
	}
}

func TestSemanticMiddleware_Stats(t *testing.T) {
	plan := &LLMPlanResponse{
		GoalSummary:         "Test plan",
		Rationale:           "Multi-task plan",
		EstimatedComplexity: "medium",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Task 1",
				Description:        "First task",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"Done"},
				ValidationSteps:    []string{"echo 1"},
			},
			{
				Title:              "Task 2",
				Description:        "Second task",
				Priority:           60,
				Complexity:         "low",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"Done"},
				ValidationSteps:    []string{"echo 2", "echo 3"},
			},
		},
	}

	middleware := NewSemanticMiddleware(MiddlewareConfig{
		SkipFileValidation: true,
	})

	result := middleware.Validate(plan)

	if result.Stats.TotalTasks != 2 {
		t.Errorf("Expected 2 total tasks, got %d", result.Stats.TotalTasks)
	}

	if result.Stats.CommandsValidated != 3 {
		t.Errorf("Expected 3 commands validated, got %d", result.Stats.CommandsValidated)
	}
}

func TestSemanticMiddleware_ExtractFilePaths(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{"absolute path", "Update /path/to/file.go", 1},
		{"relative path", "Check internal/handler.go", 1},
		{"quoted path", "Edit `config/settings.yaml`", 1},
		{"multiple paths", "Update file.go and test.ts", 2},
		{"no paths", "Just some text without files", 0},
		{"url should not match", "Visit http://example.com/test.html", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := extractFilePaths(tt.text)
			if len(paths) != tt.expected {
				t.Errorf("extractFilePaths(%q) returned %d paths, want %d: %v",
					tt.text, len(paths), tt.expected, paths)
			}
		})
	}
}

func TestSemanticMiddleware_IsLikelyFilePath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"handler.go", true},
		{"config.yaml", true},
		{"test.ts", true},
		{"noext", false},
		{"http://example.com", false},
		{"file.xyz", false}, // Unknown extension
		{"component.tsx", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isLikelyFilePath(tt.path)
			if result != tt.expected {
				t.Errorf("isLikelyFilePath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestSemanticMiddleware_ErrorSummary(t *testing.T) {
	result := SemanticValidationResult{
		Valid: false,
		Errors: []SemanticError{
			{TaskIndex: 0, TaskTitle: "Task 1", Type: "missing_file", Message: "File not found"},
			{TaskIndex: 1, TaskTitle: "Task 2", Type: "invalid_command", Message: "Syntax error"},
		},
	}

	summary := result.ErrorSummary()

	if summary == "" {
		t.Error("Expected non-empty error summary")
	}
	if !contains(summary, "Task 1") {
		t.Error("Expected summary to contain Task 1")
	}
	if !contains(summary, "Task 2") {
		t.Error("Expected summary to contain Task 2")
	}
}

func TestSemanticMiddleware_WarningSummary(t *testing.T) {
	result := SemanticValidationResult{
		Valid: true,
		Warnings: []SemanticWarning{
			{TaskIndex: 0, TaskTitle: "Task 1", Type: "missing_file", Message: "Optional file not found"},
		},
	}

	summary := result.WarningSummary()

	if summary == "" {
		t.Error("Expected non-empty warning summary")
	}
	if !contains(summary, "Task 1") {
		t.Error("Expected summary to contain Task 1")
	}
}

func TestSemanticMiddleware_SkipAllValidation(t *testing.T) {
	plan := &LLMPlanResponse{
		GoalSummary:         "Test plan",
		Rationale:           "Plan with issues that should be skipped",
		EstimatedComplexity: "low",
		Tasks: []LLMTaskSchema{
			{
				Title:              "Task",
				Description:        "Reference /nonexistent/file.go",
				Priority:           50,
				Complexity:         "low",
				AssignedAgent:      "coder",
				AcceptanceCriteria: []string{"Done"},
				ValidationSteps:    []string{"invalid ( syntax"},
			},
		},
	}

	middleware := NewSemanticMiddleware(MiddlewareConfig{
		SkipFileValidation:    true,
		SkipCommandValidation: true,
	})

	result := middleware.Validate(plan)

	if !result.Valid {
		t.Error("Expected valid when all validation is skipped")
	}

	if result.Stats.PathsChecked != 0 {
		t.Error("Expected no paths checked when file validation is skipped")
	}

	if result.Stats.CommandsValidated != 0 {
		t.Error("Expected no commands validated when command validation is skipped")
	}
}
