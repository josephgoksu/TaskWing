package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleCodeTool_InvalidAction(t *testing.T) {
	params := CodeToolParams{
		Action: "invalid_action",
		Query:  "test",
	}

	result, err := HandleCodeTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for invalid action")
	}
	if result.Action != "invalid_action" {
		t.Errorf("expected action 'invalid_action', got %q", result.Action)
	}
}

func TestHandleCodeTool_SearchMissingQuery(t *testing.T) {
	params := CodeToolParams{
		Action: CodeActionSearch,
		Query:  "", // missing query
	}

	result, err := HandleCodeTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing query")
	}
	if result.Action != "search" {
		t.Errorf("expected action 'search', got %q", result.Action)
	}
}

func TestHandleCodeTool_ExplainMissingIdentifier(t *testing.T) {
	params := CodeToolParams{
		Action:   CodeActionExplain,
		Query:    "",
		SymbolID: 0, // both missing
	}

	result, err := HandleCodeTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing query/symbol_id")
	}
}

func TestHandleCodeTool_CallersMissingIdentifier(t *testing.T) {
	params := CodeToolParams{
		Action:   CodeActionCallers,
		Query:    "",
		SymbolID: 0, // both missing
	}

	result, err := HandleCodeTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing query/symbol_id")
	}
}

func TestHandleCodeTool_ImpactMissingIdentifier(t *testing.T) {
	params := CodeToolParams{
		Action:   CodeActionImpact,
		Query:    "",
		SymbolID: 0, // both missing
	}

	result, err := HandleCodeTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing query/symbol_id")
	}
}

func TestHandleCodeTool_ActionRouting(t *testing.T) {
	// Test actions that have validation before hitting the repo
	// (search, explain, callers, impact all require query/symbol_id)
	tests := []struct {
		action        CodeAction
		name          string
		expectError   bool
		errorContains string
	}{
		{CodeActionSearch, "search", true, "query is required"},
		{CodeActionExplain, "explain", true, "query or symbol_id is required"},
		{CodeActionCallers, "callers", true, "symbol_id or query"},
		{CodeActionImpact, "impact", true, "symbol_id or query"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := CodeToolParams{
				Action: tt.action,
				// Intentionally missing required fields to trigger validation error
			}

			result, err := HandleCodeTool(context.Background(), nil, params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Action != tt.name {
				t.Errorf("expected action %q, got %q", tt.name, result.Action)
			}

			if tt.expectError && result.Error == "" {
				t.Error("expected validation error")
			}
		})
	}
}

// === Task Tool Handler Tests ===

func TestHandleTaskTool_InvalidAction(t *testing.T) {
	params := TaskToolParams{
		Action: "invalid_action",
	}

	result, err := HandleTaskTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for invalid action")
	}
	if result.Action != "invalid_action" {
		t.Errorf("expected action 'invalid_action', got %q", result.Action)
	}
}

func TestHandleTaskTool_StartMissingTaskID(t *testing.T) {
	params := TaskToolParams{
		Action:    TaskActionStart,
		TaskID:    "", // missing
		SessionID: "session-123",
	}

	result, err := HandleTaskTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing task_id")
	}
	if result.Action != "start" {
		t.Errorf("expected action 'start', got %q", result.Action)
	}
}

func TestHandleTaskTool_StartMissingSessionID(t *testing.T) {
	params := TaskToolParams{
		Action:    TaskActionStart,
		TaskID:    "task-123",
		SessionID: "", // missing
	}

	result, err := HandleTaskTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing session_id")
	}
}

func TestHandleTaskTool_CompleteMissingTaskID(t *testing.T) {
	params := TaskToolParams{
		Action: TaskActionComplete,
		TaskID: "", // missing
	}

	result, err := HandleTaskTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing task_id")
	}
	if result.Action != "complete" {
		t.Errorf("expected action 'complete', got %q", result.Action)
	}
}

func TestHandleTaskTool_NextMissingSessionID(t *testing.T) {
	params := TaskToolParams{
		Action:    TaskActionNext,
		SessionID: "", // missing
	}

	result, err := HandleTaskTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing session_id")
	}
	if !strings.Contains(result.Error, "session_id") {
		t.Errorf("error should mention session_id: %s", result.Error)
	}
	if result.Action != "next" {
		t.Errorf("expected action 'next', got %q", result.Action)
	}
	// Should have actionable guidance in content
	if !strings.Contains(result.Content, "session") {
		t.Error("content should mention session for guidance")
	}
}

