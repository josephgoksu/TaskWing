/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// mcpInstallCmd represents the install command
var mcpInstallCmd = &cobra.Command{
	Use:   "install [editor]",
	Short: "Install MCP server configuration for an editor",
	Long: `Automatically configure your editor to use TaskWing's MCP server.

Supported editors:
  - cursor         (Creates .cursor/mcp.json in current project)
  - claude         (Configures Claude Code CLI)
  - claude-desktop (Configures Claude Desktop App)
  - gemini         (Configures Gemini CLI via 'gemini mcp add')
  - codex          (Configures OpenAI Codex CLI via 'codex mcp add')
  - copilot        (Creates .vscode/mcp.json for GitHub Copilot)
  - opencode       (Creates opencode.json at project root)

Examples:
  taskwing mcp install cursor
  taskwing mcp install claude
  taskwing mcp install copilot
  taskwing mcp install opencode
  taskwing mcp install all`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Please specify an editor: cursor, claude, claude-desktop, gemini, codex, copilot, opencode, or all")
			os.Exit(1)
		}

		globalInstall, _ := cmd.Flags().GetBool("global")
		target := strings.ToLower(args[0])
		binPath, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error determining binary path: %v\n", err)
			os.Exit(1)
		}
		// Clean and absolute path
		if absPath, err := filepath.Abs(binPath); err == nil {
			binPath = filepath.Clean(absPath)
		}

		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}

		installType := "local"
		if globalInstall || target == "claude" || target == "claude-desktop" {
			installType = "global"
		}

		ui.RenderPageHeader("TaskWing MCP Install", fmt.Sprintf("Configuring for %s (%s)", target, installType))
		fmt.Printf("Binary: %s\n", binPath)
		fmt.Printf("Project: %s\n", cwd)

		switch target {
		case "cursor":
			installLocalMCP(cwd, ".cursor", "mcp.json", binPath)
		case "claude":
			installClaude(binPath, cwd)
		case "claude-desktop":
			installClaudeDesktop(binPath, cwd)
		case "codex":
			installCodexGlobal(binPath, cwd)
		case "gemini":
			installGeminiCLI(binPath, cwd)
		case "copilot":
			installCopilot(binPath, cwd)
		case "opencode":
			if err := installOpenCode(binPath, cwd); err != nil {
				fmt.Printf("❌ Failed to install for OpenCode: %v\n", err)
				os.Exit(1)
			}
		case "all":
			installLocalMCP(cwd, ".cursor", "mcp.json", binPath)
			installClaude(binPath, cwd)
			installCodexGlobal(binPath, cwd)
			installGeminiCLI(binPath, cwd)
			installCopilot(binPath, cwd)
			if err := installOpenCode(binPath, cwd); err != nil {
				fmt.Printf("⚠️  OpenCode install failed: %v\n", err)
			}
		default:
			fmt.Printf("Unknown editor: %s\n", target)
			os.Exit(1)
		}
	},
}

func init() {
	mcpCmd.AddCommand(mcpInstallCmd)
	mcpInstallCmd.Flags().Bool("global", false, "Install globally in home directory instead of current project")
}

type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Cwd     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// VSCodeMCPServerConfig is the VS Code/Copilot MCP server format
// See: https://code.visualstudio.com/docs/copilot/customization/mcp-servers
type VSCodeMCPServerConfig struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

type VSCodeMCPConfig struct {
	Servers map[string]VSCodeMCPServerConfig `json:"servers"`
}

// OpenCodeMCPServerConfig represents a single MCP server entry in OpenCode's config.
// OpenCode uses a different format than other tools:
// - "type" must be "local" for command-based execution
// - "command" is an ARRAY of strings (not a single string)
// - Optional "environment" and "timeout" fields
// See: https://opencode.ai/docs/mcp-servers/
type OpenCodeMCPServerConfig struct {
	Type        string            `json:"type"`                  // Must be "local" for command execution
	Command     []string          `json:"command"`               // Array: ["taskwing", "mcp"]
	Environment map[string]string `json:"environment,omitempty"` // Optional env vars (no secrets!)
	Timeout     int               `json:"timeout,omitempty"`     // Optional timeout in ms (default 5000)
}

// OpenCodeConfig represents the top-level opencode.json configuration.
// File is placed at project root (NOT in a subdirectory).
type OpenCodeConfig struct {
	Schema string                             `json:"$schema,omitempty"` // Optional schema URL
	MCP    map[string]OpenCodeMCPServerConfig `json:"mcp"`               // MCP servers
}

// -----------------------------------------------------------------------------
// Naming Helpers — Single implementation for consistent server naming
// -----------------------------------------------------------------------------

