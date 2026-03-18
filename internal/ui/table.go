package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
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

	// Auto-constrain to terminal width when MaxWidth is not set
	if t.MaxWidth == 0 {
		termWidth := GetTerminalWidth()
		// Account for leading space + column separators (2 chars between each column)
		overhead := 1
		if len(widths) > 1 {
			overhead += (len(widths) - 1) * 2
		}
		available := termWidth - overhead
		if available > 0 {
			total := 0
			for _, w := range widths {
				total += w
			}
			if total > available {
				// Proportionally shrink columns, but keep a minimum of 4 chars
				ratio := float64(available) / float64(total)
				for i := range widths {
					newW := int(float64(widths[i]) * ratio)
					if newW < 4 {
						newW = 4
					}
					widths[i] = newW
				}
				// Post-clamp: if min-floor caused overflow, trim widest columns
				for {
					postTotal := 0
					for _, w := range widths {
						postTotal += w
					}
					excess := postTotal - available
					if excess <= 0 {
						break
					}
					// Find widest column and shrink it
					maxIdx, maxW := 0, 0
					for i, w := range widths {
						if w > maxW {
							maxIdx, maxW = i, w
						}
					}
					// Don't shrink below minimum
					if maxW <= 4 {
						break
					}
					shrink := excess
					if shrink > maxW-4 {
						shrink = maxW - 4
					}
					widths[maxIdx] -= shrink
				}
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
			// Truncate if needed using display width (rune-safe)
			if lipgloss.Width(val) > widths[i] {
				val = truncateToWidth(val, widths[i])
			}
			cells = append(cells, cellStyle.Render(padRight(val, widths[i])))
		}
		sb.WriteString(" " + strings.Join(cells, "  ") + "\n")
	}

	return sb.String()
}

// padRight pads a string to the specified display width.
// Uses lipgloss.Width for correct handling of multi-byte and wide characters.
func padRight(s string, width int) string {
	visible := lipgloss.Width(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// GetTerminalWidthFor returns the terminal width for the given file descriptor, defaulting to 80.
func GetTerminalWidthFor(f *os.File) int {
	w, _, err := term.GetSize(int(f.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// GetTerminalWidth returns the current stdout terminal width, defaulting to 80.
func GetTerminalWidth() int {
	return GetTerminalWidthFor(os.Stdout)
}

// truncateToWidth truncates a string to fit within maxWidth display columns,
// appending "..." if truncated. Safe for multi-byte and wide characters.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i]) + "..."
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return s[:maxWidth]
}

// TruncateID shortens an ID for display (first 6 chars).
func TruncateID(id string) string {
	if len(id) > 6 {
		return id[:6]
	}
	return id
}