func TestHandleTaskTool_CurrentMissingSessionID(t *testing.T) {
	params := TaskToolParams{
		Action:    TaskActionCurrent,
		SessionID: "", // missing
	}

	result, err := HandleTaskTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing session_id")
	}
	if !strings.Contains(result.Error, "session_id") {
		t.Errorf("error should mention session_id: %s", result.Error)
	}
	if result.Action != "current" {
		t.Errorf("expected action 'current', got %q", result.Action)
	}
}

func TestHandleTaskTool_ActionRouting(t *testing.T) {
	// Test actions that have validation before hitting the repo
	tests := []struct {
		action      TaskAction
		name        string
		expectError bool
	}{
		{TaskActionNext, "next", true},         // missing session_id
		{TaskActionCurrent, "current", true},   // missing session_id
		{TaskActionStart, "start", true},       // missing task_id
		{TaskActionComplete, "complete", true}, // missing task_id
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := TaskToolParams{
				Action: tt.action,
				// Intentionally missing required fields
			}

			result, err := HandleTaskTool(context.Background(), nil, params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Action != tt.name {
				t.Errorf("expected action %q, got %q", tt.name, result.Action)
			}

			if tt.expectError && result.Error == "" {
				t.Error("expected validation error")
			}
		})
	}
}

// === Plan Tool Handler Tests ===

func TestHandlePlanTool_InvalidAction(t *testing.T) {
	params := PlanToolParams{
		Action: "invalid_action",
		Goal:   "test goal",
	}

	result, err := HandlePlanTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for invalid action")
	}
	if result.Action != "invalid_action" {
		t.Errorf("expected action 'invalid_action', got %q", result.Action)
	}
}

func TestHandlePlanTool_ClarifyMissingGoal(t *testing.T) {
	params := PlanToolParams{
		Action: PlanActionClarify,
		Goal:   "", // missing
	}

	result, err := HandlePlanTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing goal")
	}
	if result.Action != "clarify" {
		t.Errorf("expected action 'clarify', got %q", result.Action)
	}
}

func TestHandlePlanTool_GenerateMissingGoal(t *testing.T) {
	params := PlanToolParams{
		Action:       PlanActionGenerate,
		Goal:         "", // missing
		EnrichedGoal: "some enriched goal",
	}

	result, err := HandlePlanTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing goal")
	}
	if result.Action != "generate" {
		t.Errorf("expected action 'generate', got %q", result.Action)
	}
}

func TestHandlePlanTool_GenerateMissingEnrichedGoal(t *testing.T) {
	params := PlanToolParams{
		Action:       PlanActionGenerate,
		Goal:         "test goal",
		EnrichedGoal: "", // missing
	}

	result, err := HandlePlanTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing enriched_goal")
	}
	if result.Action != "generate" {
		t.Errorf("expected action 'generate', got %q", result.Action)
	}
}

// TestHandlePlanTool_GenerateErrorContainsFieldDetails validates that validation errors
// contain actionable field-level details to help AI clients self-correct.
func TestHandlePlanTool_GenerateErrorContainsFieldDetails(t *testing.T) {
	tests := []struct {
		name           string
		params         PlanToolParams
		expectedFields []string
		fieldCount     int // expected number of missing fields
	}{
		{
			name: "missing_both_fields_lists_both",
			params: PlanToolParams{
				Action: PlanActionGenerate,
				// Both goal and enriched_goal missing
			},
			expectedFields: []string{"goal", "enriched_goal"},
			fieldCount:     2,
		},
		{
			name: "missing_goal_only_lists_goal",
			params: PlanToolParams{
				Action:       PlanActionGenerate,
				EnrichedGoal: "some enriched goal",
			},
			expectedFields: []string{"goal"},
			fieldCount:     1,
		},
		{
			name: "missing_enriched_goal_only_lists_enriched_goal",
			params: PlanToolParams{
				Action: PlanActionGenerate,
				Goal:   "some goal",
			},
			expectedFields: []string{"enriched_goal"},
			fieldCount:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := HandlePlanTool(context.Background(), nil, tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Error should list missing fields
			for _, field := range tt.expectedFields {
				if !strings.Contains(result.Error, field) {
					t.Errorf("error should contain field %q: %s", field, result.Error)
				}
			}

			// Verify correct number of fields reported (check bracket contents)
			if tt.fieldCount == 1 && strings.Contains(result.Error, ", ") {
				t.Errorf("error lists more fields than expected for single missing field: %s", result.Error)
			}

			// Content should have actionable guidance
			if !strings.Contains(result.Content, "clarify") {
				t.Error("content should mention 'clarify' action for guidance")
			}
		})
	}
}

