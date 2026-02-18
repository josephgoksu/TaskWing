/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package brief

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// GenerateCompactBrief generates a compact knowledge summary from the repository.
// It is used for slash-command priming and hook context injection.
// No node IDs, file paths, or embeddings are included.
//
// This function is used by:
// - /tw-brief slash command
// - SessionStart hook auto-injection
func GenerateCompactBrief(repo *memory.Repository) (string, error) {
	nodes, err := repo.ListNodes("")
	if err != nil {
		return "", fmt.Errorf("list nodes: %w", err)
	}

	if len(nodes) == 0 {
		return "No project memory found. Run `taskwing bootstrap`.", nil
	}

	return FormatNodesAsCompactBrief(nodes), nil
}

// FormatNodesAsCompactBrief formats a slice of nodes as a compact brief string.
// Exported for testing and reuse.
func FormatNodesAsCompactBrief(nodes []memory.Node) string {
	// Group by type
	byType := make(map[string][]memory.Node)
	for _, n := range nodes {
		t := n.Type
		if t == "" {
			t = "unknown"
		}
		byType[t] = append(byType[t], n)
	}

	// Calculate stats
	typeOrder := append(memory.AllNodeTypes(), "unknown")
	var stats []string
	totalCount := 0

	for _, t := range typeOrder {
		count := len(byType[t])
		if count > 0 {
			totalCount += count
			stats = append(stats, fmt.Sprintf("%s %d", typeIcon(t), count))
		}
	}

	var sb strings.Builder

	// Header
	fmt.Fprintf(&sb, "Knowledge: %d nodes (%s)\n", totalCount, strings.Join(stats, " | "))
	sb.WriteString(strings.Repeat("-", 50) + "\n")

	// Groups
	for _, t := range typeOrder {
		groupNodes := byType[t]
		if len(groupNodes) == 0 {
			continue
		}

		fmt.Fprintf(&sb, "\n%s %ss\n", typeIcon(t), utils.ToTitle(t))

		for _, n := range groupNodes {
			summary := n.Summary
			if summary == "" {
				summary = utils.Truncate(n.Text(), 60)
			}
			fmt.Fprintf(&sb, "- %s\n", summary)
		}
	}

	return sb.String()
}

// typeIcon returns the emoji icon for a node type.
func typeIcon(t string) string {
	switch t {
	case memory.NodeTypeDecision:
		return "D" // Decisions
	case memory.NodeTypeFeature:
		return "F" // Features
	case memory.NodeTypeConstraint:
		return "C" // Constraints
	case memory.NodeTypePattern:
		return "P" // Patterns
	case memory.NodeTypePlan:
		return "PL" // Plans
	case memory.NodeTypeNote:
		return "N" // Notes
	case memory.NodeTypeMetadata:
		return "M" // Metadata
	case memory.NodeTypeDocumentation:
		return "DOC" // Documentation
	default:
		return "?"
	}
}
