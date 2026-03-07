// Package ui provides centralized styling for all terminal output.
//
// Convention: All colors use lipgloss.AdaptiveColor for light/dark theme support.
// Use the Color* variables (not lipgloss.Color directly) in cmd/ files.
// Use Icon* constants from icons.go instead of raw emoji strings.
// Use Print* helpers from output.go instead of raw fmt.Printf with emojis.
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors — AdaptiveColor picks Light on light backgrounds, Dark on dark.
	// lipgloss.TerminalColor is the interface both Color and AdaptiveColor implement.
	ColorPrimary   lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "125", Dark: "205"}
	ColorSecondary lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "244", Dark: "241"}
	ColorSuccess   lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "28", Dark: "42"}
	ColorError     lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "160", Dark: "160"}
	ColorWarning   lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "208", Dark: "214"}
	ColorText      lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
	ColorCyan      lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "30", Dark: "87"}
	ColorBlue      lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "27", Dark: "75"}
	ColorHighlight lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "21", Dark: "12"}
	ColorSelected  lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "28", Dark: "10"}
	ColorDim       lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "246", Dark: "240"}
	ColorAccent    lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "97", Dark: "141"}
	ColorBarEmpty  lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "250", Dark: "237"}
	ColorYellow    lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "136", Dark: "11"}

	// Base Styles
	StyleTitle   = lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	StyleSubtle  = lipgloss.NewStyle().Foreground(ColorSecondary)
	StylePrimary = lipgloss.NewStyle().Foreground(ColorPrimary)
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleText    = lipgloss.NewStyle().Foreground(ColorText)

	// Input Box Style for textarea border
	StyleInputBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorSecondary).
			Padding(0, 1)

	// Ready state style (green accent)
	StyleReadyBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorSuccess).
			Padding(0, 1)

	// Strategy Box - distinct box for "AI thinking" research strategy
	StyleStrategyBox = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(ColorCyan).
				Padding(0, 1)

	// Answer Box - for auto-generated answers
	StyleAnswerBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBlue).
			Padding(0, 1)

	// Components
	StyleHeader = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	StyleSectionTitle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				Underline(true)

	// Semantic Prefix Styles
	StylePrefixThinking = lipgloss.NewStyle().Foreground(ColorSecondary)          // Dim for progress
	StylePrefixStrategy = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)    // Bright for strategy
	StylePrefixQuestion = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true) // Orange for questions
	StylePrefixDone     = lipgloss.NewStyle().Foreground(ColorSuccess)            // Green for done
	StylePrefixWarn     = lipgloss.NewStyle().Foreground(ColorWarning)            // Orange for warnings
	StylePrefixError    = lipgloss.NewStyle().Foreground(ColorError).Bold(true)   // Red for errors
	StylePrefixAgent    = lipgloss.NewStyle().Foreground(ColorPrimary)            // Pink for agent
	StylePrefixUser     = lipgloss.NewStyle().Foreground(ColorSuccess)            // Green for user
	StylePrefixAnswer   = lipgloss.NewStyle().Foreground(ColorBlue).Bold(true)    // Blue for answers

	// Selection List Styles (for provider/model selection)
	StyleSelectTitle  = lipgloss.NewStyle().Bold(true).Foreground(ColorHighlight)
	StyleSelectNormal = lipgloss.NewStyle().Foreground(ColorText)
	StyleSelectActive = lipgloss.NewStyle().Foreground(ColorSelected).Bold(true)
	StyleSelectDim    = lipgloss.NewStyle().Foreground(ColorDim)
	StyleSelectBadge  = lipgloss.NewStyle().Foreground(ColorYellow).Bold(true)

	// Table Styles (alternating rows)
	ColorTableRowEven lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "255", Dark: "236"}
	ColorTableRowOdd  lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "253", Dark: "234"}
	StyleTableRowEven = lipgloss.NewStyle().Foreground(ColorText)
	StyleTableRowOdd  = lipgloss.NewStyle().Foreground(ColorDim)
	StyleTableHeader  = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Underline(true)

	// Doctor Check Styles
	StyleCheckOK   = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleCheckWarn = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	StyleCheckFail = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	StyleCheckName = lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	StyleCheckHint = lipgloss.NewStyle().Foreground(ColorDim).Italic(true)

	// Ask Output Styles
	StyleAskHeader     = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Padding(0, 0)
	StyleAskMeta       = lipgloss.NewStyle().Foreground(ColorDim)
	StyleCitationPath  = lipgloss.NewStyle().Foreground(ColorDim).Italic(true)
	StyleCitationBadge = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
)

// CategoryBadge returns a styled badge string for a knowledge node type.
func CategoryBadge(nodeType string) string {
	colors := map[string]lipgloss.TerminalColor{
		"decision":      lipgloss.AdaptiveColor{Light: "125", Dark: "205"},
		"feature":       lipgloss.AdaptiveColor{Light: "27", Dark: "75"},
		"constraint":    lipgloss.AdaptiveColor{Light: "208", Dark: "214"},
		"pattern":       lipgloss.AdaptiveColor{Light: "97", Dark: "141"},
		"plan":          lipgloss.AdaptiveColor{Light: "28", Dark: "42"},
		"note":          lipgloss.AdaptiveColor{Light: "235", Dark: "252"},
		"metadata":      lipgloss.AdaptiveColor{Light: "30", Dark: "87"},
		"documentation": lipgloss.AdaptiveColor{Light: "136", Dark: "11"},
	}

	color, ok := colors[nodeType]
	if !ok {
		color = lipgloss.AdaptiveColor{Light: "244", Dark: "241"}
	}

	badge := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "255", Dark: "0"}).
		Background(color).
		Padding(0, 1).
		Bold(true)

	return badge.Render(strings.ToUpper(nodeType[:1]) + nodeType[1:])
}
