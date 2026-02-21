package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

const (
	// contentDisplayRelativeThreshold is the minimum relative score (vs max) to show content
	contentDisplayRelativeThreshold float32 = 0.7
	// contentDisplayAbsoluteThreshold is the minimum absolute score to show content
	// This prevents showing content for low-relevance results even if they're "best" in set
	contentDisplayAbsoluteThreshold float32 = 0.25
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

// RenderContextResultsWithSymbols displays both knowledge results and code symbols.
func RenderContextResultsWithSymbols(query string, scored []knowledge.ScoredNode, symbols []app.SymbolResponse, answer string) {
	renderContextWithSymbolsInternal(query, scored, symbols, answer, false)
}

// RenderContextResultsWithSymbolsVerbose displays knowledge and symbols with full metadata.
func RenderContextResultsWithSymbolsVerbose(query string, scored []knowledge.ScoredNode, symbols []app.SymbolResponse, answer string) {
	renderContextWithSymbolsInternal(query, scored, symbols, answer, true)
}

func renderContextInternal(query string, scored []knowledge.ScoredNode, answer string, verbose bool) {
	// Styles
	var (
		titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
		sectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	)

	// Render Answer Panel
	if answer != "" {
		fmt.Println()
		fmt.Println(titleStyle.Render(fmt.Sprintf("📖 %s", query)))
		fmt.Println(RenderInfoPanel("Answer", answer))
	} else {
		fmt.Println(titleStyle.Render(fmt.Sprintf("🔍 Context for: \"%s\"", query)))
	}

	// Render Sources
	fmt.Println()
	if answer != "" {
		fmt.Println(sectionStyle.Render("📚 Sources"))
	}

	// Calculate max score for relative scaling
	var maxScore float32 = 0.01
	for _, s := range scored {
		if s.Score > maxScore {
			maxScore = s.Score
		}
	}

	// Render each result in a Panel
	for i, s := range scored {
		renderScoredNodePanel(i+1, s, maxScore, verbose)
	}
}

// renderScoredNodePanel renders a single knowledge result as a styled panel.
func renderScoredNodePanel(index int, s knowledge.ScoredNode, maxScore float32, verbose bool) {
	// Styles
	var (
		headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true) // Cyan for headers
		metaStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))           // Dim for metadata
		contentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))           // Light for content
		barFull      = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))            // Green
		barEmpty     = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))           // Dark gray
		panelBorder  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(scoreToColor(s.Score, maxScore)).Padding(0, 1).MarginTop(1)
	)

	// Calculate score bar
	relativeScore := s.Score / maxScore
	barSegments := int(relativeScore * 10)
	if barSegments < 1 && s.Score > 0 {
		barSegments = 1
	}
	if barSegments > 10 {
		barSegments = 10
	}
	bar := barFull.Render(strings.Repeat("━", barSegments)) + barEmpty.Render(strings.Repeat("━", 10-barSegments))

	// Build summary
	summary := s.Node.Summary
	if summary == "" {
		runes := []rune(s.Node.Text())
		if len(runes) > 60 {
			summary = string(runes[:60]) + "..."
		} else {
			summary = string(runes)
		}
	}

	// Build ID
	id := s.Node.ID
	if len(id) > 8 {
		id = id[:8]
	}

	// Icon
	icon := TypeIcon(s.Node.Type)

	// Graph expansion indicator
	expandedIndicator := ""
	if s.ExpandedFrom != "" {
		expandedIndicator = " 🔗"
	}

	// Build panel content
	var content strings.Builder

	// Header line: Type icon and summary
	content.WriteString(headerStyle.Render(fmt.Sprintf("%s %s", icon, summary)))
	content.WriteString(expandedIndicator)
	content.WriteString("\n")

	// Metadata line: Score bar, ID, Type
	content.WriteString(metaStyle.Render(fmt.Sprintf("Score: %s %.2f  │  Source: %s  │  Type: %s",
		bar, s.Score, id, s.Node.Type)))

	if verbose {
		// Additional metadata in verbose mode
		if s.Node.SourceAgent != "" {
			content.WriteString("\n")
			content.WriteString(metaStyle.Render(fmt.Sprintf("Agent: %s", s.Node.SourceAgent)))
		}
	}

	// Content section for high-scoring results
	cleanContent := getContentWithoutSummary(s.Node.Text(), summary)
	showContent := relativeScore > contentDisplayRelativeThreshold &&
		s.Score > contentDisplayAbsoluteThreshold &&
		cleanContent != ""

	if showContent {
		maxLen := 150
		if verbose {
			maxLen = 300
		}
		truncated := truncateContent(cleanContent, maxLen)
		content.WriteString("\n")
		content.WriteString(metaStyle.Render("─────────────────────────────────────────────────"))
		content.WriteString("\n")
		content.WriteString(contentStyle.Render(truncated))
	}

	fmt.Printf(" %d. ", index)
	fmt.Println(panelBorder.Render(content.String()))
}

// scoreToColor returns a border color based on the score (green for high, yellow for medium, gray for low).
func scoreToColor(score, maxScore float32) lipgloss.Color {
	relative := score / maxScore
	switch {
	case relative >= 0.8:
		return lipgloss.Color("42") // Green - high relevance
	case relative >= 0.5:
		return lipgloss.Color("214") // Orange - medium relevance
	default:
		return lipgloss.Color("241") // Gray - lower relevance
	}
}

