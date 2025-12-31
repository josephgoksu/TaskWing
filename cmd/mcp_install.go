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

By default, installs locally in the current project directory for Cursor/Windsurf.
For Claude, installation is always global (User level).

Supported editors:
  - cursor    (Creates .cursor/mcp.json in current project)
  - windsurf  (Creates .windsurf/mcp.json in current project)
  - claude    (Configures local Claude Code CLI and Claude Desktop App)
  - gemini    (Creates .gemini/settings.json in project, or ~/.gemini/settings.json with --global)

Examples:
  taskwing mcp install cursor
  taskwing mcp install claude
  taskwing mcp install all`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Please specify an editor: cursor, windsurf, claude, gemini, or all")
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
		if globalInstall || target == "claude" {
			installType = "global"
		}

		ui.RenderPageHeader("TaskWing MCP Install", fmt.Sprintf("Configuring for %s (%s)", target, installType))
		fmt.Printf("Binary: %s\n", binPath)
		fmt.Printf("Project: %s\n", cwd)

		switch target {
		case "cursor":
			installLocalMCP(cwd, ".cursor", "mcp.json", binPath)
		case "windsurf":
			installLocalMCP(cwd, ".windsurf", "mcp.json", binPath)
		case "claude":
			installClaude(binPath, cwd)
		case "codex":
			installCodexGlobal(binPath, cwd)
		case "gemini":
			installGeminiGlobal(binPath, cwd)
		case "all":
			installLocalMCP(cwd, ".cursor", "mcp.json", binPath)
			installLocalMCP(cwd, ".windsurf", "mcp.json", binPath)
			installClaude(binPath, cwd)
			installCodexGlobal(binPath, cwd)
			if globalInstall {
				installGlobalMCP("gemini", binPath, cwd)
			} else {
				installLocalMCP(cwd, ".gemini", "settings.json", binPath)
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

// -----------------------------------------------------------------------------
// Naming Helpers — Single implementation for consistent server naming
// -----------------------------------------------------------------------------

// sanitizeProjectName returns a server-safe project name (no dots)
func sanitizeProjectName(projectDir string) string {
	name := filepath.Base(projectDir)
	return strings.ReplaceAll(name, ".", "_")
}

// mcpServerName returns the TaskWing MCP server name for a project
func mcpServerName(projectDir string) string {
	return fmt.Sprintf("taskwing-%s", sanitizeProjectName(projectDir))
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

func installLocalMCP(projectDir, configDirName, configFileName, binPath string) {
	configPath := filepath.Join(projectDir, configDirName, configFileName)

	err := upsertMCPServer(configPath, "taskwing", MCPServerConfig{
		Command: binPath,
		Args:    []string{"mcp"},
	})
	if err != nil {
		fmt.Printf("❌ Failed to install for %s: %v\n", configDirName, err)
		return
	}
	fmt.Printf("✅ Installed for %s in %s\n", strings.TrimPrefix(configDirName, "."), configPath)
}

func installGlobalMCP(app string, binPath, projectDir string) {
	switch app {
	case "gemini":
		installGeminiGlobal(binPath, projectDir)
	}
}

func installClaude(binPath, projectDir string) {
	// 1. Install for Claude Code CLI
	installClaudeCodeCLI(binPath, projectDir)

	// 2. Install for Claude Desktop (macOS only for now)
	installClaudeDesktop(binPath, projectDir)
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

	fmt.Println("👉 Configuring Claude Code CLI...")

	if viper.GetBool("preview") {
		fmt.Printf("[PREVIEW] Would run: claude mcp add --transport stdio %s -- %s mcp\n", serverName, binPath)
		fmt.Printf("✅ Would install for claude code as '%s'\n", serverName)
		return
	}

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

func installGeminiGlobal(binPath, projectDir string) {
	// Check if gemini CLI is available
	_, err := exec.LookPath("gemini")
	if err != nil {
		fmt.Println("❌ 'gemini' CLI not found in PATH.")
		fmt.Println("   Please install the Gemini CLI first to use this integration.")
		fmt.Println("   See: https://geminicli.com/docs/getting-started")
		return
	}

	serverName := mcpServerName(projectDir)
	fmt.Println("👉 Configuring Gemini CLI...")

	if viper.GetBool("preview") {
		fmt.Printf("[PREVIEW] Would run: gemini mcp add %s %s mcp --cwd %s\n", serverName, binPath, projectDir)
		return
	}

	// Run: gemini mcp add <name> <command> [args...] --cwd <cwd>
	// Note: 'gemini mcp add' syntax: gemini mcp add <name> <command> [args...]
	cmd := exec.Command("gemini", "mcp", "add", serverName, binPath, "mcp", "--cwd", projectDir)
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
	fmt.Println("👉 Configuring OpenAI Codex...")

	if viper.GetBool("preview") {
		fmt.Printf("[PREVIEW] Would run: codex mcp add %s -- %s mcp\n", serverName, binPath)
		return
	}

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
