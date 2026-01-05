/*
Package core provides Finding and Evidence types for agent discoveries.
*/
package core

import "time"

// FindingType categorizes what kind of discovery was made.
type FindingType string

const (
	FindingTypeFeature    FindingType = "feature"
	FindingTypeDecision   FindingType = "decision"
	FindingTypeDependency FindingType = "dependency"
	FindingTypePattern    FindingType = "pattern"
	FindingTypeRisk       FindingType = "risk"
	FindingTypeConstraint FindingType = "constraint"
)

// Finding represents a single discovery made by an agent.
type Finding struct {
	Type               FindingType
	Title              string
	Description        string
	Why                string
	Tradeoffs          string
	ConfidenceScore    float64 // 0.0-1.0
	Confidence         string  // Deprecated: use ConfidenceScore
	SourceAgent        string
	SourceFiles        []string // Deprecated: use Evidence
	Evidence           []Evidence
	VerificationStatus VerificationStatus
	VerificationResult *VerificationResult
	Metadata           map[string]any
}

// Evidence represents verifiable proof for a Finding.
type Evidence struct {
	FilePath     string `json:"file_path"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	Snippet      string `json:"snippet"`
	GrepPattern  string `json:"grep_pattern,omitempty"`
	EvidenceType string `json:"evidence_type,omitempty"` // "file" (default) or "git"
}

// VerificationStatus tracks the validation state of a finding.
type VerificationStatus string

const (
	VerificationStatusPending  VerificationStatus = "pending_verification"
	VerificationStatusVerified VerificationStatus = "verified"
	VerificationStatusPartial  VerificationStatus = "partial"
	VerificationStatusRejected VerificationStatus = "rejected"
	VerificationStatusSkipped  VerificationStatus = "skipped"
)

// VerificationResult captures the outcome of verifying a Finding.
type VerificationResult struct {
	Status               VerificationStatus    `json:"status"`
	EvidenceResults      []EvidenceCheckResult `json:"evidence_results"`
	ConfidenceAdjustment float64               `json:"confidence_adjustment"`
	VerifiedAt           time.Time             `json:"verified_at"`
	VerifierVersion      string                `json:"verifier_version"`
}

// EvidenceCheckResult captures the outcome of checking a single piece of evidence.
type EvidenceCheckResult struct {
	EvidenceIndex    int     `json:"evidence_index"`
	FileExists       bool    `json:"file_exists"`
	SnippetFound     bool    `json:"snippet_found"`
	LineNumbersMatch bool    `json:"line_numbers_match"`
	SimilarityScore  float64 `json:"similarity_score"`
	ActualContent    string  `json:"actual_content,omitempty"`
	ErrorMessage     string  `json:"error_message,omitempty"`
}

// AggregateFindings combines findings from all agent outputs.
func AggregateFindings(outputs []Output) []Finding {
	var all []Finding
	for _, out := range outputs {
		for _, f := range out.Findings {
			f.SourceAgent = out.AgentName
			all = append(all, f)
		}
	}
	return all
}

// AggregateRelationships combines relationships from all agent outputs.
func AggregateRelationships(outputs []Output) []Relationship {
	var all []Relationship
	for _, out := range outputs {
		all = append(all, out.Relationships...)
	}
	return all
}

// ConfidenceLabelFromScore converts numeric confidence to label.
func ConfidenceLabelFromScore(score float64) string {
	switch {
	case score >= 0.8:
		return "high"
	case score >= 0.5:
		return "medium"
	default:
		return "low"
	}
}

// ConfidenceScoreFromLabel converts label to numeric confidence.
func ConfidenceScoreFromLabel(label string) float64 {
	switch label {
	case "high":
		return 0.9
	case "medium":
		return 0.7
	case "low":
		return 0.4
	default:
		return 0.5
	}
}
