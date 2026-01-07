package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// SearchStrategyResult contains the context and the strategy description
type SearchStrategyResult struct {
	Context  string
	Strategy string
}

// loadArchitectureMD attempts to load the generated ARCHITECTURE.md file.
// Returns empty string if not found (graceful degradation).
func loadArchitectureMD(basePath string) string {
	if basePath == "" {
		return ""
	}
	archPath := filepath.Join(basePath, "ARCHITECTURE.md")
	data, err := os.ReadFile(archPath)
	if err != nil {
		return "" // Not found or unreadable - gracefully skip
	}
	return string(data)
}

// RetrieveContext performs the standard context retrieval for planning and evaluation.
// It ensures that both the interactive CLI and the evaluation system use the exact same logic.
// If memoryBasePath is provided, it will also inject the ARCHITECTURE.md content.
func RetrieveContext(ctx context.Context, ks *knowledge.Service, goal string, memoryBasePath string) (SearchStrategyResult, error) {
	var searchLog []string

	// === NEW: Load comprehensive ARCHITECTURE.md if available ===
	archContent := loadArchitectureMD(memoryBasePath)
	if archContent != "" {
		searchLog = append(searchLog, "✓ Loaded ARCHITECTURE.md")
	}

	// === 0. Fetch Constraints Explicitly ===
	// Always retrieve 'constraint' type nodes, regardless of goal.
	// These represent mandatory rules and must be highlighted.
	// QA FIX: Use ListNodesByType to avoid semantic filtering.
	constraintNodes, _ := ks.ListNodesByType(ctx, memory.NodeTypeConstraint)

	// 1. Strategize: Generate search queries
	queries, err := ks.SuggestContextQueries(ctx, goal)
	if err != nil {
		queries = []string{goal, "Technology Stack"}
	}

	// 2. Execute Searches
	uniqueNodes := make(map[string]knowledge.ScoredNode)

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
	var sb strings.Builder

	// === NEW: Include ARCHITECTURE.md first (most comprehensive context) ===
	if archContent != "" {
		sb.WriteString("## PROJECT ARCHITECTURE OVERVIEW\n")
		sb.WriteString("Consolidated architecture document for this codebase:\n\n")
		sb.WriteString(archContent)
		sb.WriteString("\n---\n\n")
	}

	// === Format Constraints (highlighted separately for emphasis) ===
	if len(constraintNodes) > 0 {
		sb.WriteString("## MANDATORY ARCHITECTURAL CONSTRAINTS\n")
		sb.WriteString("These rules MUST be obeyed by all generated tasks.\n\n")
		for _, n := range constraintNodes {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", n.Summary, n.Content))
		}
		sb.WriteString("\n")
		// Prepend to search log so it appears first
		searchLog = append([]string{"✓ Loaded mandatory constraints."}, searchLog...)
	}

	sb.WriteString("## RELEVANT ARCHITECTURAL CONTEXT\n")

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

	// Format strategy log
	var strategyLog strings.Builder
	strategyLog.WriteString("Research Strategy:\n")
	for _, log := range searchLog {
		strategyLog.WriteString("  " + log + "\n")
	}

	return SearchStrategyResult{
		Context:  sb.String(),
		Strategy: strategyLog.String(),
	}, nil
}
