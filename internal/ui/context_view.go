package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/app"
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

// renderContextWithSymbolsInternal displays knowledge results and code symbols.
func renderContextWithSymbolsInternal(query string, scored []knowledge.ScoredNode, symbols []app.SymbolResponse, answer string, verbose bool) {
	// Styles
	var (
		cardStyle    = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).Padding(0, 2).MarginTop(1).BorderForeground(lipgloss.Color("63"))
		titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
		sectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
		sourceTitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		metaStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		locationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		barFull      = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		barEmpty     = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	)

	// Render Answer Summary
	if answer != "" {
		fmt.Println()
		fmt.Println(titleStyle.Render(fmt.Sprintf("📖 %s", query)))
		fmt.Println(cardStyle.Render(answer))
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
			relativeScore := s.Score / maxScore
			barSegments := int(relativeScore * 10)
			if barSegments < 1 && s.Score > 0 {
				barSegments = 1
			}
			if barSegments > 10 {
				barSegments = 10
			}
			bar := barFull.Render(strings.Repeat("━", barSegments)) + barEmpty.Render(strings.Repeat("━", 10-barSegments))

			summary := s.Node.Summary
			if summary == "" {
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

			icon := TypeIcon(s.Node.Type)
			expandedIndicator := ""
			if s.ExpandedFrom != "" {
				expandedIndicator = " 🔗"
			}

			if verbose {
				fmt.Printf(" %d. %s %s\n", i+1, icon, sourceTitle.Render(summary))
				fmt.Printf("    %s\n", metaStyle.Render(fmt.Sprintf("ID: %s | Type: %s | Score: %.2f%s", id, s.Node.Type, s.Score, expandedIndicator)))
				if s.Node.SourceAgent != "" {
					fmt.Printf("    %s\n", metaStyle.Render(fmt.Sprintf("Agent: %s", s.Node.SourceAgent)))
				}
			} else {
				fmt.Printf(" %d. %s %s %s%s\n", i+1, icon, sourceTitle.Render(summary), metaStyle.Render(fmt.Sprintf("[%s %s]", bar, id)), expandedIndicator)
			}
		}
	}

	// Render Code Symbols section
	if len(symbols) > 0 {
		fmt.Println()
		fmt.Println(sectionStyle.Render("💻 Code Symbols"))

		for i, sym := range symbols {
			icon := symbolKindIcon(sym.Kind)
			visibilityMark := ""
			if sym.Visibility == "private" {
				visibilityMark = metaStyle.Render(" (private)")
			}

			name := sym.Name
			if sym.Signature != "" && !verbose {
				// Show short signature in compact mode
				if len(sym.Signature) > 40 {
					name = sym.Name + sym.Signature[:40] + "..."
				} else {
					name = sym.Name + sym.Signature
				}
			}

			location := locationStyle.Render(sym.Location)

			if verbose {
				fmt.Printf(" %d. %s %s%s\n", i+1, icon, sourceTitle.Render(sym.Name), visibilityMark)
				if sym.Signature != "" {
					fmt.Printf("    %s\n", metaStyle.Render(fmt.Sprintf("Signature: %s", sym.Signature)))
				}
				fmt.Printf("    %s\n", metaStyle.Render(fmt.Sprintf("Kind: %s | Language: %s", sym.Kind, sym.Language)))
				fmt.Printf("    %s\n", location)
				if sym.DocComment != "" {
					// Show first line of doc comment
					doc := sym.DocComment
					if idx := strings.Index(doc, "\n"); idx > 0 {
						doc = doc[:idx]
					}
					if len(doc) > 80 {
						doc = doc[:80] + "..."
					}
					fmt.Printf("    %s\n", metaStyle.Render(fmt.Sprintf("Doc: %s", doc)))
				}
			} else {
				fmt.Printf(" %d. %s %s %s%s\n", i+1, icon, sourceTitle.Render(name), location, visibilityMark)
			}
		}
	}
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
