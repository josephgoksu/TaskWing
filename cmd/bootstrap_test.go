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

// TestInstallMCPServers_OpenCode tests that installMCPServers correctly installs OpenCode MCP config.
func TestInstallMCPServers_OpenCode(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock binPath - in tests we can use any path
	binPath := "/usr/local/bin/taskwing"

	// Call installMCPServers with opencode
	installMCPServers(tmpDir, []string{"opencode"})

	// Verify opencode.json was created
	configPath := filepath.Join(tmpDir, "opencode.json")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read opencode.json: %v", err)
	}

	// Parse and verify structure
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

	// Server name should include project directory name
	expectedServerName := "taskwing-mcp-" + filepath.Base(tmpDir)
	serverCfg, ok := config.MCP[expectedServerName]
	if !ok {
		// Check if any taskwing-mcp server exists
		found := false
		for name := range config.MCP {
			if len(name) >= 12 && name[:12] == "taskwing-mcp" {
				found = true
				serverCfg = config.MCP[name]
				break
			}
		}
		if !found {
			t.Fatalf("No taskwing-mcp server entry found in MCP section. Got: %v", config.MCP)
		}
	}

	// Verify type is "local"
	if serverCfg.Type != "local" {
		t.Errorf("Type = %q, want %q", serverCfg.Type, "local")
	}

	// Verify command is array format
	if len(serverCfg.Command) != 2 {
		t.Fatalf("Command length = %d, want 2", len(serverCfg.Command))
	}
	// Command[0] will use the actual executable path, not our mock binPath
	// Just verify the second element is "mcp"
	if serverCfg.Command[1] != "mcp" {
		t.Errorf("Command[1] = %q, want %q", serverCfg.Command[1], "mcp")
	}

	_ = binPath // suppress unused variable warning
}

// TestInstallMCPServers_AllIncludesOpenCode tests that "all" AIs doesn't break when including opencode.
func TestInstallMCPServers_AllIncludesOpenCode(t *testing.T) {
	tmpDir := t.TempDir()

	// Install multiple AIs including opencode
	installMCPServers(tmpDir, []string{"claude", "opencode"})

	// Verify opencode.json was created
	configPath := filepath.Join(tmpDir, "opencode.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("opencode.json was not created when installing multiple AIs including opencode")
	}
}

// TestAIConfigOrder_IncludesOpenCode verifies opencode is in the AI selection list.
func TestAIConfigOrder_IncludesOpenCode(t *testing.T) {
	found := false
	for _, ai := range aiConfigOrder {
		if ai == "opencode" {
			found = true
			break
		}
	}
	if !found {
		t.Error("opencode is not in aiConfigOrder")
	}
}

// TestAIConfigs_OpenCodeEntry verifies opencode config entry exists with correct values.
func TestAIConfigs_OpenCodeEntry(t *testing.T) {
	cfg, ok := aiConfigs["opencode"]
	if !ok {
		t.Fatal("opencode entry not found in aiConfigs")
	}

	if cfg.name != "opencode" {
		t.Errorf("name = %q, want %q", cfg.name, "opencode")
	}
	if cfg.displayName != "OpenCode" {
		t.Errorf("displayName = %q, want %q", cfg.displayName, "OpenCode")
	}
	if cfg.commandsDir != ".opencode/skills" {
		t.Errorf("commandsDir = %q, want %q", cfg.commandsDir, ".opencode/skills")
	}
	if cfg.fileExt != ".md" {
		t.Errorf("fileExt = %q, want %q", cfg.fileExt, ".md")
	}
}
