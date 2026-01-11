package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFTSQueryForNodes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty query returns empty",
			input:    "",
			expected: "",
		},
		{
			name:     "single word preserved",
			input:    "database",
			expected: `"database"`,
		},
		{
			name:     "stop words filtered",
			input:    "What is the database",
			expected: `"database"`,
		},
		{
			name:     "multiple content words use OR",
			input:    "SQLite database storage",
			expected: `"sqlite" OR "database" OR "storage"`,
		},
		{
			name:     "query rewrite with TaskWing",
			input:    "What type of database does TaskWing use?",
			expected: `"database" OR "taskwing"`,
		},
		{
			name:     "special characters escaped",
			input:    "func(ctx context.Context)",
			expected: `"func" OR "ctx" OR "context" OR "context"`, // context appears twice (lowercased)
		},
		{
			name:     "FTS operators filtered",
			input:    "database AND storage OR sqlite NOT memory",
			expected: `"database" OR "storage" OR "sqlite" OR "memory"`,
		},
		{
			name:     "short words filtered",
			input:    "a b c database",
			expected: `"database"`,
		},
		{
			name:     "all stop words returns empty",
			input:    "what is the type of",
			expected: "",
		},
		{
			name:     "preserves technical terms",
			input:    "SQLiteStore NewRepository",
			expected: `"sqlitestore" OR "newrepository"`,
		},
		{
			name:     "handles quoted input",
			input:    `"database" "storage"`,
			expected: `"database" OR "storage"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFTSQueryForNodes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeFTSQueryForNodes_RealWorldQueries(t *testing.T) {
	// Test queries that previously caused zero results
	tests := []struct {
		name        string
		input       string
		shouldMatch bool // Should produce non-empty query
	}{
		{
			name:        "natural language question",
			input:       "How does authentication work?",
			shouldMatch: true, // "authentication" OR "work" should remain
		},
		{
			name:        "database type question",
			input:       "What type of database does it use?",
			shouldMatch: true, // "database" should remain
		},
		{
			name:        "pure stop words",
			input:       "is it the",
			shouldMatch: false,
		},
		{
			name:        "technical search",
			input:       "SQLite CGO-free storage",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFTSQueryForNodes(tt.input)
			if tt.shouldMatch {
				assert.NotEmpty(t, result, "Expected non-empty query for: %s", tt.input)
			} else {
				assert.Empty(t, result, "Expected empty query for: %s", tt.input)
			}
		})
	}
}
