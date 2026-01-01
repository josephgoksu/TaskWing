package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
)

// RenderContextResults displays search results and optional answer
func RenderContextResults(query string, scored []knowledge.ScoredNode, answer string) {
	// Styles
	var (
		cardStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).Padding(0, 2).MarginTop(1).BorderForeground(lipgloss.Color("63"))
		titleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
		sourceTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		metaStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		barFull     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		barEmpty    = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	)

	// Render Answer Summary
	if answer != "" {
		fmt.Println()
		fmt.Println(titleStyle.Render(fmt.Sprintf("📖 %s", query)))
		fmt.Println(cardStyle.Render(answer))
	} else {
		fmt.Println(titleStyle.Render(fmt.Sprintf("🔍 Context for: \"%s\"", query)))
	}

	// Render Sources
	fmt.Println()
	if answer != "" {
		fmt.Println(titleStyle.Render("📚 Sources"))
	}

	for i, s := range scored {
		score := int(s.Score * 10)
		if score > 10 {
			score = 10
		}
		if score < 0 {
			score = 0
		}
		bar := barFull.Render(strings.Repeat("━", score)) + barEmpty.Render(strings.Repeat("━", 10-score))

		summary := s.Node.Summary
		if summary == "" {
			// truncate content if summary is missing
			runes := []rune(s.Node.Content)
			if len(runes) > 60 {
				summary = string(runes[:60]) + "..."
			} else {
				summary = string(runes)
			}
		}

		id := s.Node.ID
		if len(id) > 6 {
			id = id[:6]
		}

		// Map type to icon (simple version)
		icon := "📄"
		switch s.Node.Type {
		case "decision":
			icon = "🤔"
		case "feature":
			icon = "✨"
		case "pattern":
			icon = "🧩"
		}

		// Add graph expansion indicator
		expandedIndicator := ""
		if s.ExpandedFrom != "" {
			expandedIndicator = " 🔗" // Indicates this came from graph expansion
		}

		fmt.Printf(" %d. %s %s %s%s\n", i+1, icon, sourceTitle.Render(summary), metaStyle.Render(fmt.Sprintf("[%s %s]", bar, id)), expandedIndicator)
	}
}
