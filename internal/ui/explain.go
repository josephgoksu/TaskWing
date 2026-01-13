// Package ui provides rendering for explain results.
package ui

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/app"
)

// RenderExplainResult renders a deep explanation to the terminal.
func RenderExplainResult(result *app.ExplainResult, verbose bool) {
	// Symbol header
	fmt.Printf("\n%s Symbol: %s (%s)\n", StyleBold("🔍"), result.Symbol.Name, result.Symbol.Kind)
	fmt.Printf("   Location: %s\n", result.Symbol.Location)
	if result.Symbol.Signature != "" {
		fmt.Printf("   Signature: %s\n", result.Symbol.Signature)
	}
	if verbose && result.Symbol.DocComment != "" {
		fmt.Printf("   Doc: %s\n", truncate(result.Symbol.DocComment, 100))
	}

	// Call graph
	fmt.Printf("\n%s System Context\n", StyleBold("📊"))
	fmt.Println("───────────────")

	// Callers
	fmt.Printf("\n⬆️  Called By (%d):\n", len(result.Callers))
	if len(result.Callers) == 0 {
		fmt.Println("   (no callers found - this may be an entry point)")
	} else {
		for i, c := range result.Callers {
			if i >= 5 && !verbose {
				fmt.Printf("   ... and %d more (use --verbose to see all)\n", len(result.Callers)-5)
				break
			}
			fmt.Printf("   %d. %s → %s\n", i+1, c.Symbol.Name, c.Symbol.Location)
		}
	}

	// Callees
	fmt.Printf("\n⬇️  Calls (%d):\n", len(result.Callees))
	if len(result.Callees) == 0 {
		fmt.Println("   (no callees found - this may be a leaf function)")
	} else {
		for i, c := range result.Callees {
			if i >= 5 && !verbose {
				fmt.Printf("   ... and %d more (use --verbose to see all)\n", len(result.Callees)-5)
				break
			}
			fmt.Printf("   %d. %s → %s\n", i+1, c.Symbol.Name, c.Symbol.Location)
		}
	}

	// Impact stats
	fmt.Printf("\n%s Impact Analysis:\n", StyleBold("🔗"))
	fmt.Printf("   Direct callers: %d\n", result.ImpactStats.DirectCallers)
	fmt.Printf("   Direct callees: %d\n", result.ImpactStats.DirectCallees)
	if result.ImpactStats.TransitiveDependents > 0 {
		fmt.Printf("   Transitive dependents: %d (depth %d)\n",
			result.ImpactStats.TransitiveDependents, result.ImpactStats.MaxDepthReached)
	}
	if result.ImpactStats.AffectedFiles > 0 {
		fmt.Printf("   Files affected: %d\n", result.ImpactStats.AffectedFiles)
	}

	// Related decisions
	if len(result.Decisions) > 0 {
		fmt.Printf("\n%s Related Decisions:\n", StyleBold("📋"))
		for _, d := range result.Decisions {
			fmt.Printf("   • %s\n", d.Summary)
		}
	}

	// Related patterns
	if len(result.Patterns) > 0 {
		fmt.Printf("\n%s Related Patterns:\n", StyleBold("📐"))
		for _, p := range result.Patterns {
			fmt.Printf("   • %s\n", p.Summary)
		}
	}

	// Source code (only if verbose)
	if verbose && len(result.SourceCode) > 0 {
		fmt.Printf("\n%s Source Code:\n", StyleBold("📁"))
		for _, snippet := range result.SourceCode {
			fmt.Printf("\n   %s %s (%s):\n", snippet.Kind, snippet.SymbolName, snippet.FilePath)
			// Indent code
			lines := strings.Split(snippet.Content, "\n")
			for _, line := range lines[:min(10, len(lines))] {
				fmt.Printf("   %s\n", line)
			}
			if len(lines) > 10 {
				fmt.Printf("   ... (%d more lines)\n", len(lines)-10)
			}
		}
	}

	// Explanation (if not already streamed)
	// Note: When streaming is used, the explanation is printed inline
	// and result.Explanation will be set but we skip rendering it here
	// The caller handles this by passing an empty explanation when streaming
}

// RenderExplainHeader renders the header before streaming starts.
func RenderExplainHeader(symbolName string) {
	fmt.Printf("\n%s Analyzing: %s\n", StyleBold("🔍"), symbolName)
	fmt.Println("───────────────────────────")
}

// RenderExplainExplanation renders just the explanation section.
func RenderExplainExplanation(explanation string) {
	if explanation == "" {
		return
	}
	fmt.Printf("\n%s Explanation:\n", StyleBold("💬"))
	// Wrap text at 80 chars with indent
	wrapped := wrapText(explanation, 76)
	for _, line := range strings.Split(wrapped, "\n") {
		fmt.Printf("   %s\n", line)
	}
}

// wrapText wraps text at the specified width.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)

		if lineLen+wordLen+1 > width && lineLen > 0 {
			result.WriteString("\n")
			lineLen = 0
		} else if i > 0 && lineLen > 0 {
			result.WriteString(" ")
			lineLen++
		}

		result.WriteString(word)
		lineLen += wordLen
	}

	return result.String()
}

// truncate shortens a string to max length with ellipsis.
func truncate(s string, max int) string {
	// Remove newlines for display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)

	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// StyleBold returns the text with bold ANSI codes.
// This is a simple implementation - could use lipgloss for more styling.
func StyleBold(s string) string {
	return "\033[1m" + s + "\033[0m"
}
