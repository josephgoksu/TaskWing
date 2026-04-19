package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// RenderAskResult displays a complete AskResult from the ask pipeline.
// This is the primary rendering function for the `taskwing ask` command.
func RenderAskResult(result *app.AskResult, verbose bool) {
	sectionStyle := lipgloss.NewStyle().Foreground(ColorPurple).Bold(true)

	fmt.Println()

	// Compact header: just the query, no box when there's an answer
	if result.Answer != "" {
		fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(
			fmt.Sprintf("Q: %s", result.Query)))
	} else {
		headerBox := lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSecondary)
		fmt.Println(headerBox.Render(fmt.Sprintf("Search: %s", result.Query)))
	}

	// Show result count as dim metadata (skip internal pipeline details)
	if verbose {
		var metaParts []string
		metaParts = append(metaParts, result.Pipeline)
		if result.RewrittenQuery != "" {
			metaParts = append(metaParts, fmt.Sprintf("rewritten: %s", result.RewrittenQuery))
		}
		if result.Total > 0 || result.TotalSymbols > 0 {
			metaParts = append(metaParts, fmt.Sprintf("%d knowledge, %d symbols", result.Total, result.TotalSymbols))
		}
		fmt.Println(StyleAskMeta.Render("  " + strings.Join(metaParts, " | ")))
	}

	// Warning
	if result.Warning != "" {
		fmt.Println()
		fmt.Println(RenderWarningPanel("Warning", result.Warning))
	}

	// Answer: render markdown with terminal formatting, adaptive width
	if result.Answer != "" {
		fmt.Println()
		termWidth := GetTerminalWidth()
		answerWidth := termWidth - 6 // 4 for border + 2 for padding
		if answerWidth < 60 {
			answerWidth = 60
		}
		if answerWidth > 120 {
			answerWidth = 120
		}

		formatted := formatMarkdownForTerminal(result.Answer)
		answerBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBlue).
			Padding(0, 2).
			Width(answerWidth)
		fmt.Println(answerBox.Render(formatted))
	}

	// Sources as compact citations
	if len(result.Results) > 0 {
		fmt.Println()
		fmt.Println(sectionStyle.Render("Sources"))
		fmt.Println()

		scored := nodeResponsesToScoredNodes(result.Results)

		var maxScore float32 = 0.01
		for _, s := range scored {
			if s.Score > maxScore {
				maxScore = s.Score
			}
		}

		for i, s := range scored {
			renderCitation(i+1, s, maxScore)
		}
	}

	// Code symbols
	if len(result.Symbols) > 0 {
		fmt.Println()
		fmt.Println(sectionStyle.Render("Code Symbols"))
		fmt.Println()

		for i, sym := range result.Symbols {
			renderSymbolCitation(i+1, sym)
		}
	}

	// No results
	if len(result.Results) == 0 && len(result.Symbols) == 0 && result.Answer == "" {
		fmt.Println()
		fmt.Println(StyleAskMeta.Render("  No results found. Try a different query or run 'taskwing bootstrap' to populate memory."))
	}

	fmt.Println()
}

// formatMarkdownForTerminal applies basic terminal formatting to markdown text.
// Converts headings to bold+colored, **bold** to bold, --- to dim separators,
// and preserves list indentation. This avoids a full glamour dependency.
func formatMarkdownForTerminal(md string) string {
	headingStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	subheadingStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorText)
	separatorStyle := lipgloss.NewStyle().Foreground(ColorDim)
	boldStyle := lipgloss.NewStyle().Bold(true)

	lines := strings.Split(md, "\n")
	var out []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Horizontal rules
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			out = append(out, separatorStyle.Render(strings.Repeat("-", 50)))
			continue
		}

		// Headings: ## and ###
		if strings.HasPrefix(trimmed, "### ") {
			text := strings.TrimPrefix(trimmed, "### ")
			text = stripMarkdownBold(text)
			out = append(out, "")
			out = append(out, subheadingStyle.Render(text))
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			text := strings.TrimPrefix(trimmed, "## ")
			text = stripMarkdownBold(text)
			out = append(out, "")
			out = append(out, headingStyle.Render(text))
			continue
		}

		// Inline **bold** replacement
		processed := replaceMarkdownBold(line, boldStyle)
		out = append(out, processed)
	}

	return strings.Join(out, "\n")
}

