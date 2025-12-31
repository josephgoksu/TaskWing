package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
)

// RenderBootstrapDashboard displays the summary of the bootstrap process
func RenderBootstrapDashboard(findings []core.Finding) {
	var (
		headerStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
		cardStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).Padding(0, 1).BorderForeground(ColorSecondary).MarginLeft(1)
		keyStyle    = lipgloss.NewStyle().Foreground(ColorPrimary)
		valStyle    = lipgloss.NewStyle().Foreground(ColorText)
		subtleStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	)

	// 1. Extract Stack
	var stack []string
	for _, f := range findings {
		if f.Type == core.FindingTypeDependency {
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
	grouped := core.GroupFindingsByType(findings)
	counts := fmt.Sprintf("🎯 %d Decisions • 📦 %d Features • 🧩 %d Patterns",
		len(grouped[core.FindingTypeDecision]),
		len(grouped[core.FindingTypeFeature]),
		len(grouped[core.FindingTypePattern]))

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
	var highlights []core.Finding
	for _, f := range findings {
		if f.Type == core.FindingTypeDecision && f.Why != "" {
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
