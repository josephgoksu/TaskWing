package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuoteYAMLValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string no quoting needed",
			input: "simple",
			want:  "simple",
		},
		{
			name:  "string with colon",
			input: "has:colon",
			want:  `"has:colon"`,
		},
		{
			name:  "string with hash",
			input: "has#hash",
			want:  `"has#hash"`,
		},
		{
			name:  "string with space",
			input: "has space",
			want:  `"has space"`,
		},
		{
			name:  "string with double quote",
			input: `has"quote`,
			want:  `"has\"quote"`,
		},
		{
			name:  "string with newline",
			input: "has\nnewline",
			want:  `"has\nnewline"`,
		},
		{
			name:  "complex API key with special chars",
			input: "sk-proj-abc:def#123",
			want:  `"sk-proj-abc:def#123"`,
		},
		{
			name:  "backslash alone doesn't need quoting",
			input: `has\backslash`,
			want:  `has\backslash`,
		},
		{
			name:  "multiple special chars",
			input: `key:with"quotes'and#hash`,
			want:  `"key:with\"quotes'and#hash"`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "just alphanumeric",
			input: "abc123XYZ",
			want:  "abc123XYZ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteYAMLValue(tt.input)
			if got != tt.want {
				t.Errorf("quoteYAMLValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSaveGlobalLLMConfig_Validation(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		key      string
		wantErr  string
	}{
		{
			name:     "empty provider",
			provider: "",
			key:      "some-key",
			wantErr:  "provider cannot be empty",
		},
		{
			name:     "empty key",
			provider: "openai",
			key:      "",
			wantErr:  "API key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SaveGlobalLLMConfig(tt.provider, tt.key)
			if err == nil {
				t.Errorf("SaveGlobalLLMConfig(%q, %q) expected error, got nil", tt.provider, tt.key)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("SaveGlobalLLMConfig(%q, %q) error = %v, want containing %q", tt.provider, tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestSaveGlobalLLMConfig_NewFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "taskwing-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config dir for test
	origGetGlobalConfigDir := GetGlobalConfigDir
	GetGlobalConfigDir = func() (string, error) {
		return tmpDir, nil
	}
	defer func() { GetGlobalConfigDir = origGetGlobalConfigDir }()

	// Test creating new config
	err = SaveGlobalLLMConfig("gemini", "test-api-key:with#special")
	if err != nil {
		t.Fatalf("SaveGlobalLLMConfig failed: %v", err)
	}

	// Read and verify
	configPath := filepath.Join(tmpDir, "config.yaml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	contentStr := string(content)

	// Verify provider
	if !strings.Contains(contentStr, "provider: gemini") {
		t.Errorf("Config missing 'provider: gemini', got:\n%s", contentStr)
	}

	// Verify model (should be gemini default)
	if !strings.Contains(contentStr, "model: gemini-2.0-flash") {
		t.Errorf("Config missing 'model: gemini-2.0-flash', got:\n%s", contentStr)
	}

	// Verify API key is properly quoted (contains special chars)
	if !strings.Contains(contentStr, `"test-api-key:with#special"`) {
		t.Errorf("Config should have quoted API key, got:\n%s", contentStr)
	}
}

func TestSaveGlobalLLMConfig_UpdateExisting(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "taskwing-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config dir for test
	origGetGlobalConfigDir := GetGlobalConfigDir
	GetGlobalConfigDir = func() (string, error) {
		return tmpDir, nil
	}
	defer func() { GetGlobalConfigDir = origGetGlobalConfigDir }()

	// Create initial config with OpenAI
	initialConfig := `# TaskWing Global Configuration
version: "1"

llm:
  provider: openai
  model: gpt-5-mini-2025-08-07
  apiKeys:
    openai: sk-old-key
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(initialConfig), 0600); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Update to Gemini
	err = SaveGlobalLLMConfig("gemini", "new-gemini-key")
	if err != nil {
		t.Fatalf("SaveGlobalLLMConfig failed: %v", err)
	}

	// Read and verify
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	contentStr := string(content)

	// Verify provider changed
	if !strings.Contains(contentStr, "provider: gemini") {
		t.Errorf("Config should have 'provider: gemini', got:\n%s", contentStr)
	}

	// Verify model changed to Gemini default
	if !strings.Contains(contentStr, "model: gemini-2.0-flash") {
		t.Errorf("Config should have 'model: gemini-2.0-flash', got:\n%s", contentStr)
	}

	// Verify Gemini key added (may be quoted due to hyphen)
	if !strings.Contains(contentStr, "gemini:") || !strings.Contains(contentStr, "new-gemini-key") {
		t.Errorf("Config should have gemini key with 'new-gemini-key', got:\n%s", contentStr)
	}
}