// mcpServerName returns the TaskWing MCP server name for a project
// Uses a consistent name across all projects; AI tools differentiate by working directory
func mcpServerName(projectDir string) string {
	return "taskwing-mcp"
}

// legacyServerName returns the OLD server name format for migration cleanup
// Old format was: taskwing-{ProjectName} (with original casing, dots replaced with underscores)
func legacyServerName(projectDir string) string {
	name := filepath.Base(projectDir)
	name = strings.ReplaceAll(name, ".", "_")
	return fmt.Sprintf("taskwing-%s", name)
}

// removeMCPServer removes a server entry from an MCP config file if it exists.
// Used for cleaning up legacy server names during migration.
func removeMCPServer(configPath, serverName string) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return // File doesn't exist, nothing to remove
	}

	var config MCPConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return
	}

	if config.MCPServers == nil {
		return
	}

	if _, exists := config.MCPServers[serverName]; !exists {
		return // Server not in config
	}

	delete(config.MCPServers, serverName)
	_ = writeJSONFile(configPath, config) // Ignore error - best effort cleanup
}

// upsertMCPServer reads an MCP config file, adds/updates the server, and writes it back.
// This is the SINGLE implementation for all MCP config operations.
func upsertMCPServer(configPath, serverName string, serverCfg MCPServerConfig) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Read existing config or create empty
	var config MCPConfig
	if content, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(content, &config); err != nil {
			config.MCPServers = make(map[string]MCPServerConfig)
		}
	} else {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	// Upsert server
	config.MCPServers[serverName] = serverCfg

	// Write back
	return writeJSONFile(configPath, config)
}

// writeJSONFile writes data to a file as indented JSON.
func writeJSONFile(path string, data interface{}) error {
	if viper.GetBool("preview") {
		fmt.Printf("[PREVIEW] Would write JSON to %s\n", path)
		return nil
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	return os.WriteFile(path, bytes, 0644)
}

// upsertVSCodeMCPServer handles VS Code/Copilot MCP config format
// VS Code uses {"servers": {...}} instead of {"mcpServers": {...}}
func upsertVSCodeMCPServer(configPath, serverName string, serverCfg VSCodeMCPServerConfig) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Read existing config or create empty
	var config VSCodeMCPConfig
	if content, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(content, &config); err != nil {
			config.Servers = make(map[string]VSCodeMCPServerConfig)
		}
	} else {
		config.Servers = make(map[string]VSCodeMCPServerConfig)
	}

	if config.Servers == nil {
		config.Servers = make(map[string]VSCodeMCPServerConfig)
	}

	// Upsert server
	config.Servers[serverName] = serverCfg

	// Write back
	return writeJSONFile(configPath, config)
}

func installLocalMCP(projectDir, configDirName, configFileName, binPath string) {
	configPath := filepath.Join(projectDir, configDirName, configFileName)
	serverName := mcpServerName(projectDir)

	err := upsertMCPServer(configPath, serverName, MCPServerConfig{
		Command: binPath,
		Args:    []string{"mcp"},
	})
	if err != nil {
		fmt.Printf("❌ Failed to install for %s: %v\n", configDirName, err)
		return
	}
	fmt.Printf("✅ Installed for %s as '%s' in %s\n", strings.TrimPrefix(configDirName, "."), serverName, configPath)
}

func installClaude(binPath, projectDir string) {
	// Install for Claude Code CLI only
	// Claude Desktop is skipped by default (use 'tw mcp install claude-desktop' if needed)
	installClaudeCodeCLI(binPath, projectDir)
}

func installClaudeCodeCLI(binPath, projectDir string) {
	// Check if claude CLI is available
	_, err := exec.LookPath("claude")
	if err != nil {
		if viper.GetBool("verbose") {
			fmt.Println("ℹ️  Claude Code CLI not found (skipping CLI config)")
		}
		return
	}

	serverName := mcpServerName(projectDir)
	legacyName := legacyServerName(projectDir)

	fmt.Println("👉 Configuring Claude Code CLI...")

	if viper.GetBool("preview") {
		fmt.Printf("[PREVIEW] Would run: claude mcp remove %s && claude mcp remove %s && claude mcp add --transport stdio %s -- %s mcp\n", legacyName, serverName, serverName, binPath)
		fmt.Printf("✅ Would install for Claude Code as '%s'\n", serverName)
		return
	}

	// Remove legacy server name first (migration cleanup)
	legacyRemoveCmd := exec.Command("claude", "mcp", "remove", legacyName)
	legacyRemoveCmd.Dir = projectDir
	_ = legacyRemoveCmd.Run() // Ignore error - server may not exist

	// Remove current server name (idempotent reinstall)
	removeCmd := exec.Command("claude", "mcp", "remove", serverName)
	removeCmd.Dir = projectDir
	_ = removeCmd.Run() // Ignore error - server may not exist

	// Run: claude mcp add --transport stdio <name> -- <binPath> mcp
	cmd := exec.Command("claude", "mcp", "add", "--transport", "stdio", serverName, "--", binPath, "mcp")
	cmd.Dir = projectDir
	// Capture output to suppress noise, unless verbose
	if viper.GetBool("verbose") {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  Failed to run 'claude mcp add': %v\n", err)
	} else {
		fmt.Printf("✅ Installed for Claude Code as '%s'\n", serverName)
	}
}

