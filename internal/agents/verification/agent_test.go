/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package verification

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
)

func TestVerificationAgent_VerifyFindings(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	agent := NewAgent(tmpDir)

	tests := []struct {
		name           string
		finding        core.Finding
		expectedStatus core.VerificationStatus
	}{
		{
			name: "verified - file exists and snippet matches",
			finding: core.Finding{
				Title:           "Test Finding",
				ConfidenceScore: 0.8,
				Evidence: []core.Evidence{
					{
						FilePath:  "test.go",
						StartLine: 5,
						EndLine:   7,
						Snippet:   "func main() {\n\tfmt.Println(\"Hello, World!\")\n}",
					},
				},
			},
			expectedStatus: core.VerificationStatusVerified,
		},
		{
			name: "rejected - file does not exist",
			finding: core.Finding{
				Title:           "Missing File Finding",
				ConfidenceScore: 0.8,
				Evidence: []core.Evidence{
					{
						FilePath: "nonexistent.go",
						Snippet:  "some code",
					},
				},
			},
			expectedStatus: core.VerificationStatusRejected,
		},
		{
			name: "verified - snippet found anywhere in file",
			finding: core.Finding{
				Title:           "Snippet Found Finding",
				ConfidenceScore: 0.7,
				Evidence: []core.Evidence{
					{
						FilePath: "test.go",
						Snippet:  "fmt.Println",
					},
				},
			},
			expectedStatus: core.VerificationStatusVerified,
		},
		{
			name: "skipped - no evidence",
			finding: core.Finding{
				Title:           "No Evidence Finding",
				ConfidenceScore: 0.6,
				Evidence:        []core.Evidence{},
			},
			expectedStatus: core.VerificationStatusSkipped,
		},
		{
			name: "partial - file exists but wrong line numbers",
			finding: core.Finding{
				Title:           "Wrong Lines Finding",
				ConfidenceScore: 0.75,
				Evidence: []core.Evidence{
					{
						FilePath:  "test.go",
						StartLine: 1,
						EndLine:   2,
						Snippet:   "func main() {", // This is actually at line 5
					},
				},
			},
			expectedStatus: core.VerificationStatusPartial, // Snippet exists but not at specified lines
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := []core.Finding{tt.finding}
			result := agent.VerifyFindings(context.Background(), findings)

			if len(result) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(result))
			}

			if result[0].VerificationStatus != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, result[0].VerificationStatus)
				if result[0].VerificationResult != nil {
					for i, ev := range result[0].VerificationResult.EvidenceResults {
						t.Logf("  evidence[%d]: fileExists=%v snippetFound=%v lineMatch=%v similarity=%.2f err=%s",
							i, ev.FileExists, ev.SnippetFound, ev.LineNumbersMatch, ev.SimilarityScore, ev.ErrorMessage)
					}
				}
			}
		})
	}
}

func TestVerificationAgent_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	agent := NewAgent(tmpDir)

	// Try to access a file outside the base path
	finding := core.Finding{
		Title: "Path Traversal Attempt",
		Evidence: []core.Evidence{
			{
				FilePath: "../../../etc/passwd",
				Snippet:  "root:",
			},
		},
	}

	result := agent.VerifyFindings(context.Background(), []core.Finding{finding})

	if result[0].VerificationStatus != core.VerificationStatusRejected {
		t.Errorf("expected rejected status for path traversal, got %s", result[0].VerificationStatus)
	}

	if result[0].VerificationResult != nil && len(result[0].VerificationResult.EvidenceResults) > 0 {
		if result[0].VerificationResult.EvidenceResults[0].ErrorMessage != "path traversal detected" {
			t.Errorf("expected path traversal error, got: %s", result[0].VerificationResult.EvidenceResults[0].ErrorMessage)
		}
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "  hello   world  ",
			expected: "hello world",
		},
		{
			input:    "line1\n  line2  \n\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			input:    "\t\tfunc main() {\n\t\t\treturn\n\t\t}",
			expected: "func main() {\nreturn\n}",
		},
	}

	for _, tt := range tests {
		result := normalizeWhitespace(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeWhitespace(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCalculateSimilarity(t *testing.T) {
	tests := []struct {
		a, b        string
		minExpected float64
	}{
		{"hello world", "hello world", 1.0},
		{"hello world", "hello there", 0.3},
		{"", "hello", 0.0},
		{"completely different", "nothing similar", 0.0},
		{"func main() { return }", "func main() { return nil }", 0.7},
	}

	for _, tt := range tests {
		result := calculateSimilarity(tt.a, tt.b)
		if result < tt.minExpected {
			t.Errorf("calculateSimilarity(%q, %q) = %.2f, want >= %.2f", tt.a, tt.b, result, tt.minExpected)
		}
	}
}

func TestFilterVerifiedFindings(t *testing.T) {
	findings := []core.Finding{
		{Title: "Verified", VerificationStatus: core.VerificationStatusVerified},
		{Title: "Rejected", VerificationStatus: core.VerificationStatusRejected},
		{Title: "Partial", VerificationStatus: core.VerificationStatusPartial},
		{Title: "Pending", VerificationStatus: core.VerificationStatusPending},
		{Title: "Skipped", VerificationStatus: core.VerificationStatusSkipped},
	}

	result := FilterVerifiedFindings(findings)

	if len(result) != 4 {
		t.Errorf("expected 4 findings (all except rejected), got %d", len(result))
	}

	for _, f := range result {
		if f.VerificationStatus == core.VerificationStatusRejected {
			t.Errorf("rejected finding should have been filtered out: %s", f.Title)
		}
	}
}

func TestFilterByMinConfidence(t *testing.T) {
	findings := []core.Finding{
		{Title: "High", ConfidenceScore: 0.9},
		{Title: "Medium", ConfidenceScore: 0.7},
		{Title: "Low", ConfidenceScore: 0.4},
	}

	result := FilterByMinConfidence(findings, 0.6)

	if len(result) != 2 {
		t.Errorf("expected 2 findings with confidence >= 0.6, got %d", len(result))
	}
}

func TestExtractLines(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"

	tests := []struct {
		start, end int
		expected   string
	}{
		{1, 1, "line1"},
		{2, 4, "line2\nline3\nline4"},
		{5, 5, "line5"},
		{3, 0, "line3"},        // end=0 defaults to start
		{0, 2, "line1\nline2"}, // start=0 defaults to 1
	}

	for _, tt := range tests {
		result := extractLines(content, tt.start, tt.end)
		if result != tt.expected {
			t.Errorf("extractLines(%d, %d) = %q, want %q", tt.start, tt.end, result, tt.expected)
		}
	}
}
