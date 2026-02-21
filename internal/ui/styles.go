package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	ColorPrimary   = lipgloss.Color("205") // Pink
	ColorSecondary = lipgloss.Color("241") // Gray
	ColorSuccess   = lipgloss.Color("42")  // Green
	ColorError     = lipgloss.Color("160") // Red
	ColorWarning   = lipgloss.Color("214") // Orange/Yellow
	ColorText      = lipgloss.Color("252") // White/Gray
	ColorCyan      = lipgloss.Color("87")  // Cyan for strategy
	ColorBlue      = lipgloss.Color("75")  // Blue for answers
	ColorHighlight = lipgloss.Color("12")  // Blue for titles/highlights
	ColorSelected  = lipgloss.Color("10")  // Green for selected items
	ColorDim       = lipgloss.Color("240") // Dim gray for secondary text
	ColorYellow    = lipgloss.Color("11")  // Yellow for badges/accents

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
	ColorTableRowEven = lipgloss.Color("236") // Subtle dark background
	ColorTableRowOdd  = lipgloss.Color("234") // Slightly darker
	StyleTableRowEven = lipgloss.NewStyle().Foreground(ColorText)
	StyleTableRowOdd  = lipgloss.NewStyle().Foreground(ColorText).Background(ColorTableRowOdd)
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
	colors := map[string]lipgloss.Color{
		"decision":      lipgloss.Color("205"), // Pink
		"feature":       lipgloss.Color("75"),  // Blue
		"constraint":    lipgloss.Color("214"), // Orange
		"pattern":       lipgloss.Color("141"), // Purple
		"plan":          lipgloss.Color("42"),  // Green
		"note":          lipgloss.Color("252"), // White
		"metadata":      lipgloss.Color("87"),  // Cyan
		"documentation": lipgloss.Color("11"),  // Yellow
	}

	color, ok := colors[nodeType]
	if !ok {
		color = lipgloss.Color("241")
	}

	badge := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(color).
		Padding(0, 1).
		Bold(true)

	return badge.Render(strings.ToUpper(nodeType[:1]) + nodeType[1:])
}
