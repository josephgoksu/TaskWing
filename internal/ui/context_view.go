package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
)

// RenderContextResults displays search results and optional answer in compact mode.
// For verbose output with full metadata, use RenderContextResultsVerbose.
func RenderContextResults(query string, scored []knowledge.ScoredNode, answer string) {
	renderContextInternal(query, scored, answer, false)
}

// RenderContextResultsVerbose displays search results with full metadata.
func RenderContextResultsVerbose(query string, scored []knowledge.ScoredNode, answer string) {
	renderContextInternal(query, scored, answer, true)
}

func renderContextInternal(query string, scored []knowledge.ScoredNode, answer string, verbose bool) {
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

	// Calculate max score for relative scaling (handles low-score embeddings like Qwen3)
	var maxScore float32 = 0.01 // Minimum to avoid division by zero
	for _, s := range scored {
		if s.Score > maxScore {
			maxScore = s.Score
		}
	}

	for i, s := range scored {
		// Relative scoring: scale to max in result set, minimum 1 bar for any result
		relativeScore := s.Score / maxScore
		barSegments := int(relativeScore * 10)
		if barSegments < 1 && s.Score > 0 {
			barSegments = 1 // At least 1 segment for any non-zero result
		}
		if barSegments > 10 {
			barSegments = 10
		}
		bar := barFull.Render(strings.Repeat("━", barSegments)) + barEmpty.Render(strings.Repeat("━", 10-barSegments))

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

		// Use unified icon mapping from list_view.go
		icon := TypeIcon(s.Node.Type)

		// Add graph expansion indicator
		expandedIndicator := ""
		if s.ExpandedFrom != "" {
			expandedIndicator = " 🔗" // Indicates this came from graph expansion
		}

		if verbose {
			// Verbose: show full metadata
			fmt.Printf(" %d. %s %s\n", i+1, icon, sourceTitle.Render(summary))
			fmt.Printf("    %s\n", metaStyle.Render(fmt.Sprintf("ID: %s | Type: %s | Score: %.2f%s", id, s.Node.Type, s.Score, expandedIndicator)))
			if s.Node.SourceAgent != "" {
				fmt.Printf("    %s\n", metaStyle.Render(fmt.Sprintf("Agent: %s", s.Node.SourceAgent)))
			}
		} else {
			// Compact: single line with score bar
			fmt.Printf(" %d. %s %s %s%s\n", i+1, icon, sourceTitle.Render(summary), metaStyle.Render(fmt.Sprintf("[%s %s]", bar, id)), expandedIndicator)
		}
	}
}
