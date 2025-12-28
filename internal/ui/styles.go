package ui

import "github.com/charmbracelet/lipgloss"

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
)

// Icon returns a styled icon string
func Icon(icon string, style lipgloss.Style) string {
	return style.Render(icon)
}
