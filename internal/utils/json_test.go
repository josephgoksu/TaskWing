package utils

import (
	"testing"
)

func TestExtractAndParseJSON_ValidJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
		wantVal string
	}{
		{
			name:    "simple object",
			input:   `{"key": "value"}`,
			wantKey: "key",
			wantVal: "value",
		},
		{
			name:    "with markdown fence",
			input:   "```json\n{\"key\": \"value\"}\n```",
			wantKey: "key",
			wantVal: "value",
		},
		{
			name:    "with trailing text",
			input:   `{"key": "value"} some trailing text`,
			wantKey: "key",
			wantVal: "value",
		},
		{
			name:    "with leading text",
			input:   `Here is the JSON: {"key": "value"}`,
			wantKey: "key",
			wantVal: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]string
			err := func() error {
				var err error
				result, err = ExtractAndParseJSON[map[string]string](tt.input)
				return err
			}()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result[tt.wantKey] != tt.wantVal {
				t.Errorf("got %v, want %v", result[tt.wantKey], tt.wantVal)
			}
		})
	}
}

func TestExtractAndParseJSON_InvalidJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "no JSON",
			input: "just some text without JSON",
		},
		{
			name:  "incomplete JSON",
			input: `{"key": `,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractAndParseJSON[map[string]string](tt.input)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestRepairJSON_MissingCommas(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "missing comma between string properties",
			input: `{
"key1": "value1"
"key2": "value2"
}`,
			want: `{
"key1": "value1", "key2": "value2"
}`,
		},
		{
			name: "missing comma after number",
			input: `{
"count": 42
"name": "test"
}`,
			want: `{
"count": 42, "name": "test"
}`,
		},
		{
			name: "missing comma after boolean",
			input: `{
"enabled": true
"name": "test"
}`,
			want: `{
"enabled": true, "name": "test"
}`,
		},
		{
			name: "missing comma after null",
			input: `{
"value": null
"name": "test"
}`,
			want: `{
"value": null, "name": "test"
}`,
		},
		{
			name:  "missing comma after closing brace",
			input: `{"a": {"b": 1} "c": 2}`,
			want:  `{"a": {"b": 1}, "c": 2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repairJSON(tt.input)
			if got != tt.want {
				t.Errorf("repairJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepairJSON_TrailingCommas(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "trailing comma in object",
			input: `{"key": "value",}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "trailing comma in array",
			input: `[1, 2, 3,]`,
			want:  `[1, 2, 3]`,
		},
		{
			name:  "trailing comma with whitespace",
			input: `{"key": "value" , }`,
			want:  `{"key": "value" }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repairJSON(tt.input)
			if got != tt.want {
				t.Errorf("repairJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepairJSON_SingleQuotes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single quoted key",
			input: `{'key': "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "single quoted value",
			input: `{"key": 'value'}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "both single quoted",
			input: `{'key': 'value'}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "single quoted with escaped quote inside",
			input: `{"message": 'It\'s working'}`,
			want:  `{"message": "It's working"}`,
		},
		{
			name:  "single quoted with double quote inside",
			input: `{"message": 'He said "hello"'}`,
			want:  `{"message": "He said \"hello\""}`,
		},
		{
			name:  "multiple single quoted values",
			input: `{'a': 'one', 'b': 'two'}`,
			want:  `{"a": "one", "b": "two"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repairJSON(tt.input)
			if got != tt.want {
				t.Errorf("repairJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepairJSON_ComplexCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool // whether the repaired JSON should be valid
	}{
		{
			name:      "already valid JSON",
			input:     `{"key": "value", "num": 42}`,
			wantValid: true,
		},
		{
			name:      "nested objects with trailing comma",
			input:     `{"outer": {"inner": "value",},}`,
			wantValid: true,
		},
		{
			name: "LLM output with missing commas only",
			input: `{
"title": "My Feature"
"description": "A description"
}`,
			wantValid: true,
		},
		{
			name:      "all single quotes on one line",
			input:     `{'title': 'My Feature', 'description': 'A description'}`,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repaired := repairJSON(tt.input)
			_, err := ExtractAndParseJSON[map[string]any](repaired)
			isValid := err == nil
			if isValid != tt.wantValid {
				t.Errorf("repaired JSON validity = %v, want %v. Repaired: %q, error: %v",
					isValid, tt.wantValid, repaired, err)
			}
		})
	}
}

func TestCleanLLMResponse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "json code block",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "generic code block",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "no code block",
			input: "{\"key\": \"value\"}",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "with whitespace",
			input: "  \n```json\n{\"key\": \"value\"}\n```\n  ",
			want:  "{\"key\": \"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanLLMResponse(tt.input)
			if got != tt.want {
				t.Errorf("cleanLLMResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractAndParseJSON_WithRepair(t *testing.T) {
	// Test that ExtractAndParseJSON successfully uses repairJSON for broken input
	type TestStruct struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}

	tests := []struct {
		name    string
		input   string
		want    TestStruct
		wantErr bool
	}{
		{
			name: "missing comma repaired",
			input: `{
"title": "Test"
"description": "A test"
}`,
			want: TestStruct{Title: "Test", Description: "A test"},
		},
		{
			name:  "single quotes repaired",
			input: `{'title': 'Test', 'description': 'A test'}`,
			want:  TestStruct{Title: "Test", Description: "A test"},
		},
		{
			name:  "trailing comma repaired",
			input: `{"title": "Test", "description": "A test",}`,
			want:  TestStruct{Title: "Test", Description: "A test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractAndParseJSON[TestStruct](tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExtractAndParseJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ExtractAndParseJSON() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
