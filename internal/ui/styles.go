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

	// Styles
	StyleTitle   = lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	StyleSubtle  = lipgloss.NewStyle().Foreground(ColorSecondary)
	StylePrimary = lipgloss.NewStyle().Foreground(ColorPrimary)
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)

	// Components
	StyleHeader = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 1). // Add some breathing room if needed
			MarginBottom(1)

	StyleSectionTitle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				Underline(true).
				MarginBottom(1)
)

// Icon returns a styled icon string
func Icon(icon string, style lipgloss.Style) string {
	return style.Render(icon)
}
