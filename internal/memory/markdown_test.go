package memory

import (
	"testing"
)

func TestToTitleCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Single word lowercase",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "Single word uppercase",
			input:    "HELLO",
			expected: "Hello",
		},
		{
			name:     "Multiple words",
			input:    "hello world",
			expected: "Hello World",
		},
		{
			name:     "Mixed case",
			input:    "hElLo wOrLd",
			expected: "Hello World",
		},
		{
			name:     "Edge case: double spaces",
			input:    "hello  world",
			expected: "Hello World", // strings.Fields collapses spaces
		},
		{
			name:     "Edge case: single letter",
			input:    "a",
			expected: "A",
		},
		{
			name:     "Edge case: single letter word phrase",
			input:    "a b c",
			expected: "A B C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toTitleCase(tt.input)
			if got != tt.expected {
				t.Errorf("toTitleCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
