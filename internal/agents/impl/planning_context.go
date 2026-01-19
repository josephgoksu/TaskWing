package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/policy"
	"github.com/spf13/afero"
)

// SearchStrategyResult contains the context and the strategy description
type SearchStrategyResult struct {
	Context  string
	Strategy string
}

// PolicyConstraintsBudget is the maximum tokens to allocate for policy constraints.
// This ensures policy injection doesn't overflow the context budget.
const PolicyConstraintsBudget = 2000

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

// PolicyConstraint represents a constraint extracted from a policy file.
type PolicyConstraint struct {
	Name        string // Policy file name
	Description string // Extracted description from comments
	Rules       []string // Rule names (deny, warn)
}

// loadPolicyConstraints loads policy files and extracts constraint descriptions.
// It respects the token budget to prevent context overflow.
func loadPolicyConstraints(basePath string) ([]PolicyConstraint, string) {
	if basePath == "" {
		return nil, ""
	}

	// Determine policies path
	policiesPath := policy.GetPoliciesPath(filepath.Dir(basePath)) // basePath is .taskwing/memory, go up one level
	loader := policy.NewLoader(afero.NewOsFs(), policiesPath)

	policies, err := loader.LoadAll()
	if err != nil || len(policies) == 0 {
		return nil, ""
	}

	var constraints []PolicyConstraint
	var totalTokens int

	for _, p := range policies {
		constraint := extractPolicyConstraint(p)
		if constraint.Description == "" && len(constraint.Rules) == 0 {
			continue
		}

		// Estimate tokens for this constraint
		constraintText := formatPolicyConstraint(constraint)
		tokens := llm.EstimateTokens(constraintText)

		// Check budget
		if totalTokens+tokens > PolicyConstraintsBudget {
			break // Stop adding policies if budget exceeded
		}

		constraints = append(constraints, constraint)
		totalTokens += tokens
	}

	if len(constraints) == 0 {
		return nil, ""
	}

	// Format all constraints
	var sb strings.Builder
	sb.WriteString("## POLICY CONSTRAINTS\n")
	sb.WriteString("The following policies are enforced. Plans MUST comply with these rules.\n\n")

	for _, c := range constraints {
		sb.WriteString(formatPolicyConstraint(c))
	}

	return constraints, sb.String()
}

// extractPolicyConstraint extracts metadata from a policy file.
// It looks for:
// - Header comments (# Description: ...)
// - Package description comments
// - Rule names (deny, warn)
func extractPolicyConstraint(p *policy.PolicyFile) PolicyConstraint {
	constraint := PolicyConstraint{
		Name: p.Name,
	}

	lines := strings.Split(p.Content, "\n")

	// Extract description from header comments
	var descLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// Extract comment content
			comment := strings.TrimPrefix(trimmed, "#")
			comment = strings.TrimSpace(comment)

			// Skip empty comments and metadata markers
			if comment == "" || strings.HasPrefix(comment, "!") {
				continue
			}

			// Skip package/import comments
			if strings.HasPrefix(strings.ToLower(comment), "package") {
				continue
			}

			descLines = append(descLines, comment)
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "package") && !strings.HasPrefix(trimmed, "import") {
			// Stop at first non-comment, non-empty line
			break
		}
	}

	if len(descLines) > 0 {
		constraint.Description = strings.Join(descLines, " ")
		// Truncate long descriptions
		if len(constraint.Description) > 200 {
			constraint.Description = constraint.Description[:197] + "..."
		}
	}

	// Extract rule names using regex
	denyPattern := regexp.MustCompile(`deny\s+contains`)
	warnPattern := regexp.MustCompile(`warn\s+contains`)

	if denyPattern.MatchString(p.Content) {
		constraint.Rules = append(constraint.Rules, "deny")
	}
	if warnPattern.MatchString(p.Content) {
		constraint.Rules = append(constraint.Rules, "warn")
	}

	return constraint
}

// formatPolicyConstraint formats a single policy constraint for context injection.
func formatPolicyConstraint(c PolicyConstraint) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Policy: %s\n", c.Name))

	if c.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n", c.Description))
	}

	if len(c.Rules) > 0 {
		sb.WriteString(fmt.Sprintf("Rules: %s\n", strings.Join(c.Rules, ", ")))
	}

	sb.WriteString("\n")
	return sb.String()
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

	// === NEW: Load Policy Constraints (OPA-based) ===
	policyConstraints, policySection := loadPolicyConstraints(memoryBasePath)
	if len(policyConstraints) > 0 {
		searchLog = append(searchLog, fmt.Sprintf("✓ Loaded %d policy constraints", len(policyConstraints)))
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

	// === NEW: Include Policy Constraints (OPA-based guardrails) ===
	if policySection != "" {
		sb.WriteString(policySection)
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
