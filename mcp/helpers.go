package mcp

// Shared helpers for MCP tools (reference resolution, text formatting)

import (
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/types"
)

// resolveReference tries to resolve a human-provided reference (ID, partial ID, title, or description text)
// to a concrete task ID. It returns the resolved ID, a slice of candidate matches for diagnostics,
// and a boolean indicating if resolution is confident.
func resolveReference(reference string, tasks []models.Task) (string, []types.TaskMatch, bool) {
	ref := strings.TrimSpace(strings.ToLower(reference))
	if ref == "" {
		return "", nil, false
	}

	// 1) Exact ID match
	for _, t := range tasks {
		if strings.ToLower(t.ID) == ref {
			return t.ID, nil, true
		}
	}

	// 2) Partial ID prefix (>= 8 chars considered meaningful)
	if len(ref) >= 8 {
		var idPrefixMatches []models.Task
		for _, t := range tasks {
			if strings.HasPrefix(strings.ToLower(t.ID), ref) {
				idPrefixMatches = append(idPrefixMatches, t)
			}
		}
		if len(idPrefixMatches) == 1 {
			return idPrefixMatches[0].ID, nil, true
		}
	}

	// 3) Fuzzy match title/description using existing helper
	var all []types.TaskMatch
	titleMatches := findTaskMatches(reference, tasks, "title")
	all = append(all, titleMatches...)
	descMatches := findTaskMatches(reference, tasks, "description")
	all = append(all, descMatches...)

	if len(all) == 0 {
		return "", nil, false
	}

	// Choose best by score
	best := all[0]
	for i := 1; i < len(all); i++ {
		if all[i].Score > best.Score {
			best = all[i]
		}
	}

	// High confidence threshold (tuned down to improve practical matches)
	if best.Score >= 0.7 {
		return best.Task.ID, all, true
	}
	return "", all, false
}
