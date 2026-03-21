package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/freshness"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// RenderNodeList renders a list of knowledge nodes to stdout in compact mode.
// basePath is the project root for freshness checks (empty to skip).
func RenderNodeList(nodes []memory.Node, basePath string) {
	renderNodeListInternal(nodes, false, basePath)
}

// RenderNodeListVerbose renders nodes with full metadata (ID, dates, type).
func RenderNodeListVerbose(nodes []memory.Node, basePath string) {
	renderNodeListInternal(nodes, true, basePath)
}

func renderNodeListInternal(nodes []memory.Node, verbose bool, basePath string) {
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

	totalCount := 0
	for _, t := range typeOrder {
		totalCount += len(byType[t])
	}

	// Check if workspace column is useful (more than one distinct workspace)
	showWorkspace := hasMultipleWorkspaces(nodes)

	// Render header with readable type counts
	renderHeader(byType, typeOrder, totalCount)

	if verbose {
		renderVerboseTable(byType, typeOrder)
	} else {
		renderGroupedList(byType, typeOrder, showWorkspace, basePath)
	}
}

// renderHeader displays the summary box with spelled-out type names.
func renderHeader(byType map[string][]memory.Node, typeOrder []string, total int) {
	var statParts []string
	for _, t := range typeOrder {
		count := len(byType[t])
		if count > 0 {
			label := typePlural(t, count)
			statParts = append(statParts, fmt.Sprintf("%d %s", count, label))
		}
	}

	headerBox := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary)

	fmt.Println(headerBox.Render(fmt.Sprintf("Project Knowledge (%d nodes)", total)))

	// Stats line below the box - readable breakdown
	if len(statParts) > 0 {
		fmt.Printf("  %s\n", StyleSubtle.Render(strings.Join(statParts, "  ")))
	}
	fmt.Println()
}

// renderGroupedList renders nodes grouped by type with section headers.
// Each section shows a colored badge + count, then a simple indented list.
func renderGroupedList(byType map[string][]memory.Node, typeOrder []string, showWorkspace bool, basePath string) {
	termWidth := GetTerminalWidth()

	// Calculate the widest workspace name across all nodes
	maxWsWidth := 0
	if showWorkspace {
		for _, t := range typeOrder {
			for _, n := range byType[t] {
				ws := n.Workspace
				if ws == "" {
					ws = "root"
				}
				if len(ws) > maxWsWidth {
					maxWsWidth = len(ws)
				}
			}
		}
		// Minimum workspace column width
		if maxWsWidth < 4 {
			maxWsWidth = 4
		}
	}

	// Find the largest group to determine index column width
	maxGroupSize := 0
	for _, t := range typeOrder {
		if len(byType[t]) > maxGroupSize {
			maxGroupSize = len(byType[t])
		}
	}
	indexWidth := len(fmt.Sprintf("%d.", maxGroupSize))
	if indexWidth < 2 {
		indexWidth = 2
	}

	// Calculate available width for summary text
	// Layout: 4 indent + indexWidth + 1 space + summary + 2 gap + workspace
	overhead := 4 + indexWidth + 1 + 2 // indent + index + space + safety
	if showWorkspace {
		overhead += 2 + maxWsWidth // gap + workspace column
	}
	maxSummaryWidth := termWidth - overhead
	if maxSummaryWidth < 40 {
		maxSummaryWidth = 40
	}

	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorText)
	itemStyle := lipgloss.NewStyle().Foreground(ColorText)
	indexStyle := lipgloss.NewStyle().Foreground(ColorDim)
	staleStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	wsStyle := lipgloss.NewStyle().Foreground(ColorDim)

	for _, t := range typeOrder {
		groupNodes := byType[t]
		if len(groupNodes) == 0 {
			continue
		}

		// Section header: badge + "Decisions (6)"
		badge := CategoryBadge(t)
		label := fmt.Sprintf("%s (%d)", utils.ToTitle(typePlural(t, len(groupNodes))), len(groupNodes))
		fmt.Printf("  %s %s\n", badge, sectionStyle.Render(label))

		// Items with numbered indices for scanability
		for i, n := range groupNodes {
			summary := n.Summary
			if summary == "" {
				summary = utils.Truncate(n.Text(), maxSummaryWidth-4)
			}

			// Check freshness if basePath is available and node has evidence
			staleTag := ""
			staleTagWidth := 0
			if basePath != "" && n.Evidence != "" {
				result := freshness.Check(basePath, n.Evidence, n.CreatedAt)
				if result.Status == freshness.StatusStale {
					staleTag = staleStyle.Render(" [stale]")
					staleTagWidth = 8 // " [stale]"
				} else if result.Status == freshness.StatusMissing {
					staleTag = staleStyle.Render(" [missing]")
					staleTagWidth = 10 // " [missing]"
				}
			}

			// Account for stale tag width in truncation
			availWidth := maxSummaryWidth
			if staleTagWidth > 0 {
				availWidth -= staleTagWidth
			}
			if lipgloss.Width(summary) > availWidth {
				summary = truncateToWidth(summary, availWidth)
			}

			// Right-align index numbers within the index column
			idxText := fmt.Sprintf("%d.", i+1)
			idx := indexStyle.Render(padRight(idxText, indexWidth))

			if showWorkspace {
				ws := n.Workspace
				if ws == "" {
					ws = "root"
				}
				fmt.Printf("    %s %s%s  %s\n", idx, itemStyle.Render(padRight(summary, availWidth)), staleTag, wsStyle.Render(ws))
			} else {
				fmt.Printf("    %s %s%s\n", idx, itemStyle.Render(summary), staleTag)
			}
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

		label := fmt.Sprintf("%s (%d)", utils.ToTitle(typePlural(t, len(groupNodes))), len(groupNodes))
		fmt.Printf("  %s %s\n", CategoryBadge(t), StyleHeader.Render(label))

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

// hasMultipleWorkspaces returns true if nodes span more than one workspace.
// When false, the workspace column can be hidden to save horizontal space.
func hasMultipleWorkspaces(nodes []memory.Node) bool {
	if len(nodes) == 0 {
		return false
	}
	first := nodes[0].Workspace
	if first == "" {
		first = "root"
	}
	for _, n := range nodes[1:] {
		ws := n.Workspace
		if ws == "" {
			ws = "root"
		}
		if ws != first {
			return true
		}
	}
	return false
}

// typePlural returns the human-readable plural label for a node type.
func typePlural(t string, count int) string {
	if count == 1 {
		return typeSingular(t)
	}
	switch t {
	case memory.NodeTypeDecision:
		return "decisions"
	case memory.NodeTypeFeature:
		return "features"
	case memory.NodeTypeConstraint:
		return "constraints"
	case memory.NodeTypePattern:
		return "patterns"
	case memory.NodeTypePlan:
		return "plans"
	case memory.NodeTypeNote:
		return "notes"
	case memory.NodeTypeMetadata:
		return "metadata"
	case memory.NodeTypeDocumentation:
		return "docs"
	default:
		return t + "s"
	}
}

// typeSingular returns the human-readable singular label for a node type.
func typeSingular(t string) string {
	switch t {
	case memory.NodeTypeDocumentation:
		return "doc"
	case memory.NodeTypeMetadata:
		return "metadata"
	default:
		return t
	}
}

// TypeIcon returns a short abbreviation for the header stats.
// Used by bootstrap output and other compact displays.
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
