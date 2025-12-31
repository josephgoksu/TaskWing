// Package knowledge provides response types shared between CLI and MCP.
package knowledge

import (
	"encoding/json"
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/memory"
)

// -----------------------------------------------------------------------------
// Shared Response Types — Used by both CLI and MCP for consistency
// -----------------------------------------------------------------------------

// EvidenceRef is a compact file:line reference for AI to cite
type EvidenceRef struct {
	File  string `json:"file"`            // e.g., "backend-go/internal/auth/jwt.go"
	Lines string `json:"lines,omitempty"` // e.g., "45-67"
}

// NodeResponse is a token-efficient view of a Node (embeddings stripped).
// This is the canonical format for context responses across CLI and MCP.
type NodeResponse struct {
	ID                 string        `json:"id"`
	Type               string        `json:"type,omitempty"`
	Summary            string        `json:"summary,omitempty"`
	Content            string        `json:"content"`
	ConfidenceScore    float64       `json:"confidenceScore,omitempty"`
	VerificationStatus string        `json:"verificationStatus,omitempty"`
	MatchScore         float32       `json:"matchScore,omitempty"` // Semantic similarity (0-1)
	Evidence           []EvidenceRef `json:"evidence,omitempty"`   // File:line references
}

// NodeToResponse converts a memory.Node to a token-efficient NodeResponse.
func NodeToResponse(n memory.Node, matchScore float32) NodeResponse {
	return NodeResponse{
		ID:                 n.ID,
		Type:               n.Type,
		Summary:            n.Summary,
		Content:            n.Content,
		ConfidenceScore:    n.ConfidenceScore,
		VerificationStatus: n.VerificationStatus,
		MatchScore:         matchScore,
		Evidence:           parseEvidence(n.Evidence),
	}
}

// ScoredNodeToResponse converts a ScoredNode to NodeResponse.
func ScoredNodeToResponse(sn ScoredNode) NodeResponse {
	if sn.Node == nil {
		return NodeResponse{}
	}
	return NodeToResponse(*sn.Node, sn.Score)
}

// parseEvidence parses the JSON evidence field from a Node
func parseEvidence(evidenceJSON string) []EvidenceRef {
	if evidenceJSON == "" {
		return nil
	}
	var rawEvidence []struct {
		FilePath  string `json:"file_path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
	}
	if err := json.Unmarshal([]byte(evidenceJSON), &rawEvidence); err != nil {
		return nil
	}
	var refs []EvidenceRef
	for _, e := range rawEvidence {
		lines := ""
		if e.StartLine > 0 {
			if e.EndLine > e.StartLine {
				lines = fmt.Sprintf("%d-%d", e.StartLine, e.EndLine)
			} else {
				lines = fmt.Sprintf("%d", e.StartLine)
			}
		}
		refs = append(refs, EvidenceRef{File: e.FilePath, Lines: lines})
	}
	return refs
}

// TypeSummary provides an overview of nodes of a specific type.
type TypeSummary struct {
	Count    int      `json:"count"`
	Examples []string `json:"examples"` // Top 3 summaries
}

// ProjectSummary provides a high-level overview of the project memory.
type ProjectSummary struct {
	Total int                    `json:"total"`
	Types map[string]TypeSummary `json:"types"`
}
