package ui

import (
	"fmt"
	"os"

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
