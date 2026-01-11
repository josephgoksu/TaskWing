package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		maxLen   int
		expected string
	}{
		{
			name:     "short content unchanged",
			content:  "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length unchanged",
			content:  "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long content truncated",
			content:  "hello world",
			maxLen:   5,
			expected: "hello...",
		},
		{
			name:     "empty content",
			content:  "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "unicode content truncated correctly",
			content:  "日本語テスト",
			maxLen:   3,
			expected: "日本語...",
		},
		{
			name:     "mixed unicode truncated correctly",
			content:  "Hello 世界!",
			maxLen:   7,
			expected: "Hello 世...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateContent(tt.content, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetContentWithoutSummary(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		summary  string
		expected string
	}{
		{
			name:     "content starts with summary",
			content:  "SQLite Storage\nMore details here",
			summary:  "SQLite Storage",
			expected: "More details here",
		},
		{
			name:     "content does not start with summary",
			content:  "Different content here",
			summary:  "SQLite Storage",
			expected: "Different content here",
		},
		{
			name:     "empty summary returns content unchanged",
			content:  "Full content here",
			summary:  "",
			expected: "Full content here",
		},
		{
			name:     "short summary (1 char) returns content unchanged",
			content:  "A test content",
			summary:  "A",
			expected: "A test content",
		},
		{
			name:     "short summary (2 chars) returns content unchanged",
			content:  "AB test content",
			summary:  "AB",
			expected: "AB test content",
		},
		{
			name:     "summary with trailing whitespace in content",
			content:  "Title\n\n\nContent after newlines",
			summary:  "Title",
			expected: "Content after newlines",
		},
		{
			name:     "summary equals content",
			content:  "Same text",
			summary:  "Same text",
			expected: "",
		},
		{
			name:     "content is subset of summary",
			content:  "Short",
			summary:  "Short and long",
			expected: "Short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getContentWithoutSummary(tt.content, tt.summary)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContentDisplayThresholds(t *testing.T) {
	// Verify the constants are set correctly
	assert.Equal(t, float32(0.7), contentDisplayRelativeThreshold)
	assert.Equal(t, float32(0.25), contentDisplayAbsoluteThreshold)
}
