package mcp

import (
	"context"
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
		action         CodeAction
		name           string
		expectError    bool
		errorContains  string
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

func TestHandleTaskTool_ActionRouting(t *testing.T) {
	// Test actions that have validation before hitting the repo
	tests := []struct {
		action      TaskAction
		name        string
		expectError bool
	}{
		{TaskActionStart, "start", true},    // missing task_id
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
