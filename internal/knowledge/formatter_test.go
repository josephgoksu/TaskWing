package knowledge

import (
	"strings"
	"testing"
)

func TestCompactFormatter_FormatNodes_GroupsByType(t *testing.T) {
	formatter := DefaultCompactFormatter()

	nodes := []NodeResponse{
		{ID: "1", Type: "decision", Summary: "Use SQLite", Content: "SQLite for local storage", MatchScore: 0.9},
		{ID: "2", Type: "pattern", Summary: "Repository Pattern", Content: "Data access layer pattern", MatchScore: 0.85},
		{ID: "3", Type: "decision", Summary: "Use Go", Content: "Golang for CLI", MatchScore: 0.8},
		{ID: "4", Type: "constraint", Summary: "No CGO", Content: "CGO-free for portability", MatchScore: 0.75},
	}

	result := formatter.FormatNodes(nodes)

	// Check that types are grouped
	if !strings.Contains(result, "### 📋 Decisions") {
		t.Error("Expected Decisions header")
	}
	if !strings.Contains(result, "### 🧩 Patterns") {
		t.Error("Expected Patterns header")
	}
	if !strings.Contains(result, "### ⚠️ Constraints") {
		t.Error("Expected Constraints header")
	}

	// Check ordering: Decisions should come before Patterns
	decisionsIdx := strings.Index(result, "Decisions")
	patternsIdx := strings.Index(result, "Patterns")
	constraintsIdx := strings.Index(result, "Constraints")

	if decisionsIdx > patternsIdx {
		t.Error("Decisions should come before Patterns")
	}
	if patternsIdx > constraintsIdx {
		t.Error("Patterns should come before Constraints")
	}
}

func TestCompactFormatter_FormatNodes_NoJSONOrEmbeddings(t *testing.T) {
	formatter := DefaultCompactFormatter()

	nodes := []NodeResponse{
		{
			ID:              "1",
			Type:            "decision",
			Summary:         "Test Decision",
			Content:         "Some content here",
			MatchScore:      0.9,
			ConfidenceScore: 0.95,
			// These fields should NOT appear in output
			Evidence: []EvidenceRef{{File: "test.go", Lines: "10-20"}},
		},
	}

	result := formatter.FormatNodes(nodes)

	// Should not contain JSON-like patterns
	if strings.Contains(result, `"id"`) || strings.Contains(result, `"type"`) {
		t.Error("Output should not contain JSON keys")
	}
	if strings.Contains(result, "0.9") || strings.Contains(result, "0.95") {
		t.Error("Output should not contain raw scores")
	}
	if strings.Contains(result, "[{") || strings.Contains(result, "}]") {
		t.Error("Output should not contain JSON array notation")
	}
}

func TestCompactFormatter_FormatNodes_TokenReduction(t *testing.T) {
	formatter := DefaultCompactFormatter()

	// Create 5 typical nodes (standard test case)
	nodes := []NodeResponse{
		{ID: "n1", Type: "decision", Summary: "Embedded Database", Content: "modernc.org/sqlite - Pure Go implementation of SQLite (CGO-free).", MatchScore: 0.9},
		{ID: "n2", Type: "decision", Summary: "CLI Framework", Content: "Cobra and Viper for command-line interface and configuration.", MatchScore: 0.85},
		{ID: "n3", Type: "pattern", Summary: "Repository Pattern", Content: "Data access abstraction through repository interfaces.", MatchScore: 0.8},
		{ID: "n4", Type: "constraint", Summary: "No External Services", Content: "All storage must be local, no network dependencies.", MatchScore: 0.75},
		{ID: "n5", Type: "feature", Summary: "Semantic Search", Content: "Vector embeddings for natural language queries.", MatchScore: 0.7},
	}

	// Verbose format (simulating JSON-like output)
	var verboseLen int
	for _, n := range nodes {
		// Simulate JSON: {"id":"n1","type":"decision","summary":"...","content":"...","match_score":0.9}
		verbose := `{"id":"` + n.ID + `","type":"` + n.Type + `","summary":"` + n.Summary + `","content":"` + n.Content + `","match_score":` + "0.9" + `}`
		verboseLen += len(verbose)
	}

	// Compact format
	result := formatter.FormatNodes(nodes)
	compactLen := len(result)

	// Compact should be at least 30% smaller than verbose JSON
	reduction := float64(verboseLen-compactLen) / float64(verboseLen) * 100
	if reduction < 30 {
		t.Errorf("Expected at least 30%% token reduction, got %.1f%% (verbose: %d, compact: %d)",
			reduction, verboseLen, compactLen)
	}

	// Log actual reduction for visibility
	t.Logf("Token reduction: %.1f%% (verbose: %d chars, compact: %d chars)", reduction, verboseLen, compactLen)
}

