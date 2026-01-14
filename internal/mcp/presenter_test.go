package mcp

import (
	"testing"

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
