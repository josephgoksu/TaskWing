package ui

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// RenderNodeList renders a list of knowledge nodes to stdout in compact mode.
// For verbose output with full metadata, use RenderNodeListVerbose.
func RenderNodeList(nodes []memory.Node) {
	renderNodeListInternal(nodes, false)
}

// RenderNodeListVerbose renders nodes with full metadata (ID, dates, type).
func RenderNodeListVerbose(nodes []memory.Node) {
	renderNodeListInternal(nodes, true)
}

func renderNodeListInternal(nodes []memory.Node, verbose bool) {
	// Group by type
	byType := make(map[string][]memory.Node)
	for _, n := range nodes {
		t := n.Type
		if t == "" {
			t = "unknown"
		}
		byType[t] = append(byType[t], n)
	}

	// Calculate stats - use centralized type list
	typeOrder := append(memory.AllNodeTypes(), "unknown")
	var stats []string
	totalCount := 0

	for _, t := range typeOrder {
		count := len(byType[t])
		if count > 0 {
			totalCount += count
			stats = append(stats, fmt.Sprintf("%s %d", TypeIcon(t), count))
		}
	}

	// Render Header Summary
	fmt.Printf(" 🧠 Knowledge: %d nodes (%s)\n", totalCount, strings.Join(stats, " • "))
	fmt.Println(StyleSubtle.Render(strings.Repeat("─", 50)))

	if verbose {
		// Verbose mode: Table with full metadata
		renderVerboseTable(byType, typeOrder)
	} else {
		// Compact mode: Grouped bullet lists
		renderCompactList(byType, typeOrder)
	}
}

// renderCompactList renders nodes as compact grouped bullet lists.
func renderCompactList(byType map[string][]memory.Node, typeOrder []string) {
	for _, t := range typeOrder {
		groupNodes := byType[t]
		if len(groupNodes) == 0 {
			continue
		}

		fmt.Println(StyleHeader.Render(fmt.Sprintf("%s %ss", TypeIcon(t), utils.ToTitle(t))))

		for _, n := range groupNodes {
			summary := n.Summary
			if summary == "" {
				summary = utils.Truncate(n.Content, 60)
			}

			// Compact: just summary, no metadata
			fmt.Printf(" • %s\n", StyleTitle.Render(summary))
		}
		fmt.Println()
	}
}

// renderVerboseTable renders nodes as a table with full metadata.
func renderVerboseTable(byType map[string][]memory.Node, typeOrder []string) {
	for _, t := range typeOrder {
		groupNodes := byType[t]
		if len(groupNodes) == 0 {
			continue
		}

		fmt.Println(StyleHeader.Render(fmt.Sprintf("%s %ss", TypeIcon(t), utils.ToTitle(t))))

		table := &Table{
			Headers:  []string{"ID", "Summary", "Created", "Agent"},
			MaxWidth: 45,
		}

		for _, n := range groupNodes {
			summary := n.Summary
			if summary == "" {
				summary = utils.Truncate(n.Content, 45)
			}

			agent := n.SourceAgent
			if agent == "" {
				agent = "-"
			}

			table.Rows = append(table.Rows, []string{
				TruncateID(n.ID),
				summary,
				n.CreatedAt.Format("Jan 02 15:04"),
				agent,
			})
		}

		fmt.Print(table.Render())
		fmt.Println()
	}
}

func TypeIcon(t string) string {
	switch t {
	case memory.NodeTypeDecision:
		return "🎯"
	case memory.NodeTypeFeature:
		return "📦"
	case memory.NodeTypeConstraint:
		return "⚠️"
	case memory.NodeTypePattern:
		return "🧩"
	case memory.NodeTypePlan:
		return "📋"
	case memory.NodeTypeNote:
		return "📝"
	case memory.NodeTypeMetadata:
		return "📊"
	case memory.NodeTypeDocumentation:
		return "📄"
	default:
		return "❓"
	}
}
