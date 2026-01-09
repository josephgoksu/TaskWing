package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Table renders data in a compact markdown-style table format.
// This is optimized for terminal display with fixed-width columns.
type Table struct {
	Headers  []string
	Rows     [][]string
	MaxWidth int // Max width per column (0 = auto)
}

// ColumnWidths calculates optimal column widths based on content.
func (t *Table) ColumnWidths() []int {
	widths := make([]int, len(t.Headers))

	// Start with header widths
	for i, h := range t.Headers {
		widths[i] = len(h)
	}

	// Expand for content
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Apply max width constraint
	if t.MaxWidth > 0 {
		for i := range widths {
			if widths[i] > t.MaxWidth {
				widths[i] = t.MaxWidth
			}
		}
	}

	return widths
}

// Render outputs the table to a string.
func (t *Table) Render() string {
	if len(t.Headers) == 0 {
		return ""
	}

	widths := t.ColumnWidths()
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	cellStyle := lipgloss.NewStyle().Foreground(ColorText)
	dimStyle := lipgloss.NewStyle().Foreground(ColorSecondary)

	// Header row
	var headerCells []string
	for i, h := range t.Headers {
		headerCells = append(headerCells, headerStyle.Render(padRight(h, widths[i])))
	}
	sb.WriteString(" " + strings.Join(headerCells, "  ") + "\n")

	// Separator
	var sepParts []string
	for _, w := range widths {
		sepParts = append(sepParts, dimStyle.Render(strings.Repeat("─", w)))
	}
	sb.WriteString(" " + strings.Join(sepParts, "──") + "\n")

	// Data rows
	for _, row := range t.Rows {
		var cells []string
		for i := range t.Headers {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			// Truncate if needed (guard against zero/small widths)
			if widths[i] >= 2 && len(val) > widths[i] {
				val = val[:widths[i]-1] + "…"
			} else if widths[i] == 1 && len(val) > 1 {
				val = "…"
			}
			cells = append(cells, cellStyle.Render(padRight(val, widths[i])))
		}
		sb.WriteString(" " + strings.Join(cells, "  ") + "\n")
	}

	return sb.String()
}

// padRight pads a string to the specified width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// TruncateID shortens an ID for display (first 6 chars).
func TruncateID(id string) string {
	if len(id) > 6 {
		return id[:6]
	}
	return id
}
