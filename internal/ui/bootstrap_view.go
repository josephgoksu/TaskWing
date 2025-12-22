package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/agents"
)

// RenderBootstrapDashboard displays the summary of the bootstrap process
func RenderBootstrapDashboard(findings []agents.Finding) {
	var (
		headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		cardStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).Padding(0, 1).BorderForeground(lipgloss.Color("63")).MarginLeft(1)
		keyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // Pink
		valStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // White
		subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")) // Gray
	)

	// 1. Extract Stack
	var stack []string
	for _, f := range findings {
		if f.Type == agents.FindingTypeDependency {
			stack = append(stack, f.Title)
		}
	}
	// Limit stack items
	if len(stack) > 5 {
		stack = stack[:5]
	}
	stackStr := strings.Join(stack, " • ")
	if stackStr == "" {
		stackStr = "Detecting..."
	}

	// 2. Counts
	grouped := agents.GroupFindingsByType(findings)
	counts := fmt.Sprintf("🎯 %d Decisions • 📦 %d Features • 🧩 %d Patterns",
		len(grouped[agents.FindingTypeDecision]),
		len(grouped[agents.FindingTypeFeature]),
		len(grouped[agents.FindingTypePattern]))

	// Render "DNA" Summary
	fmt.Println()
	fmt.Println(headerStyle.Render(" 🧬 Project DNA"))
	dnaContent := fmt.Sprintf("%s\n%s",
		keyStyle.Render("Stack: ")+valStyle.Render(stackStr),
		keyStyle.Render("Scope: ")+valStyle.Render(counts),
	)
	fmt.Println(cardStyle.Render(dnaContent))
	fmt.Println()

	// 3. Highlights (Top 3 Decisions)
	var highlights []agents.Finding
	for _, f := range findings {
		if f.Type == agents.FindingTypeDecision && f.Why != "" {
			highlights = append(highlights, f)
		}
	}
	if len(highlights) > 3 {
		highlights = highlights[:3]
	}

	if len(highlights) > 0 {
		fmt.Println(headerStyle.Render(" 💡 Highlights"))

		for i, h := range highlights {
			title := h.Title
			why := h.Why
			if len(why) > 70 {
				why = why[:70] + "..."
			}

			fmt.Printf(" %d. %s\n", i+1, valStyle.Render(title))
			fmt.Printf("    %s\n", subtleStyle.Render(why))
		}
		fmt.Println()
	}
}
