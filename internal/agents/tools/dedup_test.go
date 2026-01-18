package tools

import (
	"testing"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
)

func TestNewFindingDeduplicator(t *testing.T) {
	d := NewFindingDeduplicator()
	if d == nil {
		t.Fatal("NewFindingDeduplicator returned nil")
	}
	if d.similarityThreshold != 0.6 {
		t.Errorf("Default threshold = %v, want 0.6", d.similarityThreshold)
	}
}

func TestFindingDeduplicator_SetSimilarityThreshold(t *testing.T) {
	d := NewFindingDeduplicator()

	// Valid threshold
	d.SetSimilarityThreshold(0.8)
	if d.similarityThreshold != 0.8 {
		t.Errorf("Threshold = %v, want 0.8", d.similarityThreshold)
	}

	// Invalid thresholds should be ignored
	d.SetSimilarityThreshold(0)
	if d.similarityThreshold != 0.8 {
		t.Error("Zero threshold should be ignored")
	}

	d.SetSimilarityThreshold(1.5)
	if d.similarityThreshold != 0.8 {
		t.Error("Threshold > 1.0 should be ignored")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		a, b     string
		expected float64
	}{
		{"hello world", "hello world", 1.0},
		{"", "", 1.0},
		{"hello", "", 0.0},
		// After stop word removal: "hello world" vs "hello there" = {hello,world} vs {hello,there}
		// Intersection=1 (hello), Union=3 (hello,world,there) -> 1/3 = 0.33
		{"hello world", "hello there", 0.33},
		// After stop word removal: "quick brown fox" vs "lazy brown dog"
		// Intersection=1 (brown), Union=5 -> 1/5 = 0.2
		{"the quick brown fox", "the lazy brown dog", 0.2},
	}

	for _, tc := range tests {
		got := jaccardSimilarity(tc.a, tc.b)
		// Allow some floating point tolerance
		if got < tc.expected-0.1 || got > tc.expected+0.1 {
			t.Errorf("jaccardSimilarity(%q, %q) = %v, want ~%v", tc.a, tc.b, got, tc.expected)
		}
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"hello-world", []string{"hello", "world"}},
		{"hello_world", []string{"hello", "world"}},
		{"the quick brown fox", []string{"quick", "brown", "fox"}}, // "the" is stop word
		{"", nil},
		{"a b c", nil}, // All too short
	}

	for _, tc := range tests {
		got := tokenize(tc.input)
		if len(got) != len(tc.expected) {
			t.Errorf("tokenize(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestDeduplicateFindings_Empty(t *testing.T) {
	d := NewFindingDeduplicator()
	result := d.DeduplicateFindings(nil)
	if result != nil {
		t.Error("Expected nil for empty input")
	}
}

func TestDeduplicateFindings_NoDuplicates(t *testing.T) {
	d := NewFindingDeduplicator()

	findings := []core.Finding{
		{Type: core.FindingTypeDecision, Title: "Use PostgreSQL", Description: "Database choice"},
		{Type: core.FindingTypeDecision, Title: "Use Redis", Description: "Caching choice"},
		{Type: core.FindingTypePattern, Title: "Repository Pattern", Description: "Data access"},
	}

	result := d.DeduplicateFindings(findings)
	if len(result) != 3 {
		t.Errorf("Expected 3 findings, got %d", len(result))
	}
}

func TestDeduplicateFindings_ExactDuplicates(t *testing.T) {
	d := NewFindingDeduplicator()

	findings := []core.Finding{
		{Type: core.FindingTypeDecision, Title: "Use PostgreSQL", Description: "Database choice", ConfidenceScore: 0.9},
		{Type: core.FindingTypeDecision, Title: "Use PostgreSQL", Description: "Database choice", ConfidenceScore: 0.8},
	}

	result := d.DeduplicateFindings(findings)
	if len(result) != 1 {
		t.Errorf("Expected 1 finding after dedup, got %d", len(result))
	}

	// Should keep the higher confidence one
	if result[0].ConfidenceScore != 0.9 {
		t.Errorf("Expected to keep finding with higher confidence (0.9), got %v", result[0].ConfidenceScore)
	}
}

func TestDeduplicateFindings_SimilarTitles(t *testing.T) {
	d := NewFindingDeduplicator()
	d.SetSimilarityThreshold(0.5) // Lower threshold for this test

	// These titles share significant words after tokenization
	findings := []core.Finding{
		{Type: core.FindingTypeDecision, Title: "PostgreSQL database storage decision", Description: "Primary storage choice"},
		{Type: core.FindingTypeDecision, Title: "PostgreSQL database storage choice", Description: "Data storage selection"},
	}

	result := d.DeduplicateFindings(findings)
	if len(result) != 1 {
		t.Errorf("Expected 1 finding after dedup of similar titles, got %d", len(result))
	}
}

func TestDeduplicateFindings_DifferentTypes(t *testing.T) {
	d := NewFindingDeduplicator()

	// Same title but different types should NOT be deduplicated
	findings := []core.Finding{
		{Type: core.FindingTypeDecision, Title: "Repository Pattern", Description: "Decision to use"},
		{Type: core.FindingTypePattern, Title: "Repository Pattern", Description: "Pattern implementation"},
	}

	result := d.DeduplicateFindings(findings)
	if len(result) != 2 {
		t.Errorf("Different types should not be deduplicated, got %d findings", len(result))
	}
}

func TestDeduplicateRelationships(t *testing.T) {
	d := NewFindingDeduplicator()

	rels := []core.Relationship{
		{From: "A", To: "B", Relation: "depends_on"},
		{From: "A", To: "B", Relation: "depends_on"}, // duplicate
		{From: "B", To: "C", Relation: "extends"},
		{From: "A", To: "B", Relation: "extends"}, // same from/to but different relation
	}

	result := d.DeduplicateRelationships(rels)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique relationships, got %d", len(result))
	}
}

func TestDeduplicateRelationships_Empty(t *testing.T) {
	d := NewFindingDeduplicator()
	result := d.DeduplicateRelationships(nil)
	if result != nil {
		t.Error("Expected nil for empty input")
	}
}

func TestGetConfidence_Float(t *testing.T) {
	d := NewFindingDeduplicator()

	finding := core.Finding{ConfidenceScore: 0.85}
	got := d.getConfidence(finding)
	if got != 0.85 {
		t.Errorf("getConfidence = %v, want 0.85", got)
	}
}

func TestGetConfidence_String(t *testing.T) {
	d := NewFindingDeduplicator()

	tests := []struct {
		confidence string
		expected   float64
	}{
		{"high", 0.9},
		{"HIGH", 0.9},
		{"medium", 0.6},
		{"low", 0.3},
		{"unknown", 0.5},
	}

	for _, tc := range tests {
		finding := core.Finding{Confidence: tc.confidence}
		got := d.getConfidence(finding)
		if got != tc.expected {
			t.Errorf("getConfidence(%q) = %v, want %v", tc.confidence, got, tc.expected)
		}
	}
}
