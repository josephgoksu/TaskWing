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
// OpenCode Skills Doctor Tests
// =============================================================================

// TestDoctor_CheckOpenCodeSkills_NoSkillsDir tests that missing skills dir is not an error.
func TestDoctor_CheckOpenCodeSkills_NoSkillsDir(t *testing.T) {
	tmpDir := t.TempDir()

	checks := checkOpenCodeSkills(tmpDir)

	// No skills directory should return empty checks
	if len(checks) != 0 {
		t.Errorf("Expected 0 checks when no skills directory exists, got %d", len(checks))
	}
}

// TestDoctor_CheckOpenCodeSkills_ValidSkill tests valid skill passes.
func TestDoctor_CheckOpenCodeSkills_ValidSkill(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid skill
	skillDir := filepath.Join(tmpDir, ".opencode", "skills", "tw-brief")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillContent := `---
name: tw-brief
description: Get compact project knowledge brief
---

# tw-brief

This skill provides project context.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	checks := checkOpenCodeSkills(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for valid skill")
	}

	if checks[0].Status != "ok" {
		t.Errorf("Expected status 'ok' for valid skill, got %q with message %q", checks[0].Status, checks[0].Message)
	}
}

// TestDoctor_CheckOpenCodeSkills_MissingFrontmatter tests skill without frontmatter.
func TestDoctor_CheckOpenCodeSkills_MissingFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create skill without frontmatter
	skillDir := filepath.Join(tmpDir, ".opencode", "skills", "bad-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillContent := `# Just markdown, no frontmatter
This skill is missing YAML frontmatter.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	checks := checkOpenCodeSkills(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for invalid skill")
	}

	if checks[0].Status != "warn" {
		t.Errorf("Expected status 'warn' for missing frontmatter, got %q", checks[0].Status)
	}
}

// TestDoctor_CheckOpenCodeSkills_MissingRequiredFields tests skill missing name/description.
func TestDoctor_CheckOpenCodeSkills_MissingRequiredFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Create skill with incomplete frontmatter
	skillDir := filepath.Join(tmpDir, ".opencode", "skills", "incomplete")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillContent := `---
name: incomplete
---

Missing description field.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	checks := checkOpenCodeSkills(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for incomplete skill")
	}

	if checks[0].Status != "warn" {
		t.Errorf("Expected status 'warn' for missing description, got %q", checks[0].Status)
	}
}

// TestDoctor_CheckOpenCodeSkills_NameMismatch tests skill with name not matching directory.
func TestDoctor_CheckOpenCodeSkills_NameMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create skill with mismatched name
	skillDir := filepath.Join(tmpDir, ".opencode", "skills", "tw-next")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillContent := `---
name: tw-different
description: Name doesn't match directory
---

The name field doesn't match the directory name.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	checks := checkOpenCodeSkills(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for mismatched name")
	}

	if checks[0].Status != "warn" {
		t.Errorf("Expected status 'warn' for name mismatch, got %q", checks[0].Status)
	}
}

// TestDoctor_CheckOpenCodeSkills_MultipleSkills tests multiple skills validation.
func TestDoctor_CheckOpenCodeSkills_MultipleSkills(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple valid skills
	skills := []string{"tw-brief", "tw-next", "tw-done"}
	for _, skillName := range skills {
		skillDir := filepath.Join(tmpDir, ".opencode", "skills", skillName)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("Failed to create skill dir: %v", err)
		}

		skillContent := "---\nname: " + skillName + "\ndescription: Test skill\n---\n\nContent.\n"
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
			t.Fatalf("Failed to write SKILL.md: %v", err)
		}
	}

	checks := checkOpenCodeSkills(tmpDir)

	if len(checks) == 0 {
		t.Fatal("Expected at least one check for multiple skills")
	}

	if checks[0].Status != "ok" {
		t.Errorf("Expected status 'ok' for valid skills, got %q", checks[0].Status)
	}

	// Verify message mentions the count
	if checks[0].Message != "3 skills configured" {
		t.Errorf("Expected message '3 skills configured', got %q", checks[0].Message)
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