func TestHandlePlanTool_ActionRouting(t *testing.T) {
	// Test actions that have validation before hitting the repo
	tests := []struct {
		action        PlanAction
		name          string
		expectError   bool
		errorContains string
	}{
		{PlanActionClarify, "clarify", true, "goal is required"},
		{PlanActionGenerate, "generate", true, "goal is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := PlanToolParams{
				Action: tt.action,
				// Intentionally missing required fields
			}

			result, err := HandlePlanTool(context.Background(), nil, params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Action != tt.name {
				t.Errorf("expected action %q, got %q", tt.name, result.Action)
			}

			if tt.expectError && result.Error == "" {
				t.Error("expected validation error")
			}
		})
	}
}

// === Path Validation Tests ===

func TestValidateAndResolvePath_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		path        string
		projectRoot string
		wantErr     bool
		errContains string
	}{
		{
			name:        "direct traversal",
			path:        "../../../etc/passwd",
			projectRoot: tmpDir,
			wantErr:     true,
			errContains: "path traversal not allowed",
		},
		{
			name:        "hidden traversal in middle",
			path:        "foo/../../../etc/passwd",
			projectRoot: tmpDir,
			wantErr:     true,
			errContains: "path traversal not allowed",
		},
		{
			name:        "absolute path outside project",
			path:        "/etc/passwd",
			projectRoot: tmpDir,
			wantErr:     true,
			errContains: "path outside project root",
		},
		{
			name:        "relative path no project root",
			path:        "foo/bar.go",
			projectRoot: "",
			wantErr:     true,
			errContains: "cannot resolve relative path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateAndResolvePath(tt.path, tt.projectRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAndResolvePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errContains != "" && !stringContains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

func TestValidateAndResolvePath_ValidPaths(t *testing.T) {
	// Create a temp directory with a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create a subdirectory with a file
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	subFile := filepath.Join(subDir, "sub.go")
	if err := os.WriteFile(subFile, []byte("package sub"), 0644); err != nil {
		t.Fatalf("failed to create sub file: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		projectRoot string
		wantPath    string
	}{
		{
			name:        "relative path in root",
			path:        "test.go",
			projectRoot: tmpDir,
			wantPath:    testFile,
		},
		{
			name:        "relative path in subdir",
			path:        "subdir/sub.go",
			projectRoot: tmpDir,
			wantPath:    subFile,
		},
		{
			name:        "absolute path within project",
			path:        testFile,
			projectRoot: tmpDir,
			wantPath:    testFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAndResolvePath(tt.path, tt.projectRoot)
			if err != nil {
				t.Errorf("validateAndResolvePath() unexpected error: %v", err)
				return
			}
			if got != tt.wantPath {
				t.Errorf("validateAndResolvePath() = %q, want %q", got, tt.wantPath)
			}
		})
	}
}

func TestValidateAndResolvePath_DirectoryRejection(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := validateAndResolvePath(tmpDir, tmpDir)
	if err == nil {
		t.Error("expected error for directory path")
	}
	if !stringContains(err.Error(), "directory") {
		t.Errorf("error %q does not mention directory", err.Error())
	}
}

func TestHandleCodeTool_SimplifyMissingInput(t *testing.T) {
	params := CodeToolParams{
		Action:   CodeActionSimplify,
		Code:     "", // missing
		FilePath: "", // missing
	}

	result, err := HandleCodeTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for missing code/file_path")
	}
	if result.Action != "simplify" {
		t.Errorf("expected action 'simplify', got %q", result.Action)
	}
}

func TestHandleCodeTool_SimplifyPathTraversal(t *testing.T) {
	params := CodeToolParams{
		Action:   CodeActionSimplify,
		FilePath: "../../../etc/passwd",
	}

	result, err := HandleCodeTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error for path traversal attempt")
	}
	if !stringContains(result.Error, "path traversal") && !stringContains(result.Error, "invalid file path") {
		t.Errorf("error %q does not mention path traversal", result.Error)
	}
}

// stringContains checks if a string contains a substring (avoids conflict with presenter_test.go)
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
