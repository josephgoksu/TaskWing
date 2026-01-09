package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTable_ColumnWidths(t *testing.T) {
	table := &Table{
		Headers: []string{"ID", "Name", "Status"},
		Rows: [][]string{
			{"abc123", "First item", "active"},
			{"def456", "Second item with longer name", "pending"},
		},
	}

	widths := table.ColumnWidths()

	assert.Equal(t, 6, widths[0])  // "abc123" is longest in first column
	assert.Equal(t, 28, widths[1]) // "Second item with longer name"
	assert.Equal(t, 7, widths[2])  // "pending" is longest
}

func TestTable_ColumnWidths_MaxWidth(t *testing.T) {
	table := &Table{
		Headers:  []string{"ID", "Description"},
		Rows:     [][]string{{"a", "This is a very long description that should be truncated"}},
		MaxWidth: 20,
	}

	widths := table.ColumnWidths()

	assert.Equal(t, 2, widths[0])  // "ID" is longest
	assert.Equal(t, 20, widths[1]) // Capped at MaxWidth
}

func TestTable_Render(t *testing.T) {
	table := &Table{
		Headers: []string{"ID", "Name"},
		Rows: [][]string{
			{"1", "Alice"},
			{"2", "Bob"},
		},
	}

	output := table.Render()

	// Should contain headers and rows (with ANSI codes)
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "Bob")
	// Should contain separator line
	assert.Contains(t, output, "─")
}

func TestTable_Render_Empty(t *testing.T) {
	table := &Table{
		Headers: []string{},
		Rows:    [][]string{},
	}

	output := table.Render()
	assert.Empty(t, output)
}

func TestTable_Render_Truncation(t *testing.T) {
	table := &Table{
		Headers:  []string{"Text"},
		Rows:     [][]string{{"This is way too long"}},
		MaxWidth: 10,
	}

	output := table.Render()

	// Should contain truncation indicator
	assert.Contains(t, output, "…")
}

func TestTruncateID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc123def456", "abc123"},
		{"short", "short"},
		{"abc", "abc"},
		{"", ""},
	}

	for _, tc := range tests {
		result := TruncateID(tc.input)
		assert.Equal(t, tc.expected, result)
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"abc", 5, "abc  "},
		{"hello", 5, "hello"},
		{"longer", 3, "longer"},
		{"", 3, "   "},
	}

	for _, tc := range tests {
		result := padRight(tc.input, tc.width)
		assert.Equal(t, tc.expected, result)
	}
}

func TestTable_Render_RowsHaveFewerColumns(t *testing.T) {
	table := &Table{
		Headers: []string{"ID", "Name", "Status"},
		Rows: [][]string{
			{"1", "Alice"}, // Missing Status column
		},
	}

	output := table.Render()

	// Should not panic and should render what's available
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "Alice")
	// Count lines - should have header, separator, and 1 data row
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 3, len(lines))
}
