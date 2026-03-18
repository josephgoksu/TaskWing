// Package knowledge provides unified project context retrieval.
// GetProjectContext is the single source of truth for all context consumers:
// planning, task creation, hooks, and MCP tools.
package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/freshness"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// ContextOptions controls what gets retrieved and how deep.
type ContextOptions struct {
	// Query is the goal or question to retrieve context for.
	// Used to generate search queries and find relevant nodes.
	Query string

	// IncludeArchitectureMD loads .taskwing/ARCHITECTURE.md content.
	// Default: true
	IncludeArchitectureMD bool

	// IncludeConstraints fetches all constraint-type nodes explicitly.
	// Default: true
	IncludeConstraints bool

	// IncludeRelevantNodes runs hybrid search for nodes relevant to Query.
	// Default: true
	IncludeRelevantNodes bool

	// UseLLMQueries uses LLM to generate optimized search queries from Query.
	// When false, uses Query directly as the search term.
	// Default: true
	UseLLMQueries bool

	// IncludeSymbols adds code symbol search results.
	// Default: false (opt-in)
	IncludeSymbols bool

	// MaxNodes caps the total number of nodes returned.
	// Default: 15
	MaxNodes int

	// NodesPerQuery limits results per individual search query.
	// Default: 3
	NodesPerQuery int

	// CheckFreshness annotates nodes with stale/missing tags.
	// Default: true
	CheckFreshness bool

	// BasePath is the project root for freshness checks and ARCHITECTURE.md.
	// Resolved from config if empty.
	BasePath string

	// MemoryBasePath is the .taskwing/memory path.
	// Resolved from config if empty.
	MemoryBasePath string
}

// DefaultContextOptions returns options optimized for rich planning-grade context.
func DefaultContextOptions() ContextOptions {
	return ContextOptions{
		IncludeArchitectureMD: true,
		IncludeConstraints:    true,
		IncludeRelevantNodes:  true,
		UseLLMQueries:         true,
		IncludeSymbols:        false,
		MaxNodes:              15,
		NodesPerQuery:         3,
		CheckFreshness:        true,
	}
}

// ProjectContext is the unified payload returned by GetProjectContext.
// All context consumers (planning, tasks, hooks) use this same structure.
type ProjectContext struct {
	// ArchitectureMD is the full content of .taskwing/ARCHITECTURE.md.
	ArchitectureMD string

	// Constraints are all constraint-type nodes from the knowledge graph.
	Constraints []memory.Node

	// RelevantNodes are nodes matching the query, sorted by relevance score.
	RelevantNodes []ScoredNode

	// SearchLog records what retrieval steps were taken (for debugging).
	SearchLog []string
}

