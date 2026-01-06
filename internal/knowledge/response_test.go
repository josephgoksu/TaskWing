package knowledge

import (
	"reflect"
	"testing"
)

func TestParseEvidence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []EvidenceRef
	}{
		{
			name:  "No duplicates",
			input: `[{"file_path":"a.go","start_line":10,"end_line":20}, {"file_path":"b.go","start_line":5,"end_line":5}]`,
			expected: []EvidenceRef{
				{File: "a.go", Lines: "10-20"},
				{File: "b.go", Lines: "5"},
			},
		},
		{
			name:  "With duplicates",
			input: `[{"file_path":"a.go","start_line":10,"end_line":20}, {"file_path":"a.go","start_line":10,"end_line":20}, {"file_path":"b.go","start_line":5,"end_line":5}]`,
			expected: []EvidenceRef{
				{File: "a.go", Lines: "10-20"},
				{File: "b.go", Lines: "5"},
			},
		},
		{
			name:     "Empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "Invalid JSON",
			input:    "{invalid",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEvidence(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseEvidence() = %v, want %v", got, tt.expected)
			}
		})
	}
}
