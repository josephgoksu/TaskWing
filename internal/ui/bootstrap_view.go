package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	agentcore "github.com/josephgoksu/TaskWing/internal/agents/core"
)

// RenderBootstrapResults displays the bootstrap coverage report using the same
// visual language as tw knowledge (bordered header, dim stats, grouped sections).
func RenderBootstrapResults(report *agentcore.BootstrapReport) {
	// Header box - matches knowledge command style
	headerBox := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary)

	fmt.Println()
	fmt.Println(headerBox.Render(fmt.Sprintf("Bootstrap Results (%d findings)", report.TotalFindings)))

	// Stats line - dim, below header, matches knowledge style
	var statParts []string
	if len(report.FindingCounts) > 0 {
		for fType, count := range report.FindingCounts {
			statParts = append(statParts, fmt.Sprintf("%d %s", count, fType))
		}
	}
	if report.Coverage.FilesAnalyzed > 0 {
		fileStat := fmt.Sprintf("%d files analyzed", report.Coverage.FilesAnalyzed)
		if report.Coverage.FilesSkipped > 0 {
			fileStat += fmt.Sprintf(", %d skipped", report.Coverage.FilesSkipped)
		}
		statParts = append(statParts, fileStat)
	}
	if len(statParts) > 0 {
		fmt.Printf("  %s\n", StyleSubtle.Render(strings.Join(statParts, "  ")))
	}
	fmt.Println()

	// Agent results - grouped with badges like knowledge sections
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorText)
	itemStyle := lipgloss.NewStyle().Foreground(ColorText)
	indexStyle := lipgloss.NewStyle().Foreground(ColorDim)
	skipStyle := lipgloss.NewStyle().Foreground(ColorDim)

	var active, skipped []agentEntry
	for name, ar := range report.AgentReports {
		if ar.FindingCount > 0 {
			active = append(active, agentEntry{name: name, report: ar})
		} else {
			skipped = append(skipped, agentEntry{name: name, report: ar})
		}
	}

	// Active agents with findings
	if len(active) > 0 {
		label := fmt.Sprintf("Agents with findings (%d)", len(active))
		fmt.Printf("  %s %s\n", StyleSuccess.Render("✓"), sectionStyle.Render(label))
		for i, a := range active {
			findingWord := "findings"
			if a.report.FindingCount == 1 {
				findingWord = "finding"
			}
			idx := indexStyle.Render(fmt.Sprintf("%d.", i+1))
			text := fmt.Sprintf("%s: %d %s", a.name, a.report.FindingCount, findingWord)
			fmt.Printf("    %s %s\n", idx, itemStyle.Render(text))
		}
		fmt.Println()
	}

	// Skipped agents
	if len(skipped) > 0 {
		label := fmt.Sprintf("Skipped (%d)", len(skipped))
		fmt.Printf("  %s %s\n", skipStyle.Render("-"), skipStyle.Render(label))
		for _, a := range skipped {
			reason := summarizeSkipReason(a.report.Error)
			fmt.Printf("    %s\n", skipStyle.Render(fmt.Sprintf("%s: %s", a.name, reason)))
		}
		fmt.Println()
	}

	fmt.Printf("  %s\n", StyleSubtle.Render("Full report: .taskwing/last-bootstrap-report.json"))
}

type agentEntry struct {
	name   string
	report agentcore.AgentReport
}

// summarizeSkipReason returns a brief human-readable reason for skipping.
func summarizeSkipReason(errMsg string) string {
	if errMsg == "" {
		return "no findings"
	}
	lower := strings.ToLower(errMsg)
	switch {
	case strings.Contains(lower, "no source files") || strings.Contains(lower, "no source code"):
		return "no source files"
	case strings.Contains(lower, "no git history"):
		return "no git history"
	case strings.Contains(lower, "no dependency"):
		return "no dependency files"
	case strings.Contains(lower, "chunking failed"):
		return "no source files"
	default:
		if len(errMsg) > 40 {
			return errMsg[:40] + "..."
		}
		return errMsg
	}
}