// renderContextWithSymbolsInternal displays knowledge results and code symbols.
func renderContextWithSymbolsInternal(query string, scored []knowledge.ScoredNode, symbols []app.SymbolResponse, answer string, verbose bool) {
	// Styles
	var (
		titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
		sectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	)

	// Render Answer Panel
	if answer != "" {
		fmt.Println()
		fmt.Println(titleStyle.Render(fmt.Sprintf("📖 %s", query)))
		fmt.Println(RenderInfoPanel("Answer", answer))
	} else {
		fmt.Println(titleStyle.Render(fmt.Sprintf("🔍 Context for: \"%s\"", query)))
	}

	// Render Knowledge Results section
	if len(scored) > 0 {
		fmt.Println()
		if answer != "" {
			fmt.Println(sectionStyle.Render("📚 Knowledge"))
		} else {
			fmt.Println(sectionStyle.Render("📚 Architectural Knowledge"))
		}

		// Calculate max score for relative scaling
		var maxScore float32 = 0.01
		for _, s := range scored {
			if s.Score > maxScore {
				maxScore = s.Score
			}
		}

		for i, s := range scored {
			renderScoredNodePanel(i+1, s, maxScore, verbose)
		}
	}

	// Render Code Symbols section
	if len(symbols) > 0 {
		fmt.Println()
		fmt.Println(sectionStyle.Render("💻 Code Symbols"))

		for i, sym := range symbols {
			renderSymbolPanel(i+1, sym, verbose)
		}
	}
}

// renderSymbolPanel renders a code symbol as a styled panel.
func renderSymbolPanel(index int, sym app.SymbolResponse, verbose bool) {
	// Styles
	var (
		headerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
		metaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		locationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		panelBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(0, 1).MarginTop(1)
	)

	icon := symbolKindIcon(sym.Kind)
	visibilityMark := ""
	if sym.Visibility == "private" {
		visibilityMark = " (private)"
	}

	// Build panel content
	var content strings.Builder

	// Header line: Icon, name, visibility
	content.WriteString(headerStyle.Render(fmt.Sprintf("%s %s", icon, sym.Name)))
	content.WriteString(metaStyle.Render(visibilityMark))
	content.WriteString("\n")

	// Metadata line: Kind, Language, Location
	content.WriteString(metaStyle.Render(fmt.Sprintf("Kind: %s  │  Language: %s  │  ", sym.Kind, sym.Language)))
	content.WriteString(locationStyle.Render(sym.Location))

	if verbose {
		// Additional metadata in verbose mode
		if sym.Signature != "" {
			content.WriteString("\n")
			content.WriteString(metaStyle.Render(fmt.Sprintf("Signature: %s", truncateContent(sym.Signature, 60))))
		}
		if sym.DocComment != "" {
			doc := sym.DocComment
			if idx := strings.Index(doc, "\n"); idx > 0 {
				doc = doc[:idx]
			}
			doc = truncateContent(doc, 80)
			content.WriteString("\n")
			content.WriteString(metaStyle.Render("─────────────────────────────────────────────────"))
			content.WriteString("\n")
			content.WriteString(metaStyle.Render(doc))
		}
	}

	fmt.Printf(" %d. ", index)
	fmt.Println(panelBorder.Render(content.String()))
}

// truncateContent truncates content to maxLen runes and adds ellipsis if needed.
func truncateContent(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen]) + "..."
}

// getContentWithoutSummary returns content with the summary prefix removed.
// Many knowledge nodes have Content that starts with Summary text - this avoids redundancy.
func getContentWithoutSummary(content, summary string) string {
	// Guard: if summary is empty or too short, return content as-is
	// This prevents CutPrefix("anything", "") always matching
	if len(summary) < 3 {
		return content
	}
	// Check if content starts with summary and remove it
	if remainder, found := strings.CutPrefix(content, summary); found {
		// Remove leading newlines/whitespace
		return strings.TrimLeft(remainder, "\n\r\t ")
	}
	return content
}

// RenderAskResult displays a complete AskResult from the ask pipeline.
// This is the primary rendering function for the `taskwing ask` command.
func RenderAskResult(result *app.AskResult, verbose bool) {
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)

	// Header with query in a styled box
	headerBox := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary)

	fmt.Println()
	if result.Answer != "" {
		fmt.Println(headerBox.Render(fmt.Sprintf("Q: %s", result.Query)))
	} else {
		fmt.Println(headerBox.Render(fmt.Sprintf("Search: %s", result.Query)))
	}

	// Pipeline & rewrite as dim metadata
	var metaParts []string
	metaParts = append(metaParts, result.Pipeline)
	if result.RewrittenQuery != "" {
		metaParts = append(metaParts, fmt.Sprintf("rewritten: %s", result.RewrittenQuery))
	}
	if result.Total > 0 || result.TotalSymbols > 0 {
		metaParts = append(metaParts, fmt.Sprintf("%d knowledge, %d symbols", result.Total, result.TotalSymbols))
	}
	fmt.Println(StyleAskMeta.Render("  " + strings.Join(metaParts, " | ")))

	// Warning
	if result.Warning != "" {
		fmt.Println()
		fmt.Println(RenderWarningPanel("Warning", result.Warning))
	}

	// Answer box with accent border
	if result.Answer != "" {
		fmt.Println()
		answerBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBlue).
			Padding(1, 2).
			Width(80)
		fmt.Println(answerBox.Render(result.Answer))
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
