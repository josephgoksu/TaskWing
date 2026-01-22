/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// OpenCode MCP Install Tests
// =============================================================================

// TestInstallOpenCode_Success tests successful creation of opencode.json
func TestInstallOpenCode_Success(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := "/usr/local/bin/taskwing"

	err := installOpenCode(binPath, tmpDir)
	if err != nil {
		t.Fatalf("installOpenCode failed: %v", err)
	}

	// Verify file was created at project root
	configPath := filepath.Join(tmpDir, "opencode.json")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read opencode.json: %v", err)
	}

	// Parse and verify JSON structure
	var config OpenCodeConfig
	if err := json.Unmarshal(content, &config); err != nil {
		t.Fatalf("Invalid JSON in opencode.json: %v", err)
	}

	// Verify schema
	if config.Schema != "https://opencode.ai/config.json" {
		t.Errorf("Schema = %q, want %q", config.Schema, "https://opencode.ai/config.json")
	}

	// Verify MCP section exists
	if config.MCP == nil {
		t.Fatal("MCP section is nil")
	}

	// Verify taskwing-mcp entry
	serverCfg, ok := config.MCP["taskwing-mcp"]
	if !ok {
		t.Fatal("taskwing-mcp entry not found in MCP section")
	}

	// Verify type is "local"
	if serverCfg.Type != "local" {
		t.Errorf("Type = %q, want %q", serverCfg.Type, "local")
	}

	// Verify command is array format
	if len(serverCfg.Command) != 2 {
		t.Fatalf("Command length = %d, want 2", len(serverCfg.Command))
	}
	if serverCfg.Command[0] != binPath {
		t.Errorf("Command[0] = %q, want %q", serverCfg.Command[0], binPath)
	}
	if serverCfg.Command[1] != "mcp" {
		t.Errorf("Command[1] = %q, want %q", serverCfg.Command[1], "mcp")
	}

	// Verify timeout is set
	if serverCfg.Timeout != 5000 {
		t.Errorf("Timeout = %d, want %d", serverCfg.Timeout, 5000)
	}
}

// TestInstallOpenCode_CommandIsArray tests that command is JSON array, not string
func TestInstallOpenCode_CommandIsArray(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := "/path/to/taskwing"

	err := installOpenCode(binPath, tmpDir)
	if err != nil {
		t.Fatalf("installOpenCode failed: %v", err)
	}

	// Read raw JSON to verify array format
	content, err := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Check raw JSON contains array syntax for command
	if !containsSubstr(string(content), `"command": [`) {
		t.Error("command must be JSON array format (not string)")
	}
}

// TestInstallOpenCode_PreservesExistingConfig tests that existing config is preserved
func TestInstallOpenCode_PreservesExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "opencode.json")

	// Create existing config with another MCP server
	existingConfig := OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		MCP: map[string]OpenCodeMCPServerConfig{
			"other-mcp": {
				Type:    "local",
				Command: []string{"other", "command"},
			},
		},
	}
	existingBytes, _ := json.MarshalIndent(existingConfig, "", "  ")
	if err := os.WriteFile(configPath, existingBytes, 0644); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Install TaskWing
	err := installOpenCode("/usr/local/bin/taskwing", tmpDir)
	if err != nil {
		t.Fatalf("installOpenCode failed: %v", err)
	}

	// Read back and verify both servers exist
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config OpenCodeConfig
	if err := json.Unmarshal(content, &config); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Verify existing server preserved
	if _, ok := config.MCP["other-mcp"]; !ok {
		t.Error("Existing 'other-mcp' server was removed")
	}

	// Verify new server added
	if _, ok := config.MCP["taskwing-mcp"]; !ok {
		t.Error("'taskwing-mcp' server was not added")
	}
}

// TestInstallOpenCode_Idempotent tests that running twice doesn't duplicate
func TestInstallOpenCode_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := "/usr/local/bin/taskwing"

	// Install twice
	if err := installOpenCode(binPath, tmpDir); err != nil {
		t.Fatalf("First install failed: %v", err)
	}
	if err := installOpenCode(binPath, tmpDir); err != nil {
		t.Fatalf("Second install failed: %v", err)
	}

	// Read and verify only one taskwing-mcp entry
	content, err := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config OpenCodeConfig
	if err := json.Unmarshal(content, &config); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Should have exactly one entry
	if len(config.MCP) != 1 {
		t.Errorf("Expected 1 MCP entry, got %d", len(config.MCP))
	}
}

