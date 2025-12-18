package bootstrap

import (
	"testing"
)

func TestParsePackageJSONDeps(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "basic dependencies",
			content: `{
				"name": "test-app",
				"dependencies": {
					"react": "^18.0.0",
					"vite": "^5.0.0"
				}
			}`,
			expected: []string{"react", "vite"},
		},
		{
			name: "empty dependencies",
			content: `{
				"name": "test-app",
				"dependencies": {}
			}`,
			expected: []string{},
		},
		{
			name:     "invalid JSON",
			content:  `not json`,
			expected: nil,
		},
		{
			name:     "no dependencies key",
			content:  `{"name": "test"}`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePackageJSONDeps(tt.content)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d deps, got %d", len(tt.expected), len(result))
				return
			}
			// Check all expected deps are present (order may vary due to map iteration)
			for _, exp := range tt.expected {
				found := false
				for _, got := range result {
					if got == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected dep %s not found in result %v", exp, result)
				}
			}
		})
	}
}

func TestParseGoModDeps(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "direct dependencies only",
			content: `module github.com/example/test

go 1.21

require (
	github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.18.0
	github.com/other/lib v1.0.0 // indirect
)`,
			expected: []string{"spf13/cobra", "spf13/viper"},
		},
		{
			name:     "no require block",
			content:  `module github.com/example/test`,
			expected: []string{},
		},
		{
			name: "empty require block",
			content: `module github.com/example/test
require (
)`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGoModDeps(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d deps, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("at index %d: expected %s, got %s", i, exp, result[i])
				}
			}
		})
	}
}
