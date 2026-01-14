package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// IsInteractive checks if stdout is a terminal.
// This is useful to avoid prompting when piping output or running in non-interactive environments.
func IsInteractive() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// RenderPageHeader displays a consistent styled header for commands
func RenderPageHeader(title, subtitle string) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		MarginBottom(1)

	fmt.Println(titleStyle.Render(fmt.Sprintf("🤖 %s", title)))
	if subtitle != "" {
		fmt.Printf("  ⚡  %s\n", subtitle)
	}
}

// Panel represents a styled panel with optional title and content.
// Similar to Python's rich.Panel for displaying boxed content.
type Panel struct {
	Title       string
	Content     string
	BorderColor lipgloss.Color
	Width       int
}

// NewPanel creates a new panel with default styling.
func NewPanel(title, content string) *Panel {
	return &Panel{
		Title:       title,
		Content:     content,
		BorderColor: ColorSecondary,
		Width:       0, // auto
	}
}

// WithBorderColor sets the border color and returns the panel.
func (p *Panel) WithBorderColor(color lipgloss.Color) *Panel {
	p.BorderColor = color
	return p
}

// WithWidth sets the panel width and returns the panel.
func (p *Panel) WithWidth(width int) *Panel {
	p.Width = width
	return p
}

// Render returns the styled panel as a string.
func (p *Panel) Render() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.BorderColor).
		Padding(0, 1)

	if p.Width > 0 {
		style = style.Width(p.Width)
	}

	var content string
	if p.Title != "" {
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
		content = titleStyle.Render(p.Title) + "\n" + p.Content
	} else {
		content = p.Content
	}

	return style.Render(content)
}

// RenderPanel is a convenience function to create and render a panel.
func RenderPanel(title, content string) string {
	return NewPanel(title, content).Render()
}

// RenderInfoPanel renders a panel with info styling (cyan border).
func RenderInfoPanel(title, content string) string {
	return NewPanel(title, content).WithBorderColor(ColorCyan).Render()
}

// RenderSuccessPanel renders a panel with success styling (green border).
func RenderSuccessPanel(title, content string) string {
	return NewPanel(title, content).WithBorderColor(ColorSuccess).Render()
}

// RenderErrorPanel renders a panel with error styling (red border).
func RenderErrorPanel(title, content string) string {
	return NewPanel(title, content).WithBorderColor(ColorError).Render()
}

// RenderWarningPanel renders a panel with warning styling (yellow border).
func RenderWarningPanel(title, content string) string {
	return NewPanel(title, content).WithBorderColor(ColorWarning).Render()
}

// Truncate truncates a string to maxLen characters, adding ellipsis if needed.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// WrapText wraps text to the specified width.
func WrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		if len(line) <= width {
			result.WriteString(line)
			continue
		}

		// Simple word-wrap
		words := strings.Fields(line)
		currentLine := ""
		for _, word := range words {
			if currentLine == "" {
				currentLine = word
			} else if len(currentLine)+1+len(word) <= width {
				currentLine += " " + word
			} else {
				result.WriteString(currentLine + "\n")
				currentLine = word
			}
		}
		if currentLine != "" {
			result.WriteString(currentLine)
		}
	}

	return result.String()
}