// TestInstallOpenCode_NoSecrets tests that no secrets are written
func TestInstallOpenCode_NoSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := "/usr/local/bin/taskwing"

	err := installOpenCode(binPath, tmpDir)
	if err != nil {
		t.Fatalf("installOpenCode failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "opencode.json"))
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	contentStr := string(content)

	// Check no common secret patterns
	secretPatterns := []string{
		"password",
		"secret",
		"api_key",
		"apikey",
		"API_KEY",
		"token",
		"credential",
	}

	for _, pattern := range secretPatterns {
		if containsSubstr(contentStr, pattern) {
			t.Errorf("Config contains potential secret pattern: %s", pattern)
		}
	}

	// Verify no .env file was created
	envPath := filepath.Join(tmpDir, ".env")
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Error(".env file should NOT be created")
	}
}

// =============================================================================
// upsertOpenCodeMCPServer Validation Tests
// =============================================================================

// TestUpsertOpenCodeMCPServer_EmptyConfigPath tests validation of empty configPath
func TestUpsertOpenCodeMCPServer_EmptyConfigPath(t *testing.T) {
	err := upsertOpenCodeMCPServer("", "taskwing-mcp", OpenCodeMCPServerConfig{
		Type:    "local",
		Command: []string{"taskwing", "mcp"},
	})
	if err == nil {
		t.Error("Expected error for empty configPath, got nil")
	}
}

// TestUpsertOpenCodeMCPServer_EmptyServerName tests validation of empty serverName
func TestUpsertOpenCodeMCPServer_EmptyServerName(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "opencode.json")

	err := upsertOpenCodeMCPServer(configPath, "", OpenCodeMCPServerConfig{
		Type:    "local",
		Command: []string{"taskwing", "mcp"},
	})
	if err == nil {
		t.Error("Expected error for empty serverName, got nil")
	}
}

// TestUpsertOpenCodeMCPServer_EmptyCommand tests validation of empty command
func TestUpsertOpenCodeMCPServer_EmptyCommand(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "opencode.json")

	err := upsertOpenCodeMCPServer(configPath, "taskwing-mcp", OpenCodeMCPServerConfig{
		Type:    "local",
		Command: []string{}, // Empty command array
	})
	if err == nil {
		t.Error("Expected error for empty command array, got nil")
	}
}

// TestUpsertOpenCodeMCPServer_InvalidType tests validation of invalid type
func TestUpsertOpenCodeMCPServer_InvalidType(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "opencode.json")

	err := upsertOpenCodeMCPServer(configPath, "taskwing-mcp", OpenCodeMCPServerConfig{
		Type:    "remote", // Invalid - must be "local"
		Command: []string{"taskwing", "mcp"},
	})
	if err == nil {
		t.Error("Expected error for invalid type, got nil")
	}
}

// TestUpsertOpenCodeMCPServer_MalformedExistingJSON tests handling of malformed JSON
func TestUpsertOpenCodeMCPServer_MalformedExistingJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "opencode.json")

	// Write malformed JSON
	if err := os.WriteFile(configPath, []byte("not valid json{"), 0644); err != nil {
		t.Fatalf("Failed to write malformed JSON: %v", err)
	}

	// Should succeed by creating fresh config
	err := upsertOpenCodeMCPServer(configPath, "taskwing-mcp", OpenCodeMCPServerConfig{
		Type:    "local",
		Command: []string{"taskwing", "mcp"},
	})
	if err != nil {
		t.Fatalf("Should handle malformed JSON gracefully: %v", err)
	}

	// Verify valid config was written
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config OpenCodeConfig
	if err := json.Unmarshal(content, &config); err != nil {
		t.Fatalf("Config should be valid JSON now: %v", err)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// containsSubstr checks if s contains substr
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
