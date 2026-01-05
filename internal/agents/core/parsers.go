/*
Package core provides shared parsing utilities for all agents.
*/
package core

import (
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// EvidenceJSON is the structured evidence format from LLM responses.
type EvidenceJSON struct {
	FilePath     string `json:"file_path"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	Snippet      string `json:"snippet"`
	GrepPattern  string `json:"grep_pattern,omitempty"`
	EvidenceType string `json:"evidence_type,omitempty"` // "file" (default) or "git"
}

// ParseJSONResponse extracts JSON from LLM response and unmarshals it.
func ParseJSONResponse[T any](response string) (T, error) {
	return utils.ExtractAndParseJSON[T](response)
}

// ConvertEvidence transforms JSON evidence to Evidence structs.
func ConvertEvidence(jsonEvidence []EvidenceJSON) []Evidence {
	if len(jsonEvidence) == 0 {
		return nil
	}
	evidence := make([]Evidence, len(jsonEvidence))
	for i, e := range jsonEvidence {
		evidence[i] = Evidence(e)
	}
	return evidence
}

// ParseConfidence handles both numeric and string confidence values.
func ParseConfidence(raw any) (float64, string) {
	switch v := raw.(type) {
	case float64:
		return v, ConfidenceLabelFromScore(v)
	case int:
		score := float64(v)
		return score, ConfidenceLabelFromScore(score)
	case string:
		return ConfidenceScoreFromLabel(v), v
	default:
		return 0.5, "medium"
	}
}

// NewFindingWithEvidence creates a Finding with proper defaults.
func NewFindingWithEvidence(
	findingType FindingType,
	title, description, why, tradeoffs string,
	confidence any,
	evidence []EvidenceJSON,
	sourceAgent string,
	metadata map[string]any,
) Finding {
	confidenceScore, confidenceLabel := ParseConfidence(confidence)
	return Finding{
		Type:               findingType,
		Title:              title,
		Description:        description,
		Why:                why,
		Tradeoffs:          tradeoffs,
		ConfidenceScore:    confidenceScore,
		Confidence:         confidenceLabel,
		Evidence:           ConvertEvidence(evidence),
		VerificationStatus: VerificationStatusPending,
		SourceAgent:        sourceAgent,
		Metadata:           metadata,
	}
}