func installClaudeDesktop(binPath, projectDir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	// macOS standard path
	configPath := filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	if _, err := os.Stat(filepath.Dir(configPath)); os.IsNotExist(err) {
		// Claude Desktop not installed or different OS
		return
	}

	fmt.Println("👉 Configuring Claude Desktop App...")

	serverName := mcpServerName(projectDir)
	legacyName := legacyServerName(projectDir)

	// Remove legacy server name (migration cleanup)
	removeMCPServer(configPath, legacyName)

	err = upsertMCPServer(configPath, serverName, MCPServerConfig{
		Command: binPath,
		Args:    []string{"mcp"},
		Env:     map[string]string{},
	})
	if err != nil {
		fmt.Printf("⚠️  Failed to configure Claude Desktop: %v\n", err)
		return
	}
	fmt.Printf("✅ Installed for Claude Desktop as '%s' in %s\n", serverName, configPath)
	fmt.Println("   (You may need to restart Claude Desktop to see the changes)")
}

// installCopilot configures MCP for GitHub Copilot in VS Code
// Uses .vscode/mcp.json with VS Code's MCP format
// See: https://code.visualstudio.com/docs/copilot/customization/mcp-servers
func installCopilot(binPath, projectDir string) {
	configPath := filepath.Join(projectDir, ".vscode", "mcp.json")
	serverName := mcpServerName(projectDir)

	fmt.Println("👉 Configuring GitHub Copilot (VS Code)...")

	err := upsertVSCodeMCPServer(configPath, serverName, VSCodeMCPServerConfig{
		Type:    "stdio",
		Command: binPath,
		Args:    []string{"mcp"},
	})
	if err != nil {
		fmt.Printf("❌ Failed to install for Copilot: %v\n", err)
		return
	}
	fmt.Printf("✅ Installed for GitHub Copilot as '%s' in %s\n", serverName, configPath)
	fmt.Println("   (Reload VS Code window to activate)")
}

func installGeminiCLI(binPath, projectDir string) {
	// Check if gemini CLI is available
	_, err := exec.LookPath("gemini")
	if err != nil {
		fmt.Println("❌ 'gemini' CLI not found in PATH.")
		fmt.Println("   Please install the Gemini CLI first to use this integration.")
		fmt.Println("   See: https://geminicli.com/docs/getting-started")
		return
	}

	serverName := mcpServerName(projectDir)
	legacyName := legacyServerName(projectDir)
	fmt.Println("👉 Configuring Gemini CLI...")

	if viper.GetBool("preview") {
		fmt.Printf("[PREVIEW] Would run: gemini mcp remove -s project %s && gemini mcp add -s project %s %s mcp\n", legacyName, serverName, binPath)
		return
	}

	// Remove legacy server name (migration cleanup)
	legacyRemoveCmd := exec.Command("gemini", "mcp", "remove", "-s", "project", legacyName)
	legacyRemoveCmd.Dir = projectDir
	_ = legacyRemoveCmd.Run() // Ignore error - server may not exist

	// Remove current server name (idempotent reinstall)
	removeCmd := exec.Command("gemini", "mcp", "remove", "-s", "project", serverName)
	removeCmd.Dir = projectDir
	_ = removeCmd.Run() // Ignore error - server may not exist

	// Run: gemini mcp add -s project <name> <command> [args...]
	// Uses -s project for project-level config (stored in .gemini/settings.json)
	cmd := exec.Command("gemini", "mcp", "add", "-s", "project", serverName, binPath, "mcp")
	cmd.Dir = projectDir

	// Capture output to suppress noise, unless verbose
	if viper.GetBool("verbose") {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  Failed to run 'gemini mcp add': %v\n", err)
	} else {
		fmt.Printf("✅ Installed for Gemini as '%s'\n", serverName)
	}
}

