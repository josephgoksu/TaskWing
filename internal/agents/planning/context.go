package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/knowledge"
)

// SearchStrategyResult contains the context and the strategy description
type SearchStrategyResult struct {
	Context  string
	Strategy string
}

// RetrieveContext performs the standard context retrieval for planning and evaluation.
// It ensures that both the interactive CLI and the evaluation system use the exact same logic.
func RetrieveContext(ctx context.Context, ks *knowledge.Service, goal string) (SearchStrategyResult, error) {
	// 1. Strategize: Generate search queries
	queries, err := ks.SuggestContextQueries(ctx, goal)
	if err != nil {
		queries = []string{goal, "Technology Stack"}
	}

	// 2. Execute Searches
	uniqueNodes := make(map[string]knowledge.ScoredNode)
	var searchLog []string

	for _, q := range queries {
		// Search (this uses the hybrid FTS + Vector approach defined in knowledge.Service)
		nodes, _ := ks.Search(ctx, q, 3) // Limit 3 per query

		for _, sn := range nodes {
			// Deduplicate by ID
			if _, exists := uniqueNodes[sn.Node.ID]; !exists {
				uniqueNodes[sn.Node.ID] = sn
			} else {
				// Keep higher score
				if sn.Score > uniqueNodes[sn.Node.ID].Score {
					uniqueNodes[sn.Node.ID] = sn
				}
			}
		}
		searchLog = append(searchLog, fmt.Sprintf("• Checking memory for: '%s'", q))
	}

	// 3. Format Context
	var strategyLog strings.Builder
	strategyLog.WriteString("Research Strategy:\n")
	for _, log := range searchLog {
		strategyLog.WriteString("  " + log + "\n")
	}

	var sb strings.Builder
	sb.WriteString("KNOWN ARCHITECTURAL CONTEXT:\n")

	// Sort by Score
	var allNodes []knowledge.ScoredNode
	for _, sn := range uniqueNodes {
		allNodes = append(allNodes, sn)
	}
	sort.Slice(allNodes, func(i, j int) bool {
		return allNodes[i].Score > allNodes[j].Score
	})

	for _, node := range allNodes {
		sb.WriteString(fmt.Sprintf("### [%s] %s\n%s\n", node.Node.Type, node.Node.Summary, node.Node.Content))

		// Append evidence file paths if available (Phase 2 feature)
		if node.Node.Evidence != "" {
			var evidenceList []struct {
				FilePath  string `json:"file_path"`
				StartLine int    `json:"start_line"`
			}
			if json.Unmarshal([]byte(node.Node.Evidence), &evidenceList) == nil && len(evidenceList) > 0 {
				sb.WriteString("Referenced files: ")
				for i, ev := range evidenceList {
					if i > 0 {
						sb.WriteString(", ")
					}
					if ev.StartLine > 0 {
						sb.WriteString(fmt.Sprintf("%s:L%d", ev.FilePath, ev.StartLine))
					} else {
						sb.WriteString(ev.FilePath)
					}
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	return SearchStrategyResult{
		Context:  sb.String(),
		Strategy: strategyLog.String(),
	}, nil
}
