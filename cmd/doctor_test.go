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
// OpenCode MCP Doctor Tests
// =============================================================================

// TestDoctor_CheckOpenCodeMCP_NoConfig tests that missing opencode.json is not an error.
func TestDoctor_CheckOpenCodeMCP_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()

	checks := checkOpenCodeMCP(tmpDir)

	// No opencode.json should return empty checks (not an error - OpenCode not configured)
	if len(checks) != 0 {
		t.Errorf("Expected 0 checks when no opencode.json exists, got %d", len(checks))
	}
}

// TestDoctor_CheckOpenCodeMCP_ValidConfig tests valid opencode.json passes.
func TestDoctor_CheckOpenCodeMCP_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid opencode.json
	config := OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		MCP: map[string]OpenCodeMCPServerConfig{
			"taskwing-mcp": {
				Type:    "local",
				Command: []string{"/usr/local/bin/taskwing", "mcp"},
				Timeout: 5000,
			},
		},
	}
	writeOpenCodeConfig(t, tmpDir, config)

	checks := checkOpenCodeMCP(tmpDir)

	// Should have exactly one check (MCP config OK)
	if len(checks) == 0 {
		t.Fatal("Expected at least one check for valid config")
	}

	// First check should be OK
	if checks[0].Status != "ok" {
		t.Errorf("Expected status 'ok', got %q with message %q", checks[0].Status, checks[0].Message)
	}
	if checks[0].Name != "MCP (OpenCode)" {
		t.Errorf("Expected name 'MCP (OpenCode)', got %q", checks[0].Name)
	}
}

// TestDoctor_CheckOpenCodeMCP_InvalidJSON tests that malformed JSON is detected.
func TestDoctor_CheckOpenCodeMCP_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invalid JSON
	configPath := filepath.Join(tmpDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte("not valid json{"), 0644); err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	checks := checkOpenCodeMCP(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for invalid JSON")
	}

	// Should fail with invalid JSON message
	if checks[0].Status != "fail" {
		t.Errorf("Expected status 'fail' for invalid JSON, got %q", checks[0].Status)
	}
	if checks[0].Message != "Invalid JSON in opencode.json" {
		t.Errorf("Unexpected message: %q", checks[0].Message)
	}
}

// TestDoctor_CheckOpenCodeMCP_NoMCPServers tests that empty MCP section is detected.
func TestDoctor_CheckOpenCodeMCP_NoMCPServers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config with no MCP servers
	config := OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		MCP:    map[string]OpenCodeMCPServerConfig{},
	}
	writeOpenCodeConfig(t, tmpDir, config)

	checks := checkOpenCodeMCP(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for empty MCP section")
	}

	if checks[0].Status != "fail" {
		t.Errorf("Expected status 'fail' for empty MCP, got %q", checks[0].Status)
	}
}

// TestDoctor_CheckOpenCodeMCP_NoTaskwingMCP tests that missing taskwing-mcp is detected.
func TestDoctor_CheckOpenCodeMCP_NoTaskwingMCP(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config with other MCP server but no taskwing-mcp
	config := OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		MCP: map[string]OpenCodeMCPServerConfig{
			"other-mcp": {
				Type:    "local",
				Command: []string{"other", "command"},
			},
		},
	}
	writeOpenCodeConfig(t, tmpDir, config)

	checks := checkOpenCodeMCP(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for missing taskwing-mcp")
	}

	if checks[0].Status != "fail" {
		t.Errorf("Expected status 'fail' for missing taskwing-mcp, got %q", checks[0].Status)
	}
	if checks[0].Message != "taskwing-mcp not found in opencode.json" {
		t.Errorf("Unexpected message: %q", checks[0].Message)
	}
}

// TestDoctor_CheckOpenCodeMCP_InvalidType tests that wrong type is detected.
func TestDoctor_CheckOpenCodeMCP_InvalidType(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config with invalid type
	config := OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		MCP: map[string]OpenCodeMCPServerConfig{
			"taskwing-mcp": {
				Type:    "remote", // Invalid - must be "local"
				Command: []string{"/usr/local/bin/taskwing", "mcp"},
			},
		},
	}
	writeOpenCodeConfig(t, tmpDir, config)

	checks := checkOpenCodeMCP(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for invalid type")
	}

	if checks[0].Status != "fail" {
		t.Errorf("Expected status 'fail' for invalid type, got %q", checks[0].Status)
	}
}

// TestDoctor_CheckOpenCodeMCP_InvalidCommand tests that invalid command is detected.
func TestDoctor_CheckOpenCodeMCP_InvalidCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config with invalid command (too few elements)
	config := OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		MCP: map[string]OpenCodeMCPServerConfig{
			"taskwing-mcp": {
				Type:    "local",
				Command: []string{"taskwing"}, // Missing "mcp"
			},
		},
	}
	writeOpenCodeConfig(t, tmpDir, config)

	checks := checkOpenCodeMCP(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for invalid command")
	}

	if checks[0].Status != "fail" {
		t.Errorf("Expected status 'fail' for invalid command, got %q", checks[0].Status)
	}
}