func installCodexGlobal(binPath, projectDir string) {
	// Check if codex CLI is available
	_, err := exec.LookPath("codex")
	if err != nil {
		fmt.Println("❌ 'codex' CLI not found in PATH.")
		fmt.Println("   Please install the OpenAI Codex CLI first to use this integration.")
		fmt.Println("   See: https://developers.openai.com/codex/mcp/")
		return
	}

	serverName := mcpServerName(projectDir)
	legacyName := legacyServerName(projectDir)
	fmt.Println("👉 Configuring OpenAI Codex...")

	if viper.GetBool("preview") {
		fmt.Printf("[PREVIEW] Would run: codex mcp remove %s && codex mcp add %s -- %s mcp\n", legacyName, serverName, binPath)
		return
	}

	// Remove legacy server name (migration cleanup)
	legacyRemoveCmd := exec.Command("codex", "mcp", "remove", legacyName)
	legacyRemoveCmd.Dir = projectDir
	_ = legacyRemoveCmd.Run() // Ignore error - server may not exist

	// Remove current server name (idempotent reinstall)
	removeCmd := exec.Command("codex", "mcp", "remove", serverName)
	removeCmd.Dir = projectDir
	_ = removeCmd.Run() // Ignore error - server may not exist

	// Run: codex mcp add <name> -- <binPath> mcp
	cmd := exec.Command("codex", "mcp", "add", serverName, "--", binPath, "mcp")
	cmd.Dir = projectDir

	// Capture output to suppress noise, unless verbose
	if viper.GetBool("verbose") {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  Failed to run 'codex mcp add': %v\n", err)
	} else {
		fmt.Printf("✅ Installed for Codex as '%s'\n", serverName)
	}
}

// installOpenCode configures MCP for OpenCode by creating/updating opencode.json
// at the project root. OpenCode's config format differs from other tools:
// - File is at project root (opencode.json), not in a subdirectory
// - Uses "$schema" and "mcp" top-level keys
// - Command is an array, not a string
// - Type must be "local" for command execution
// See: https://opencode.ai/docs/mcp-servers/
func installOpenCode(binPath, projectDir string) error {
	configPath := filepath.Join(projectDir, "opencode.json")
	serverName := mcpServerName(projectDir)

	fmt.Println("👉 Configuring OpenCode...")

	return upsertOpenCodeMCPServer(configPath, serverName, OpenCodeMCPServerConfig{
		Type:    "local",
		Command: []string{binPath, "mcp"},
		Timeout: 5000, // Default 5s timeout
	})
}

// upsertOpenCodeMCPServer handles OpenCode's unique config format
// Unlike other tools, OpenCode uses:
// - Project root file: opencode.json
// - Top-level "$schema" and "mcp" keys
// - Command as array: ["taskwing", "mcp"]
func upsertOpenCodeMCPServer(configPath, serverName string, serverCfg OpenCodeMCPServerConfig) error {
	// Validate inputs
	if configPath == "" {
		return fmt.Errorf("configPath cannot be empty")
	}
	if serverName == "" {
		return fmt.Errorf("serverName cannot be empty")
	}
	if len(serverCfg.Command) == 0 {
		return fmt.Errorf("command array cannot be empty")
	}
	if serverCfg.Type != "local" {
		return fmt.Errorf("type must be 'local' for OpenCode MCP servers, got: %s", serverCfg.Type)
	}

	// Read existing config or create new
	var config OpenCodeConfig
	if content, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(content, &config); err != nil {
			// Invalid JSON - start fresh but preserve what we can
			config = OpenCodeConfig{
				Schema: "https://opencode.ai/config.json",
				MCP:    make(map[string]OpenCodeMCPServerConfig),
			}
		}
	} else {
		// File doesn't exist - create new config
		config = OpenCodeConfig{
			Schema: "https://opencode.ai/config.json",
			MCP:    make(map[string]OpenCodeMCPServerConfig),
		}
	}

	// Ensure MCP map exists
	if config.MCP == nil {
		config.MCP = make(map[string]OpenCodeMCPServerConfig)
	}

	// Ensure schema is set
	if config.Schema == "" {
		config.Schema = "https://opencode.ai/config.json"
	}

	// Upsert server
	config.MCP[serverName] = serverCfg

	// Write back
	if err := writeJSONFile(configPath, config); err != nil {
		return fmt.Errorf("write opencode.json: %w", err)
	}

	fmt.Printf("✅ Installed for OpenCode as '%s' in %s\n", serverName, configPath)
	fmt.Println("   (opencode.json is at project root per OpenCode spec)")
	return nil
}
