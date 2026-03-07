package core

import (
	"testing"
)

// =============================================================================
// TestParseJSONResponse_Hallucination: Reject or flag hallucinated outputs
// =============================================================================

func TestParseJSONResponse_Hallucination(t *testing.T) {
	type simpleResponse struct {
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Evidence    []EvidenceJSON `json:"evidence"`
	}

	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantTitle string
	}{
		{
			name:    "empty string rejected",
			input:   "",
			wantErr: true,
		},
		{
			name:    "plain text without JSON rejected",
			input:   "This is just some text with no JSON at all.",
			wantErr: true,
		},
		{
			name:    "valid JSON accepted",
			input:   `{"title": "Valid Finding", "description": "desc", "evidence": [{"file_path": "main.go"}]}`,
			wantErr: false,
			wantTitle: "Valid Finding",
		},
		{
			name:    "JSON with markdown fences accepted",
			input:   "```json\n{\"title\": \"Fenced\", \"description\": \"d\"}\n```",
			wantErr: false,
			wantTitle: "Fenced",
		},
		{
			name:      "truncated JSON repaired when simple",
			input:     `{"title": "Truncated", "description": "cut off here`,
			wantErr:   false, // Simple truncation is repairable (closing quote + brace)
			wantTitle: "Truncated",
		},
		{
			name:    "JSON with trailing comma repaired",
			input:   `{"title": "Trailing", "description": "desc",}`,
			wantErr: false,
			wantTitle: "Trailing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseJSONResponse[simpleResponse](tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantTitle != "" && result.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", result.Title, tt.wantTitle)
			}
		})
	}
}

// =============================================================================
// TestGate3_Enforcement: Findings without evidence are flagged
// =============================================================================

func TestGate3_Enforcement(t *testing.T) {
	t.Run("finding_with_evidence_is_pending", func(t *testing.T) {
		f := NewFindingWithEvidence(
			FindingTypeDecision,
			"Has Evidence", "desc", "why", "tradeoffs",
			0.8,
			[]EvidenceJSON{{FilePath: "main.go", StartLine: 1, EndLine: 5, Snippet: "func main()"}},
			"test-agent",
			nil,
		)

		if f.VerificationStatus != VerificationStatusPending {
			t.Errorf("VerificationStatus = %q, want %q", f.VerificationStatus, VerificationStatusPending)
		}
		if !f.HasEvidence() {
			t.Error("HasEvidence() = false, want true")
		}
		if f.NeedsHumanVerification() {
			t.Error("NeedsHumanVerification() = true for finding with evidence and high confidence")
		}
	})

	t.Run("finding_without_evidence_is_skipped", func(t *testing.T) {
		f := NewFindingWithEvidence(
			FindingTypeDecision,
			"No Evidence", "desc", "why", "tradeoffs",
			0.9,
			nil, // No evidence
			"test-agent",
			nil,
		)

		if f.VerificationStatus != VerificationStatusSkipped {
			t.Errorf("VerificationStatus = %q, want %q (Gate 3: no evidence)", f.VerificationStatus, VerificationStatusSkipped)
		}
		if f.HasEvidence() {
			t.Error("HasEvidence() = true, want false")
		}
	})

	t.Run("finding_with_empty_evidence_is_skipped", func(t *testing.T) {
		f := NewFindingWithEvidence(
			FindingTypeDecision,
			"Empty Evidence", "desc", "why", "tradeoffs",
			0.8,
			[]EvidenceJSON{}, // Empty slice
			"test-agent",
			nil,
		)

		if f.VerificationStatus != VerificationStatusSkipped {
			t.Errorf("VerificationStatus = %q, want %q (Gate 3: empty evidence)", f.VerificationStatus, VerificationStatusSkipped)
		}
	})

	t.Run("finding_with_low_confidence_needs_verification", func(t *testing.T) {
		f := NewFindingWithEvidence(
			FindingTypeDecision,
			"Low Confidence", "desc", "why", "tradeoffs",
			0.3, // Below 0.5 threshold
			[]EvidenceJSON{{FilePath: "main.go"}},
			"test-agent",
			nil,
		)

		if !f.NeedsHumanVerification() {
			t.Error("NeedsHumanVerification() = false for low-confidence finding, want true")
		}
	})

	t.Run("finding_with_empty_file_path_has_no_evidence", func(t *testing.T) {
		f := NewFindingWithEvidence(
			FindingTypeDecision,
			"Bad Evidence", "desc", "why", "tradeoffs",
			0.8,
			[]EvidenceJSON{{FilePath: "", Snippet: "some code"}}, // Empty file path
			"test-agent",
			nil,
		)

		if f.HasEvidence() {
			t.Error("HasEvidence() = true for evidence with empty FilePath, want false")
		}
	})

	t.Run("debt_finding_with_evidence_not_flagged", func(t *testing.T) {
		f := NewFindingWithDebt(
			FindingTypeDecision,
			"Debt Finding", "desc", "why", "tradeoffs",
			0.8,
			[]EvidenceJSON{{FilePath: "legacy.go", StartLine: 10}},
			"test-agent",
			nil,
			DebtInfo{DebtScore: 0.8, DebtReason: "legacy pattern"},
		)

		if f.VerificationStatus != VerificationStatusPending {
			t.Errorf("VerificationStatus = %q, want %q", f.VerificationStatus, VerificationStatusPending)
		}
		if !f.IsDebt() {
			t.Error("IsDebt() = false, want true")
		}
	})
}

// =============================================================================
// TestParseConfidence: Confidence parsing edge cases
// =============================================================================

func TestParseConfidence(t *testing.T) {
	tests := []struct {
		name      string
		input     any
		wantScore float64
		wantLabel string
	}{
		{"high float", 0.9, 0.9, "high"},
		{"medium float", 0.6, 0.6, "medium"},
		{"low float", 0.3, 0.3, "low"},
		{"integer 1", 1, 1.0, "high"},
		{"integer 0", 0, 0.0, "low"},
		{"string high", "high", 0.9, "high"},
		{"string medium", "medium", 0.7, "medium"},
		{"string low", "low", 0.4, "low"},
		{"nil defaults to medium", nil, 0.5, "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, label := ParseConfidence(tt.input)
			if score != tt.wantScore {
				t.Errorf("score = %f, want %f", score, tt.wantScore)
			}
			if label != tt.wantLabel {
				t.Errorf("label = %q, want %q", label, tt.wantLabel)
			}
		})
	}
}