// TestDoctor_CheckOpenCodeMCP_WithProjectSuffix tests that taskwing-mcp-<project> is recognized.
func TestDoctor_CheckOpenCodeMCP_WithProjectSuffix(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config with project-specific server name
	config := OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		MCP: map[string]OpenCodeMCPServerConfig{
			"taskwing-mcp-my-project": {
				Type:    "local",
				Command: []string{"/usr/local/bin/taskwing", "mcp"},
			},
		},
	}
	writeOpenCodeConfig(t, tmpDir, config)

	checks := checkOpenCodeMCP(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for config with project suffix")
	}

	if checks[0].Status != "ok" {
		t.Errorf("Expected status 'ok' for valid config with suffix, got %q", checks[0].Status)
	}
}

// =============================================================================
// OpenCode Commands Doctor Tests
// =============================================================================

// TestDoctor_CheckOpenCodeCommands_NoCommandsDir tests that missing commands dir is not an error.
func TestDoctor_CheckOpenCodeCommands_NoCommandsDir(t *testing.T) {
	tmpDir := t.TempDir()

	checks := checkOpenCodeCommands(tmpDir)

	// No commands directory should return empty checks
	if len(checks) != 0 {
		t.Errorf("Expected 0 checks when no commands directory exists, got %d", len(checks))
	}
}

// TestDoctor_CheckOpenCodeCommands_ValidCommand tests valid command passes.
func TestDoctor_CheckOpenCodeCommands_ValidCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid command (OpenCode format: .opencode/commands/<name>.md)
	commandsDir := filepath.Join(tmpDir, ".opencode", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmdContent := `---
description: Get compact project knowledge brief
---

!taskwing slash brief
`
	if err := os.WriteFile(filepath.Join(commandsDir, "tw-brief.md"), []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to write tw-brief.md: %v", err)
	}

	checks := checkOpenCodeCommands(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for valid command")
	}

	if checks[0].Status != "ok" {
		t.Errorf("Expected status 'ok' for valid command, got %q with message %q", checks[0].Status, checks[0].Message)
	}
}

// TestDoctor_CheckOpenCodeCommands_MissingFrontmatter tests command without frontmatter.
func TestDoctor_CheckOpenCodeCommands_MissingFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create command without frontmatter
	commandsDir := filepath.Join(tmpDir, ".opencode", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmdContent := `# Just markdown, no frontmatter
This command is missing YAML frontmatter.
`
	if err := os.WriteFile(filepath.Join(commandsDir, "bad-cmd.md"), []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to write bad-cmd.md: %v", err)
	}

	checks := checkOpenCodeCommands(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for invalid command")
	}

	if checks[0].Status != "warn" {
		t.Errorf("Expected status 'warn' for missing frontmatter, got %q", checks[0].Status)
	}
}

// TestDoctor_CheckOpenCodeCommands_MissingDescription tests command missing description.
func TestDoctor_CheckOpenCodeCommands_MissingDescription(t *testing.T) {
	tmpDir := t.TempDir()

	// Create command with incomplete frontmatter (missing description)
	commandsDir := filepath.Join(tmpDir, ".opencode", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmdContent := `---
agent: coder
---

Missing description field.
`
	if err := os.WriteFile(filepath.Join(commandsDir, "incomplete.md"), []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to write incomplete.md: %v", err)
	}

	checks := checkOpenCodeCommands(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for incomplete command")
	}

	if checks[0].Status != "warn" {
		t.Errorf("Expected status 'warn' for missing description, got %q", checks[0].Status)
	}
}

// TestDoctor_CheckOpenCodeCommands_MultipleCommands tests multiple commands validation.
func TestDoctor_CheckOpenCodeCommands_MultipleCommands(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple valid commands
	commandsDir := filepath.Join(tmpDir, ".opencode", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	commands := []string{"tw-brief", "tw-next", "tw-done"}
	for _, cmdName := range commands {
		cmdContent := "---\ndescription: Test command " + cmdName + "\n---\n\n!taskwing slash " + cmdName + "\n"
		if err := os.WriteFile(filepath.Join(commandsDir, cmdName+".md"), []byte(cmdContent), 0644); err != nil {
			t.Fatalf("Failed to write %s.md: %v", cmdName, err)
		}
	}

	checks := checkOpenCodeCommands(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for multiple commands")
	}

	if checks[0].Status != "ok" {
		t.Errorf("Expected status 'ok' for valid commands, got %q", checks[0].Status)
	}

	// Verify message mentions the count
	if checks[0].Message != "3 commands configured" {
		t.Errorf("Expected message '3 commands configured', got %q", checks[0].Message)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func writeOpenCodeConfig(t *testing.T, dir string, config OpenCodeConfig) {
	t.Helper()
	configPath := filepath.Join(dir, "opencode.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
}
