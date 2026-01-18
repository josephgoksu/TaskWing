package mcp

import (
	"testing"

	agentcore "github.com/josephgoksu/TaskWing/internal/agents/core"
	agentimpl "github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
)

func TestFormatRecall_NilResult(t *testing.T) {
	result := FormatRecall(nil)
	if result != "No results found." {
		t.Errorf("expected 'No results found.', got %q", result)
	}
}

func TestFormatRecall_EmptyResult(t *testing.T) {
	result := FormatRecall(&app.RecallResult{})
	if result != "No results found." {
		t.Errorf("expected 'No results found.', got %q", result)
	}
}

func TestFormatRecall_WithAnswer(t *testing.T) {
	result := FormatRecall(&app.RecallResult{
		Answer: "This is the answer.",
	})
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !contains(result, "## Answer") {
		t.Error("expected Answer section header")
	}
	if !contains(result, "This is the answer.") {
		t.Error("expected answer content")
	}
}

func TestFormatSymbolList_Empty(t *testing.T) {
	result := FormatSymbolList(nil)
	if result != "No symbols found." {
		t.Errorf("expected 'No symbols found.', got %q", result)
	}
}

func TestFormatSymbolList_WithSymbols(t *testing.T) {
	symbols := []codeintel.Symbol{
		{Name: "TestFunc", Kind: "function", FilePath: "test.go", StartLine: 10},
		{Name: "TestStruct", Kind: "struct", FilePath: "test.go", StartLine: 20},
	}
	result := FormatSymbolList(symbols)
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !contains(result, "TestFunc") {
		t.Error("expected TestFunc in result")
	}
	if !contains(result, "function") {
		t.Error("expected 'function' kind in result")
	}
}

func TestFormatSearchResults_Empty(t *testing.T) {
	result := FormatSearchResults(nil)
	if result != "No matching symbols found." {
		t.Errorf("expected 'No matching symbols found.', got %q", result)
	}
}

func TestFormatSearchResults_WithResults(t *testing.T) {
	results := []codeintel.SymbolSearchResult{
		{
			Symbol: codeintel.Symbol{Name: "SearchFunc", Kind: "function", FilePath: "search.go", StartLine: 5},
			Score:  0.95,
		},
	}
	result := FormatSearchResults(results)
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !contains(result, "SearchFunc") {
		t.Error("expected SearchFunc in result")
	}
	if !contains(result, "Search Results") {
		t.Error("expected header in result")
	}
}

func TestFormatTask_NilResult(t *testing.T) {
	result := FormatTask(nil)
	if result != "No task information." {
		t.Errorf("expected 'No task information.', got %q", result)
	}
}

func TestFormatError(t *testing.T) {
	result := FormatError("something went wrong")
	if !contains(result, "Error") {
		t.Error("expected Error header")
	}
	if !contains(result, "something went wrong") {
		t.Error("expected error message")
	}
}

func TestFormatValidationError(t *testing.T) {
	result := FormatValidationError("query", "query is required")
	if !contains(result, "Validation Error") {
		t.Error("expected Validation Error header")
	}
	if !contains(result, "query") {
		t.Error("expected field name")
	}
	if !contains(result, "query is required") {
		t.Error("expected error message")
	}
}

func TestFormatCallers_NilResult(t *testing.T) {
	result := FormatCallers(nil)
	if result == "" {
		t.Error("expected non-empty result for nil input")
	}
}

func TestFormatImpact_NilResult(t *testing.T) {
	result := FormatImpact(nil)
	if !contains(result, "Failed") {
		t.Error("expected failure message for nil input")
	}
}

func TestFormatExplainResult_NilResult(t *testing.T) {
	result := FormatExplainResult(nil)
	if result != "No explanation available." {
		t.Errorf("expected 'No explanation available.', got %q", result)
	}
}

func TestFormatDebugResult_Empty(t *testing.T) {
	result := FormatDebugResult(nil)
	if result != "No debug analysis available." {
		t.Errorf("expected 'No debug analysis available.', got %q", result)
	}
}

