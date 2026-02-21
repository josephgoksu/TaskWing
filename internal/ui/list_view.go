package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	// Render Header
	headerBox := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary)

	fmt.Println(headerBox.Render(fmt.Sprintf("Knowledge: %d nodes (%s)", totalCount, strings.Join(stats, " | "))))
	fmt.Println()

	if verbose {
		renderVerboseTable(byType, typeOrder)
	} else {
		renderStyledTable(byType, typeOrder)
	}
}

// renderStyledTable renders all nodes in a single styled table with category badges.
func renderStyledTable(byType map[string][]memory.Node, typeOrder []string) {
	// Collect all nodes in order
	type nodeRow struct {
		node memory.Node
	}
	var rows []nodeRow

	for _, t := range typeOrder {
		for _, n := range byType[t] {
			rows = append(rows, nodeRow{node: n})
		}
	}

	if len(rows) == 0 {
		return
	}

	// Column widths
	const (
		colBadge     = 15
		colSummary   = 50
		colWorkspace = 12
	)

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Underline(true)
	dimSep := StyleSubtle.Render("  ")

	fmt.Printf("  %s%s%s%s%s\n",
		headerStyle.Render(padRight("Category", colBadge)),
		dimSep,
		headerStyle.Render(padRight("Summary", colSummary)),
		dimSep,
		headerStyle.Render(padRight("Workspace", colWorkspace)),
	)

	// Separator
	fmt.Printf("  %s\n", StyleSubtle.Render(strings.Repeat("─", colBadge+colSummary+colWorkspace+6)))

	// Rows with alternating colors
	for i, r := range rows {
		summary := r.node.Summary
		if summary == "" {
			summary = utils.Truncate(r.node.Text(), colSummary)
		}
		if len(summary) > colSummary {
			summary = summary[:colSummary-1] + "…"
		}

		workspace := r.node.Workspace
		if workspace == "" {
			workspace = "root"
		}

		badge := CategoryBadge(r.node.Type)

		// Alternating row style
		var rowStyle lipgloss.Style
		if i%2 == 0 {
			rowStyle = StyleTableRowEven
		} else {
			rowStyle = StyleTableRowOdd
		}

		fmt.Printf("  %s  %s  %s\n",
			badge+strings.Repeat(" ", max(0, colBadge-lipgloss.Width(badge))),
			rowStyle.Render(padRight(summary, colSummary)),
			StyleSubtle.Render(padRight(workspace, colWorkspace)),
		)
	}
	fmt.Println()
}

// renderVerboseTable renders nodes as a table with full metadata.
func renderVerboseTable(byType map[string][]memory.Node, typeOrder []string) {
	for _, t := range typeOrder {
		groupNodes := byType[t]
		if len(groupNodes) == 0 {
			continue
		}

		fmt.Printf("  %s %s\n", CategoryBadge(t), StyleHeader.Render(fmt.Sprintf("%s %ss", TypeIcon(t), utils.ToTitle(t))))

		table := &Table{
			Headers:  []string{"ID", "Summary", "Workspace", "Created", "Agent"},
			MaxWidth: 40,
		}

		for _, n := range groupNodes {
			summary := n.Summary
			if summary == "" {
				summary = utils.Truncate(n.Text(), 40)
			}

			agent := n.SourceAgent
			if agent == "" {
				agent = "-"
			}

			workspace := n.Workspace
			if workspace == "" {
				workspace = "root"
			}

			table.Rows = append(table.Rows, []string{
				TruncateID(n.ID),
				summary,
				workspace,
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
		return "D"
	case memory.NodeTypeFeature:
		return "F"
	case memory.NodeTypeConstraint:
		return "C"
	case memory.NodeTypePattern:
		return "P"
	case memory.NodeTypePlan:
		return "PL"
	case memory.NodeTypeNote:
		return "N"
	case memory.NodeTypeMetadata:
		return "M"
	case memory.NodeTypeDocumentation:
		return "DOC"
	default:
		return "?"
	}
}
