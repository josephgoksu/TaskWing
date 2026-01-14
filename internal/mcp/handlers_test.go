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