func TestCompactFormatter_FormatWithAnswer(t *testing.T) {
	formatter := DefaultCompactFormatter()

	answer := "The codebase uses SQLite for persistence."
	nodes := []NodeResponse{
		{ID: "1", Type: "decision", Summary: "SQLite", Content: "Local storage", MatchScore: 0.9},
	}
	symbols := []SymbolMatch{
		{Name: "NewSQLiteStore", Kind: "function", Location: "store.go:45"},
	}

	result := formatter.FormatWithAnswer(answer, nodes, symbols)

	// Check sections exist in correct order
	answerIdx := strings.Index(result, "## Answer")
	decisionsIdx := strings.Index(result, "Decisions")
	symbolsIdx := strings.Index(result, "## Symbols")

	if answerIdx == -1 {
		t.Error("Expected Answer section")
	}
	if decisionsIdx == -1 {
		t.Error("Expected Decisions section")
	}
	if symbolsIdx == -1 {
		t.Error("Expected Symbols section")
	}

	// Order: Answer -> Decisions -> Symbols
	if answerIdx > decisionsIdx {
		t.Error("Answer should come before Decisions")
	}
	if decisionsIdx > symbolsIdx {
		t.Error("Decisions should come before Symbols")
	}

	// Check symbol format
	if !strings.Contains(result, "`NewSQLiteStore`") {
		t.Error("Expected backtick-wrapped symbol name")
	}
}

func TestCompactFormatter_MaxNodesPerType(t *testing.T) {
	formatter := &CompactFormatter{
		MaxContentLen:   100,
		MaxNodesPerType: 2, // Limit to 2
	}

	// Create 5 decisions
	nodes := []NodeResponse{
		{ID: "1", Type: "decision", Summary: "D1", MatchScore: 0.9},
		{ID: "2", Type: "decision", Summary: "D2", MatchScore: 0.8},
		{ID: "3", Type: "decision", Summary: "D3", MatchScore: 0.7},
		{ID: "4", Type: "decision", Summary: "D4", MatchScore: 0.6},
		{ID: "5", Type: "decision", Summary: "D5", MatchScore: 0.5},
	}

	result := formatter.FormatNodes(nodes)

	// Should only show top 2 by score
	if !strings.Contains(result, "D1") || !strings.Contains(result, "D2") {
		t.Error("Expected top 2 decisions (D1, D2)")
	}
	if strings.Contains(result, "D3") || strings.Contains(result, "D4") || strings.Contains(result, "D5") {
		t.Error("Should not include decisions beyond MaxNodesPerType limit")
	}
}

func TestCleanContentPreview(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		summary  string
		maxLen   int
		expected string
	}{
		{
			name:     "removes summary prefix",
			content:  "Use SQLite: For local persistence",
			summary:  "Use SQLite",
			maxLen:   100,
			expected: "For local persistence",
		},
		{
			name:     "truncates long content",
			content:  "This is a very long content that should be truncated because it exceeds the maximum length",
			summary:  "Short",
			maxLen:   30,
			expected: "This is a very long content th...",
		},
		{
			name:     "removes newlines",
			content:  "Line one\nLine two\nLine three",
			summary:  "Different",
			maxLen:   100,
			expected: "Line one Line two Line three",
		},
		{
			name:     "returns empty for same content and summary",
			content:  "Same text",
			summary:  "Same text",
			maxLen:   100,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanContentPreview(tt.content, tt.summary, tt.maxLen)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNormalizeType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"decision", "decision"},
		{"decisions", "decision"},
		{"Decision", "decision"},
		{"DECISION", "decision"},
		{"architectural_decision", "decision"},
		{"pattern", "pattern"},
		{"patterns", "pattern"},
		{"constraint", "constraint"},
		{"feature", "feature"},
		{"docs", "documentation"},
		{"doc", "documentation"},
		{"unknown_type", "unknown_type"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTokenEstimate(t *testing.T) {
	// ~4 chars per token heuristic
	text := "This is a test string for token estimation"
	estimate := TokenEstimate(text)

	// 43 chars / 4 = ~10 tokens
	if estimate < 10 || estimate > 12 {
		t.Errorf("TokenEstimate returned %d, expected ~10-11", estimate)
	}
}

func TestFormatNodes_EmptyInput(t *testing.T) {
	formatter := DefaultCompactFormatter()

	result := formatter.FormatNodes(nil)
	if result != "No results found." {
		t.Errorf("Expected 'No results found.', got %q", result)
	}

	result = formatter.FormatNodes([]NodeResponse{})
	if result != "No results found." {
		t.Errorf("Expected 'No results found.', got %q", result)
	}
}

func TestCompactFormatter_ShowEvidence(t *testing.T) {
	formatter := &CompactFormatter{
		MaxContentLen:   100,
		MaxNodesPerType: 5,
		ShowEvidence:    true, // Enable evidence
	}

	nodes := []NodeResponse{
		{
			ID:       "1",
			Type:     "decision",
			Summary:  "Test",
			Content:  "Content",
			Evidence: []EvidenceRef{{File: "main.go", Lines: "10-20"}},
		},
	}

	result := formatter.FormatNodes(nodes)

	if !strings.Contains(result, "main.go:10-20") {
		t.Error("Expected evidence reference when ShowEvidence is true")
	}
}

func TestCompactFormatter_DebtWarning(t *testing.T) {
	formatter := DefaultCompactFormatter()

	nodes := []NodeResponse{
		{
			ID:          "1",
			Type:        "pattern",
			Summary:     "Legacy Pattern",
			Content:     "Old way of doing things",
			DebtWarning: "TECH DEBT: Consider refactoring",
		},
	}

	result := formatter.FormatNodes(nodes)

	if !strings.Contains(result, "⚠️") {
		t.Error("Expected debt warning icon")
	}
	if !strings.Contains(result, "TECH DEBT") {
		t.Error("Expected debt warning text")
	}
}
