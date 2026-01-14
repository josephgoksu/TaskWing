package ui

import (
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"empty", "", 10, ""},
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 3, "hel"},
		{"zero max", "hello", 0, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		contains []string
	}{
		{"short text", "hello world", 20, []string{"hello world"}},
		{"needs wrap", "hello world foo bar", 10, []string{"hello", "world", "foo", "bar"}},
		{"zero width", "hello", 0, []string{"hello"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapText(tt.input, tt.width)
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("WrapText(%q, %d) = %q, expected to contain %q", tt.input, tt.width, result, substr)
				}
			}
		})
	}
}

func TestPanel(t *testing.T) {
	t.Run("basic panel", func(t *testing.T) {
		panel := NewPanel("Title", "Content")
		result := panel.Render()

		if !strings.Contains(result, "Title") {
			t.Error("Panel should contain title")
		}
		if !strings.Contains(result, "Content") {
			t.Error("Panel should contain content")
		}
	})

	t.Run("panel without title", func(t *testing.T) {
		panel := NewPanel("", "Content only")
		result := panel.Render()

		if !strings.Contains(result, "Content only") {
			t.Error("Panel should contain content")
		}
	})

	t.Run("panel with custom color", func(t *testing.T) {
		panel := NewPanel("Info", "Details").WithBorderColor(ColorCyan)
		result := panel.Render()

		if !strings.Contains(result, "Info") {
			t.Error("Panel should contain title")
		}
	})

	t.Run("convenience functions", func(t *testing.T) {
		info := RenderInfoPanel("Info", "content")
		success := RenderSuccessPanel("Success", "content")
		errPanel := RenderErrorPanel("Error", "content")
		warning := RenderWarningPanel("Warning", "content")

		if !strings.Contains(info, "Info") {
			t.Error("Info panel should contain title")
		}
		if !strings.Contains(success, "Success") {
			t.Error("Success panel should contain title")
		}
		if !strings.Contains(errPanel, "Error") {
			t.Error("Error panel should contain title")
		}
		if !strings.Contains(warning, "Warning") {
			t.Error("Warning panel should contain title")
		}
	})
}

func TestTable(t *testing.T) {
	t.Run("basic table", func(t *testing.T) {
		table := Table{
			Headers: []string{"ID", "Name", "Status"},
			Rows: [][]string{
				{"1", "Task A", "pending"},
				{"2", "Task B", "done"},
			},
		}

		result := table.Render()

		if !strings.Contains(result, "ID") {
			t.Error("Table should contain ID header")
		}
		if !strings.Contains(result, "Task A") {
			t.Error("Table should contain Task A")
		}
		if !strings.Contains(result, "done") {
			t.Error("Table should contain done status")
		}
	})

	t.Run("empty table", func(t *testing.T) {
		table := Table{}
		result := table.Render()

		if result != "" {
			t.Error("Empty table should render empty string")
		}
	})

	t.Run("with max width", func(t *testing.T) {
		table := Table{
			Headers:  []string{"Description"},
			Rows:     [][]string{{"This is a very long description that should be truncated"}},
			MaxWidth: 20,
		}

		widths := table.ColumnWidths()
		if widths[0] > 20 {
			t.Errorf("Column width should be <= 20, got %d", widths[0])
		}
	})
}