// Format renders the context as a single string suitable for LLM prompt injection.
// This is the canonical formatting -- all consumers use this instead of custom formatting.
func (pc *ProjectContext) Format() string {
	var sb strings.Builder

	// ARCHITECTURE.md first (most comprehensive)
	if pc.ArchitectureMD != "" {
		sb.WriteString("## PROJECT ARCHITECTURE OVERVIEW\n")
		sb.WriteString("Consolidated architecture document for this codebase:\n\n")
		sb.WriteString(pc.ArchitectureMD)
		sb.WriteString("\n---\n\n")
	}

	// Constraints (highlighted for emphasis)
	if len(pc.Constraints) > 0 {
		sb.WriteString("## MANDATORY ARCHITECTURAL CONSTRAINTS\n")
		sb.WriteString("These rules MUST be obeyed by all generated tasks.\n\n")
		for _, n := range pc.Constraints {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", n.Summary, n.Text()))
		}
		sb.WriteString("\n")
	}

	// Relevant nodes
	if len(pc.RelevantNodes) > 0 {
		sb.WriteString("## RELEVANT ARCHITECTURAL CONTEXT\n")
		for _, node := range pc.RelevantNodes {
			if node.Node == nil {
				continue
			}
			sb.WriteString(fmt.Sprintf("### [%s] %s\n%s\n", node.Node.Type, node.Node.Summary, node.Node.Text()))

			// Evidence file paths
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
	}

	return sb.String()
}

// FormatCompact returns a shorter version suitable for task context embedding.
// Omits ARCHITECTURE.md (too large for per-task context) and truncates content.
func (pc *ProjectContext) FormatCompact() string {
	var sb strings.Builder

	if len(pc.Constraints) > 0 {
		sb.WriteString("## Architectural Constraints\n")
		for _, n := range pc.Constraints {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", n.Summary, utils.Truncate(n.Text(), 200)))
		}
		sb.WriteString("\n")
	}

	if len(pc.RelevantNodes) > 0 {
		sb.WriteString("## Relevant Context\n")
		for _, node := range pc.RelevantNodes {
			if node.Node == nil {
				continue
			}
			content := utils.Truncate(node.Node.Text(), 300)
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", node.Node.Summary, node.Node.Type, content))
		}
	}

	return sb.String()
}

// GetProjectContext performs unified project context retrieval.
// This is the single source of truth for all context consumers.
// Always reads live from the database -- no caching.
func GetProjectContext(ctx context.Context, svc *Service, opts ContextOptions) (*ProjectContext, error) {
	pc := &ProjectContext{}

	// Resolve paths
	basePath := opts.BasePath
	memoryBasePath := opts.MemoryBasePath
	if basePath == "" && memoryBasePath != "" {
		basePath = filepath.Dir(filepath.Dir(memoryBasePath))
	}

	// 1. Load ARCHITECTURE.md
	if opts.IncludeArchitectureMD && memoryBasePath != "" {
		archPath := filepath.Join(memoryBasePath, "..", "ARCHITECTURE.md")
		if content, err := os.ReadFile(archPath); err == nil {
			pc.ArchitectureMD = string(content)
			pc.SearchLog = append(pc.SearchLog, "Loaded ARCHITECTURE.md")
		}
	}

	// 2. Fetch all constraints
	if opts.IncludeConstraints {
		constraints, err := svc.ListNodesByType(ctx, memory.NodeTypeConstraint)
		if err != nil {
			pc.SearchLog = append(pc.SearchLog, fmt.Sprintf("Constraint fetch failed: %v", err))
		}
		if err == nil && len(constraints) > 0 {
			// Annotate freshness
			if opts.CheckFreshness && basePath != "" {
				for i := range constraints {
					annotateFreshness(&constraints[i], basePath)
				}
			}
			pc.Constraints = constraints
			pc.SearchLog = append(pc.SearchLog, fmt.Sprintf("Loaded %d constraints", len(constraints)))
		}
	}

	// 3. Hybrid search for relevant nodes
	if opts.IncludeRelevantNodes && opts.Query != "" {
		var queries []string

		if opts.UseLLMQueries {
			generated, err := svc.SuggestContextQueries(ctx, opts.Query)
			if err == nil && len(generated) > 0 {
				queries = generated
			} else {
				queries = []string{opts.Query, "Technology Stack"}
			}
		} else {
			queries = []string{opts.Query}
		}

		// Execute searches with deduplication
		uniqueNodes := make(map[string]ScoredNode)
		for _, q := range queries {
			nodes, err := svc.Search(ctx, q, opts.NodesPerQuery)
			if err != nil {
				pc.SearchLog = append(pc.SearchLog, fmt.Sprintf("Search failed for '%s': %v", q, err))
				continue
			}
			for _, sn := range nodes {
				if existing, exists := uniqueNodes[sn.Node.ID]; !exists || sn.Score > existing.Score {
					uniqueNodes[sn.Node.ID] = sn
				}
			}
			pc.SearchLog = append(pc.SearchLog, fmt.Sprintf("Searched: '%s'", q))
		}

		// Sort by score, cap at MaxNodes
		var allNodes []ScoredNode
		for _, sn := range uniqueNodes {
			allNodes = append(allNodes, sn)
		}
		sort.Slice(allNodes, func(i, j int) bool {
			return allNodes[i].Score > allNodes[j].Score
		})
		if opts.MaxNodes > 0 && len(allNodes) > opts.MaxNodes {
			allNodes = allNodes[:opts.MaxNodes]
		}

		// Annotate freshness
		if opts.CheckFreshness && basePath != "" {
			for i := range allNodes {
				annotateFreshnessScored(&allNodes[i], basePath)
			}
		}

		pc.RelevantNodes = allNodes
		pc.SearchLog = append(pc.SearchLog, fmt.Sprintf("Found %d relevant nodes", len(allNodes)))
	}

	return pc, nil
}

// annotateFreshness adds a stale tag to a node's Summary if its evidence is stale.
func annotateFreshness(n *memory.Node, basePath string) {
	if n.Evidence == "" {
		return
	}
	result := freshness.Check(basePath, n.Evidence, n.CreatedAt)
	if result.Status == freshness.StatusStale {
		n.Summary += " [STALE]"
	} else if result.Status == freshness.StatusMissing {
		n.Summary += " [MISSING]"
	}
}

// annotateFreshnessScored adds a stale tag to a scored node.
func annotateFreshnessScored(sn *ScoredNode, basePath string) {
	if sn.Node == nil || sn.Node.Evidence == "" {
		return
	}
	result := freshness.Check(basePath, sn.Node.Evidence, sn.Node.CreatedAt)
	if result.Status == freshness.StatusStale {
		sn.Node.Summary += " [STALE]"
	} else if result.Status == freshness.StatusMissing {
		sn.Node.Summary += " [MISSING]"
	}
}

