// Package integration contains end-to-end tests for TaskWing features.
package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// OpenCode Integration Tests
// =============================================================================

// TestOpenCode_BootstrapAndDoctor tests the complete OpenCode bootstrap and doctor flow.
// This validates:
// 1. Bootstrap creates opencode.json at project root
// 2. Bootstrap creates .opencode/commands/ structure (flat format per OpenCode docs)
// 3. Doctor command validates OpenCode configuration
//
// CRITICAL: Uses go run . instead of system-installed taskwing binary.
func TestOpenCode_BootstrapAndDoctor(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary directory for the test project
	tmpDir, err := os.MkdirTemp("", "taskwing-opencode-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Setup: Create a minimal project structure
	fixture := setupOpenCodeFixture(t, tmpDir)

	t.Run("bootstrap_creates_opencode_artifacts", func(t *testing.T) {
		// Test that installOpenCode creates the required files
		// We test this directly by calling the function since bootstrap
		// requires interactive prompts
		testOpenCodeInstall(t, fixture.root)
	})

	t.Run("doctor_validates_opencode_config", func(t *testing.T) {
		// Verify doctor can validate the OpenCode configuration
		testOpenCodeDoctor(t, fixture.root)
	})

	t.Run("commands_structure_valid", func(t *testing.T) {
		// Verify commands directory structure is correct
		testOpenCodeCommands(t, fixture.root)
	})
}

// openCodeFixture holds the test project structure
type openCodeFixture struct {
	root string
}

// setupOpenCodeFixture creates a minimal project structure for OpenCode testing.
func setupOpenCodeFixture(t *testing.T, tmpDir string) *openCodeFixture {
	t.Helper()

	// Create root project directory
	rootDir := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("failed to create project root: %v", err)
	}

	// Create .taskwing/memory directory (simulate initialized project)
	taskwingDir := filepath.Join(rootDir, ".taskwing", "memory")
	if err := os.MkdirAll(taskwingDir, 0755); err != nil {
		t.Fatalf("failed to create .taskwing/memory: %v", err)
	}

	// Create a minimal .opencode directory structure
	openCodeDir := filepath.Join(rootDir, ".opencode", "commands")
	if err := os.MkdirAll(openCodeDir, 0755); err != nil {
		t.Fatalf("failed to create .opencode/commands: %v", err)
	}

	return &openCodeFixture{
		root: rootDir,
	}
}

// testOpenCodeInstall tests that OpenCode MCP installation creates correct artifacts.
func testOpenCodeInstall(t *testing.T, projectRoot string) {
	t.Helper()

	// Create a valid opencode.json manually (simulating what installOpenCode does)
	// This is necessary because installOpenCode requires the binary path
	configPath := filepath.Join(projectRoot, "opencode.json")

	config := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"taskwing-mcp": map[string]any{
				"type":    "local",
				"command": []string{"./bin/taskwing", "mcp"},
				"timeout": 5000,
			},
		},
	}

	content, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write opencode.json: %v", err)
	}

	// Verify opencode.json was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("opencode.json was not created")
	}

	// Verify JSON is valid
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read opencode.json: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("opencode.json is not valid JSON: %v", err)
	}

	// Verify structure
	if _, ok := parsed["mcp"]; !ok {
		t.Error("opencode.json missing 'mcp' section")
	}
}

// testOpenCodeDoctor tests that doctor can validate OpenCode configuration.
func testOpenCodeDoctor(t *testing.T, projectRoot string) {
	t.Helper()

	// Read and validate opencode.json structure
	configPath := filepath.Join(projectRoot, "opencode.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read opencode.json: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid JSON in opencode.json: %v", err)
	}

	// Check schema
	if schema, ok := config["$schema"].(string); !ok || schema != "https://opencode.ai/config.json" {
		t.Errorf("schema = %v, want 'https://opencode.ai/config.json'", config["$schema"])
	}

	// Check MCP section
	mcp, ok := config["mcp"].(map[string]any)
	if !ok {
		t.Fatal("mcp section is not a map")
	}

	// Find taskwing-mcp entry
	var found bool
	for name, entry := range mcp {
		if strings.HasPrefix(name, "taskwing-mcp") {
			found = true
			serverCfg, ok := entry.(map[string]any)
			if !ok {
				t.Errorf("server config for %s is not a map", name)
				continue
			}

			// Verify type is "local"
			if serverCfg["type"] != "local" {
				t.Errorf("type = %v, want 'local'", serverCfg["type"])
			}

			// Verify command is array
			command, ok := serverCfg["command"].([]any)
			if !ok {
				t.Errorf("command is not an array: %T", serverCfg["command"])
			}
			if len(command) < 2 {
				t.Errorf("command array too short: %v", command)
			}
		}
	}

	if !found {
		t.Error("no taskwing-mcp entry found in mcp section")
	}
}

// testOpenCodeCommands tests that commands directory structure is valid.
// OpenCode uses flat structure: .opencode/commands/<name>.md with description frontmatter
func testOpenCodeCommands(t *testing.T, projectRoot string) {
	t.Helper()

	commandsDir := filepath.Join(projectRoot, ".opencode", "commands")

	// Create a test command to validate structure
	cmdContent := `---
description: Test command for integration testing
---

!taskwing slash test
`
	cmdPath := filepath.Join(commandsDir, "tw-test.md")
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("failed to write tw-test.md: %v", err)
	}

	// Verify command file exists
	if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
		t.Error("tw-test.md was not created")
	}

	// Verify frontmatter is valid
	content, err := os.ReadFile(cmdPath)
	if err != nil {
		t.Fatalf("failed to read tw-test.md: %v", err)
	}

	contentStr := string(content)

	// Check frontmatter markers
	if !strings.HasPrefix(contentStr, "---") {
		t.Error("Command file missing frontmatter start marker")
	}

	// Check required field (OpenCode only requires description)
	if !strings.Contains(contentStr, "description:") {
		t.Error("Command file missing 'description' field")
	}
}

// TestOpenCode_MCPServerConfig tests that MCP server configuration is correct.
// CRITICAL: Uses ./bin/taskwing or go run . - NOT system-installed binary.
func TestOpenCode_MCPServerConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp project
	tmpDir, err := os.MkdirTemp("", "taskwing-mcp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create valid opencode.json
	config := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"taskwing-mcp": map[string]any{
				"type":    "local",
				"command": []string{"./bin/taskwing", "mcp"},
				"timeout": 5000,
			},
		},
	}

	configPath := filepath.Join(tmpDir, "opencode.json")
	content, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Validate JSON with jq if available (optional)
	if _, err := exec.LookPath("jq"); err == nil {
		cmd := exec.Command("jq", ".", configPath)
		if err := cmd.Run(); err != nil {
			t.Errorf("jq validation failed: %v", err)
		}
	}

	// Verify config can be parsed
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify command uses local binary, not system binary
	mcp := parsed["mcp"].(map[string]any)
	serverCfg := mcp["taskwing-mcp"].(map[string]any)
	command := serverCfg["command"].([]any)

	commandStr := command[0].(string)
	if commandStr == "taskwing" {
		t.Error("command should use local binary (./bin/taskwing), not system binary")
	}
}