func TestFormatDebugResult_WithJSONStyleData(t *testing.T) {
	// Simulate data as it would come from JSON deserialization
	// (slices become []interface{}, maps become map[string]interface{})
	findings := []agentcore.Finding{
		{
			Type:        "debug",
			Description: "Database connection timeout",
			Metadata: map[string]any{
				"hypotheses": []interface{}{
					map[string]interface{}{
						"cause":          "Connection pool exhausted",
						"likelihood":     "high",
						"reasoning":      "Many concurrent requests",
						"code_locations": []interface{}{"db/pool.go:45"},
					},
					map[string]interface{}{
						"cause":      "Network latency",
						"likelihood": "medium",
						"reasoning":  "Remote database",
					},
				},
				"investigation_steps": []interface{}{
					map[string]interface{}{
						"step":             float64(1), // JSON numbers are float64
						"action":           "Check connection pool",
						"command":          "netstat -an | grep 5432",
						"expected_finding": "Many ESTABLISHED connections",
					},
				},
				"quick_fixes": []interface{}{
					map[string]interface{}{
						"fix":  "Increase pool size",
						"when": "Pool is exhausted",
					},
				},
			},
		},
	}

	result := FormatDebugResult(findings)

	if result == "No debug analysis available." {
		t.Error("expected non-empty result")
	}
	if !contains(result, "Debug Analysis") {
		t.Error("expected Debug Analysis header")
	}
	if !contains(result, "Connection pool exhausted") {
		t.Error("expected hypothesis cause")
	}
	if !contains(result, "high") {
		t.Error("expected likelihood")
	}
	if !contains(result, "Check connection pool") {
		t.Error("expected investigation step")
	}
	if !contains(result, "Increase pool size") {
		t.Error("expected quick fix")
	}
}

func TestFormatSimplifyResult_Empty(t *testing.T) {
	result := FormatSimplifyResult(nil)
	if result != "No simplification results." {
		t.Errorf("expected 'No simplification results.', got %q", result)
	}
}

func TestFormatSimplifyResult_WithJSONStyleData(t *testing.T) {
	// Simulate data as it would come from JSON deserialization
	findings := []agentcore.Finding{
		{
			Type:        "simplification",
			Description: "Simplified error handling",
			Metadata: map[string]any{
				"simplified_code":      "return err",
				"original_lines":      float64(10), // JSON numbers are float64
				"simplified_lines":    float64(3),
				"reduction_percentage": float64(70),
				"risk_assessment":      "low",
				"changes": []interface{}{
					map[string]interface{}{
						"what": "Removed redundant nil check",
						"why":  "Error is always non-nil in this branch",
						"risk": "none",
					},
					map[string]interface{}{
						"what": "Consolidated error wrapping",
						"why":  "Multiple wrap calls were redundant",
						"risk": "low",
					},
				},
			},
		},
	}

	result := FormatSimplifyResult(findings)

	if result == "No simplification results." {
		t.Error("expected non-empty result")
	}
	if !contains(result, "Code Simplification") {
		t.Error("expected Code Simplification header")
	}
	if !contains(result, "return err") {
		t.Error("expected simplified code")
	}
	if !contains(result, "10") && !contains(result, "3") {
		t.Error("expected line counts")
	}
	if !contains(result, "Removed redundant nil check") {
		t.Error("expected change description")
	}
	if !contains(result, "low") {
		t.Error("expected risk assessment")
	}
}

func TestExtractSimplifyChanges_DirectType(t *testing.T) {
	// Test with direct type (from agent before serialization)
	direct := []agentimpl.SimplifyChange{
		{What: "removed", Why: "unused", Risk: "none"},
	}
	result := extractSimplifyChanges(direct)
	if len(result) != 1 {
		t.Errorf("expected 1 change, got %d", len(result))
	}
	if result[0].What != "removed" {
		t.Errorf("expected 'removed', got %q", result[0].What)
	}
}

func TestExtractDebugHypotheses_DirectType(t *testing.T) {
	// Test with direct type (from agent before serialization)
	direct := []agentimpl.DebugHypothesis{
		{Cause: "bug", Likelihood: "high", Reasoning: "test"},
	}
	result := extractDebugHypotheses(direct)
	if len(result) != 1 {
		t.Errorf("expected 1 hypothesis, got %d", len(result))
	}
	if result[0].Cause != "bug" {
		t.Errorf("expected 'bug', got %q", result[0].Cause)
	}
}

func TestGetIntFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		key      string
		want     int
	}{
		{"nil map", nil, "x", 0},
		{"missing key", map[string]any{}, "x", 0},
		{"float64 value", map[string]any{"x": float64(42)}, "x", 42},
		{"int value", map[string]any{"x": 42}, "x", 42},
		{"string value", map[string]any{"x": "42"}, "x", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getIntFromMetadata(tt.metadata, tt.key)
			if got != tt.want {
				t.Errorf("getIntFromMetadata() = %d, want %d", got, tt.want)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