// stripMarkdownBold removes **markers** from text.
func stripMarkdownBold(s string) string {
	return strings.ReplaceAll(s, "**", "")
}

// replaceMarkdownBold converts **text** to bold-styled text.
func replaceMarkdownBold(line string, style lipgloss.Style) string {
	result := line
	for {
		start := strings.Index(result, "**")
		if start == -1 {
			break
		}
		end := strings.Index(result[start+2:], "**")
		if end == -1 {
			break
		}
		end += start + 2
		boldText := result[start+2 : end]
		result = result[:start] + style.Render(boldText) + result[end+2:]
	}
	return result
}

// renderCitation renders a knowledge source as a compact citation line.
func renderCitation(index int, s knowledge.ScoredNode, maxScore float32) {
	summary := s.Node.Summary
	if summary == "" {
		runes := []rune(s.Node.Text())
		if len(runes) > 60 {
			summary = string(runes[:60]) + "..."
		} else {
			summary = string(runes)
		}
	}

	badge := CategoryBadge(s.Node.Type)
	scoreBar := renderMiniBar(s.Score, maxScore)

	id := s.Node.ID
	if len(id) > 8 {
		id = id[:8]
	}

	fmt.Printf("  %s %s  %s  %s\n",
		StyleCitationBadge.Render(fmt.Sprintf("[%d]", index)),
		badge,
		lipgloss.NewStyle().Foreground(ColorText).Render(summary),
		StyleCitationPath.Render(fmt.Sprintf("(%s %s)", id, scoreBar)),
	)
}

// renderSymbolCitation renders a code symbol as a compact citation line.
func renderSymbolCitation(index int, sym app.SymbolResponse) {
	icon := symbolKindIcon(sym.Kind)
	fmt.Printf("  %s %s %s  %s\n",
		StyleCitationBadge.Render(fmt.Sprintf("[%d]", index)),
		icon,
		lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(sym.Name),
		StyleCitationPath.Render(sym.Location),
	)
}

// renderMiniBar renders a compact score indicator.
func renderMiniBar(score, maxScore float32) string {
	rel := score / maxScore
	filled := int(rel * 5)
	if filled < 1 && score > 0 {
		filled = 1
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 5-filled)
	return bar
}

// nodeResponsesToScoredNodes converts NodeResponse slice to ScoredNode slice
// for reuse with the existing renderScoredNodePanel renderer.
func nodeResponsesToScoredNodes(responses []knowledge.NodeResponse) []knowledge.ScoredNode {
	scored := make([]knowledge.ScoredNode, len(responses))
	for i, r := range responses {
		scored[i] = knowledge.ScoredNode{
			Node: &memory.Node{
				ID:          r.ID,
				Type:        r.Type,
				Summary:     r.Summary,
				Content:     r.Content,
				SourceAgent: "", // Not available in NodeResponse
			},
			Score: r.MatchScore,
		}
	}
	return scored
}

// symbolKindIcon returns an icon for a symbol kind.
func symbolKindIcon(kind string) string {
	switch kind {
	case "function", "method":
		return "ƒ"
	case "struct", "class":
		return "⬡"
	case "interface":
		return "◇"
	case "type":
		return "τ"
	case "constant":
		return "π"
	case "variable":
		return "ν"
	case "package", "module":
		return "📦"
	case "field":
		return "·"
	case "decorator":
		return "@"
	case "macro":
		return "#"
	default:
		return "○"
	}
}
