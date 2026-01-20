package utils

import (
	"strings"
	"testing"
)

// TestExtractAndParseJSON_InvalidEscapeSequences tests JSON parsing with invalid
// escape sequences that LLMs commonly produce (e.g., \c, \s, \d from regex patterns).
// This is a regression test for the "invalid character 'c' in string escape code" error.
func TestExtractAndParseJSON_InvalidEscapeSequences(t *testing.T) {
	type TestResult struct {
		Name        string `json:"name"`
		Pattern     string `json:"pattern"`
		Description string `json:"description"`
	}

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid JSON",
			input:   `{"name": "test", "pattern": "foo", "description": "bar"}`,
			wantErr: false,
		},
		{
			name:    "regex pattern with backslash-s",
			input:   `{"name": "regex", "pattern": "^\s+match\s*$", "description": "whitespace"}`,
			wantErr: false,
		},
		{
			name:    "regex pattern with backslash-d",
			input:   `{"name": "digits", "pattern": "\d+", "description": "numbers"}`,
			wantErr: false,
		},
		{
			name:    "regex pattern with backslash-w",
			input:   `{"name": "word", "pattern": "\w+", "description": "word chars"}`,
			wantErr: false,
		},
		{
			name:    "regex pattern with backslash-c (the specific failing case)",
			input:   `{"name": "ctrl", "pattern": "\c", "description": "control char"}`,
			wantErr: false,
		},
		{
			name:    "multiple invalid escapes",
			input:   `{"name": "complex", "pattern": "\s\d\w\c\x", "description": "mixed"}`,
			wantErr: false,
		},
		{
			name:    "Windows path with backslash-C (common in file paths)",
			input:   `{"name": "path", "pattern": "C:\code\project", "description": "Windows path"}`,
			wantErr: false,
		},
		{
			name:    "Windows path with lowercase",
			input:   `{"name": "path", "pattern": "c:\code\project", "description": "lowercase drive"}`,
			wantErr: false,
		},
		{
			name:    "JSON embedded in markdown code block",
			input:   "```json\n{\"name\": \"test\", \"pattern\": \"\\s+\", \"description\": \"wrapped\"}\n```",
			wantErr: false,
		},
		{
			name:    "LLM response with explanation before JSON",
			input:   "Here's the analysis:\n\n{\"name\": \"test\", \"pattern\": \"\\d+\", \"description\": \"with prefix\"}",
			wantErr: false,
		},
		{
			name:    "nested invalid escapes in code snippet",
			input:   `{"name": "code", "pattern": "func match(s string) bool {\n\treturn regexp.MustCompile(`+"`"+`\s+`+"`"+`).MatchString(s)\n}", "description": "code with regex"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractAndParseJSON[TestResult](tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractAndParseJSON() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ExtractAndParseJSON() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractAndParseJSON() unexpected error: %v", err)
				return
			}

			// Basic validation that parsing worked
			if result.Name == "" {
				t.Error("ExtractAndParseJSON() result.Name is empty, expected non-empty")
			}
		})
	}
}

// TestSanitizeControlChars_InvalidEscapes specifically tests the sanitization
// of invalid JSON escape sequences.
func TestSanitizeControlChars_InvalidEscapes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
	}{
		{
			name:   "backslash-c inside string",
			input:  `{"key": "value\c"}`,
			want:   `{"key": "value\\c"}`,
		},
		{
			name:   "backslash-s inside string",
			input:  `{"key": "\s+"}`,
			want:   `{"key": "\\s+"}`,
		},
		{
			name:   "backslash-d inside string",
			input:  `{"key": "\d{3}"}`,
			want:   `{"key": "\\d{3}"}`,
		},
		{
			name:   "backslash-w inside string",
			input:  `{"key": "\w*"}`,
			want:   `{"key": "\\w*"}`,
		},
		{
			name:   "valid escapes preserved",
			input:  `{"key": "line1\nline2\ttab"}`,
			want:   `{"key": "line1\nline2\ttab"}`,
		},
		{
			name:   "mixed valid and invalid",
			input:  `{"key": "\n\s\t\d"}`,
			want:   `{"key": "\n\\s\t\\d"}`,
		},
		{
			name:   "backslash outside string unchanged",
			input:  `{"key": "value"}\extra`,
			want:   `{"key": "value"}\extra`,
		},
		{
			// Note: \t is a valid JSON escape for tab, so it's preserved.
			// The actual fix for Windows paths happens in the full repair pipeline.
			name:   "Windows path - partial (t is valid escape)",
			input:  `{"path": "C:\code\test"}`,
			want:   `{"path": "C:\\code\test"}`,
		},
		{
			name:   "escaped backslash preserved",
			input:  `{"key": "path\\to\\file"}`,
			want:   `{"key": "path\\to\\file"}`,
		},
		{
			name:   "escaped quote preserved",
			input:  `{"key": "say \"hello\""}`,
			want:   `{"key": "say \"hello\""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeControlChars(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeControlChars() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRepairJSON_InvalidEscapes tests the full repair pipeline including
// control character sanitization.
func TestRepairJSON_InvalidEscapes(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool // should the repaired JSON be valid?
	}{
		{
			name:      "regex pattern with backslash-s",
			input:     `{"pattern": "\s+"}`,
			wantValid: true,
		},
		{
			name:      "Windows path",
			input:     `{"path": "C:\code\project\file.go"}`,
			wantValid: true,
		},
		{
			name:      "multiple regex escapes",
			input:     `{"regex": "^\s*\d+\w+\c$"}`,
			wantValid: true,
		},
		{
			name:      "complex code snippet with escapes",
			input:     `{"code": "if match, _ := regexp.MatchString(`+"`"+`\s+`+"`"+`, s); match {\n\tfmt.Println(\"found\")\n}"}`,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repaired := repairJSON(tt.input)

			// Try to parse the repaired JSON
			var result map[string]any
			_, err := ExtractAndParseJSON[map[string]any](repaired)

			if tt.wantValid && err != nil {
				t.Errorf("repairJSON() produced invalid JSON: %v\nInput: %s\nRepaired: %s", err, tt.input, repaired)
			}
			if !tt.wantValid && err == nil {
				t.Errorf("repairJSON() unexpectedly produced valid JSON: %v", result)
			}
		})
	}
}

// TestExtractAndParseJSON_WindowsPaths tests handling of Windows-style paths
// which commonly cause "invalid character 'c'" errors due to \c sequences.
func TestExtractAndParseJSON_WindowsPaths(t *testing.T) {
	type PathResult struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "C drive path with Users",
			input:   `{"file_path": "C:\Users\dev\project\main.go", "content": "package main"}`,
			wantErr: false,
		},
		{
			name:    "path with backslash-t (taskwing) - valid escape in wrong context",
			input:   `{"file_path": "c:\code\taskwing\file.go", "content": "package utils"}`,
			wantErr: false,
		},
		{
			name:    "path with backslash-n (new folder) - valid escape in wrong context",
			input:   `{"file_path": "D:\projects\new\app\main.go", "content": "package main"}`,
			wantErr: false,
		},
		{
			name:    "path with backslash-u (utils) - unicode prefix in wrong context",
			input:   `{"file_path": "C:\code\utils\helper.go", "content": "package utils"}`,
			wantErr: false,
		},
		{
			name:    "path with backslash-r (release) - valid escape in wrong context",
			input:   `{"file_path": "C:\build\release\app.exe", "content": "binary"}`,
			wantErr: false,
		},
		{
			name:    "simple invalid escape",
			input:   `{"file_path": "C:\code\file.go", "content": "test"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractAndParseJSON[PathResult](tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("ExtractAndParseJSON() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractAndParseJSON() error = %v", err)
				return
			}

			if result.FilePath == "" {
				t.Error("ExtractAndParseJSON() result.FilePath is empty")
			}
		})
	}
}

// TestExtractAndParseJSON_LLMCodeAnalysis simulates real LLM output that caused
// the "invalid character 'c'" error during bootstrap code analysis.
func TestExtractAndParseJSON_LLMCodeAnalysis(t *testing.T) {
	type Evidence struct {
		FilePath  string `json:"file_path"`
		StartLine int    `json:"start_line"`
		Snippet   string `json:"snippet"`
	}

	type Finding struct {
		Title       string     `json:"title"`
		Description string     `json:"description"`
		Evidence    []Evidence `json:"evidence"`
	}

	type AnalysisResult struct {
		Decisions []Finding `json:"decisions"`
		Patterns  []Finding `json:"patterns"`
	}

	// This simulates the kind of JSON that might contain file paths or code snippets
	// with problematic escape sequences
	input := `{
		"decisions": [{
			"title": "Use structured logging",
			"description": "The codebase uses structured logging with fields",
			"evidence": [{
				"file_path": "internal/bootstrap/scanner.go",
				"start_line": 42,
				"snippet": "log.WithFields(log.Fields{\"path\": path}).Info(\"scanning\")"
			}]
		}],
		"patterns": [{
			"title": "Regex-based parsing",
			"description": "Uses regex patterns like \s+ and \d+ for parsing",
			"evidence": [{
				"file_path": "internal/utils/parser.go",
				"start_line": 15,
				"snippet": "regexp.MustCompile(`+"`"+`^\s*(\w+)\s*=\s*(.*)$`+"`"+`)"
			}]
		}]
	}`

	result, err := ExtractAndParseJSON[AnalysisResult](input)
	if err != nil {
		t.Fatalf("ExtractAndParseJSON() failed on LLM-like output: %v", err)
	}

	if len(result.Decisions) != 1 {
		t.Errorf("Expected 1 decision, got %d", len(result.Decisions))
	}
	if len(result.Patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(result.Patterns))
	}
}
