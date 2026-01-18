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

	// Debt Classification - distinguishes essential from accidental complexity
	// When DebtScore >= 0.7, this pattern represents technical debt that shouldn't be propagated
	DebtScore    float64 `json:"debtScore,omitempty"`    // 0.0 = clean, 1.0 = pure debt
	DebtReason   string  `json:"debtReason,omitempty"`   // Why this is considered debt
	RefactorHint string  `json:"refactorHint,omitempty"` // How to eliminate the debt
	DebtWarning  string  `json:"debtWarning,omitempty"`  // Human/AI-readable warning (auto-generated)
}

// NodeToResponse converts a memory.Node to a token-efficient NodeResponse.
func NodeToResponse(n memory.Node, matchScore float32) NodeResponse {
	resp := NodeResponse{
		ID:                 n.ID,
		Type:               n.Type,
		Summary:            n.Summary,
		Content:            n.Content,
		ConfidenceScore:    n.ConfidenceScore,
		VerificationStatus: n.VerificationStatus,
		MatchScore:         matchScore,
		Evidence:           parseEvidence(n.Evidence),
		DebtScore:          n.DebtScore,
		DebtReason:         n.DebtReason,
		RefactorHint:       n.RefactorHint,
	}

	// Generate debt warning for AI consumption
	resp.DebtWarning = n.DebtWarning()

	return resp
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

	seen := make(map[string]bool)
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

		// Deduplicate based on file and lines
		key := fmt.Sprintf("%s|%s", e.FilePath, lines)
		if seen[key] {
			continue
		}
		seen[key] = true

		refs = append(refs, EvidenceRef{File: e.FilePath, Lines: lines})
	}
	return refs
}

// TypeSummary provides an overview of nodes of a specific type.
type TypeSummary struct {
	Count    int      `json:"count"`
	Examples []string `json:"examples"` // Top 3 summaries
}

// ProjectOverviewInfo is a compact version of the project overview for API responses.
type ProjectOverviewInfo struct {
	ShortDescription string `json:"short_description"`
	LongDescription  string `json:"long_description,omitempty"`
}

// ProjectSummary provides a high-level overview of the project memory.
type ProjectSummary struct {
	Overview *ProjectOverviewInfo   `json:"overview,omitempty"` // High-level project description
	Total    int                    `json:"total"`
	Types    map[string]TypeSummary `json:"types"`
}

// -----------------------------------------------------------------------------
// Debug Retrieval Types — Raw search results for inspection and debugging
// -----------------------------------------------------------------------------

// DebugRetrievalResult represents a single search result with full debug information.
// This is used by the `memory inspect` command to show raw retrieval data.
type DebugRetrievalResult struct {
	// Core identification
	ID       string `json:"id"`        // Node/chunk ID (e.g., "n-abc123")
	ChunkID  string `json:"chunk_id"`  // Same as ID for nodes, may differ for sub-chunks
	NodeType string `json:"node_type"` // Type: decision, constraint, pattern, etc.

	// Source information
	SourceFilePath string `json:"source_file_path,omitempty"` // Absolute path to source file
	SourceAgent    string `json:"source_agent,omitempty"`     // Agent that created this (doc-loader, code-agent, etc.)

	// Content
	Summary string `json:"summary"`           // Brief summary/title
	Content string `json:"content,omitempty"` // Full content (truncated in display)

	// Scoring (all normalized to 0-1 range)
	FTSScore        float32 `json:"fts_score"`        // BM25 keyword match score
	VectorScore     float32 `json:"vector_score"`     // Cosine similarity score
	CombinedScore   float32 `json:"combined_score"`   // Weighted combination
	RerankScore     float32 `json:"rerank_score"`     // After reranking (0 if not reranked)
	IsExactMatch    bool    `json:"is_exact_match"`   // True if matched by exact ID
	IsGraphExpanded bool    `json:"is_graph_expanded"` // True if added via graph expansion

	// Evidence references
	Evidence []EvidenceRef `json:"evidence,omitempty"` // File:line references

	// Embedding info (for --verbose mode)
	EmbeddingDimension int `json:"embedding_dimension,omitempty"` // Vector dimension (e.g., 1536)
}

// DebugRetrievalResponse contains the full debug output from a search.
type DebugRetrievalResponse struct {
	Query           string                 `json:"query"`            // Original query
	RewrittenQuery  string                 `json:"rewritten_query"`  // Query after rewriting (if enabled)
	TotalCandidates int                    `json:"total_candidates"` // Total candidates before filtering
	Results         []DebugRetrievalResult `json:"results"`          // Ranked results
	Pipeline        []string               `json:"pipeline"`         // Search stages used (FTS, Vector, Rerank, Graph)
	Timings         map[string]int64       `json:"timings_ms"`       // Time per stage in milliseconds
}
