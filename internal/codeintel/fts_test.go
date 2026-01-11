package codeintel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFTSQuery(t *testing.T) {
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
			name:     "natural language question",
			input:    "How do embeddings work in this project?",
			expected: `"embeddings" OR "work" OR "project"`,
		},
		{
			name:     "special characters escaped",
			input:    "func(ctx context.Context)",
			expected: `"func" OR "ctx" OR "context"`,
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
			input:    "what is the of",
			expected: "",
		},
		{
			name:     "type preserved for code search",
			input:    "what is the type of",
			expected: `"type"`, // "type" is NOT a stop word for code (valid: "type Repository")
		},
		{
			name:     "preserves technical terms",
			input:    "SQLiteStore NewRepository",
			expected: `"sqlitestore" OR "newrepository"`,
		},
		{
			name:     "prefix wildcard preserved",
			input:    "Create*",
			expected: "create*",
		},
		{
			name:     "mixed wildcard and regular",
			input:    "Create* Handler",
			expected: `create* OR "handler"`,
		},
		{
			name:     "duplicates removed",
			input:    "database database storage",
			expected: `"database" OR "storage"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFTSQuery(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeFTSQuery_RealWorldQueries(t *testing.T) {
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
			name:        "code search",
			input:       "func NewRepository",
			shouldMatch: true,
		},
		{
			name:        "pure stop words",
			input:       "is it the",
			shouldMatch: false,
		},
		{
			name:        "technical with hyphens",
			input:       "CGO-free storage",
			shouldMatch: true,
		},
		{
			name:        "method signature",
			input:       "func (s *Service) Search",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFTSQuery(tt.input)
			if tt.shouldMatch {
				assert.NotEmpty(t, result, "Expected non-empty query for: %s", tt.input)
			} else {
				assert.Empty(t, result, "Expected empty query for: %s", tt.input)
			}
		})
	}
}
