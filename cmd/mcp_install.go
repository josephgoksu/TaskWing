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
		fmt.Printf("Installing TaskWing MCP for %s (%s)...\n", target, installType)
		fmt.Printf("Binary: %s\n", binPath)
		fmt.Printf("Project: %s\n", cwd)

		switch target {
		case "cursor":
			installLocalMCP(cwd, ".cursor", "mcp.json", binPath)
		case "windsurf":
			installLocalMCP(cwd, ".windsurf", "mcp.json", binPath)
		case "claude":
			installClaude(binPath, cwd)
		case "gemini":
			if globalInstall {
				installGlobalMCP("gemini", binPath, cwd)
			} else {
				installLocalMCP(cwd, ".gemini", "settings.json", binPath)
			}
		case "all":
			installLocalMCP(cwd, ".cursor", "mcp.json", binPath)
			installLocalMCP(cwd, ".windsurf", "mcp.json", binPath)
			installClaude(binPath, cwd)
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

func installLocalMCP(projectDir, configDirName, configFileName, binPath string) {
	configDir := filepath.Join(projectDir, configDirName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("❌ Failed to create %s directory: %v\n", configDirName, err)
		return
	}

	configFile := filepath.Join(configDir, configFileName)

	// Check if file exists to avoid overwriting other servers (though unlikely in a pure .cursor/mcp.json)
	// For Cursor/Windsurf, it's usually safe to read/modify.

	var config MCPConfig
	if content, err := os.ReadFile(configFile); err == nil {
		if err := json.Unmarshal(content, &config); err != nil {
			// If invalid JSON, start fresh but warn
			fmt.Printf("⚠️  Existing %s/%s was invalid, creating new one.\n", configDirName, configFileName)
			config.MCPServers = make(map[string]MCPServerConfig)
		}
	} else {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	config.MCPServers["taskwing"] = MCPServerConfig{
		Command: binPath,
		Args:    []string{"mcp"},
	}

	writeJSON(configFile, config)
	fmt.Printf("✅ Installed for %s in %s\n", strings.TrimPrefix(configDirName, "."), configFile)
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
	projectName := filepath.Base(projectDir)
	// Sanitize: replace dots with underscores (Claude CLI doesn't allow dots in names)
	projectName = strings.ReplaceAll(projectName, ".", "_")
	serverName := fmt.Sprintf("taskwing-%s", projectName)

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
		fmt.Printf("   (This is expected if 'claude' CLI is not installed)\n")
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

	var config MCPConfig
	// Read existing config or create new
	if content, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(content, &config); err != nil {
			fmt.Printf("⚠️  Existing Claude Desktop config was invalid json, skipping integration.\n")
			return
		}
	} else {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	projectName := filepath.Base(projectDir)
	serverName := fmt.Sprintf("taskwing-%s", projectName)

	// Update configuration
	config.MCPServers[serverName] = MCPServerConfig{
		Command: binPath,
		Args:    []string{"mcp"},
		Env:     map[string]string{}, // Claude desktop might need empty env to not inherit messy envs
	}

	writeJSON(configPath, config)
	fmt.Printf("✅ Installed for Claude Desktop as '%s' in %s\n", serverName, configPath)
	fmt.Println("   (You may need to restart Claude Desktop to see the changes)")
}

func installGeminiGlobal(binPath, projectDir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("❌ Could not find home directory: %v\n", err)
		return
	}

	configFile := filepath.Join(home, ".gemini", "settings.json")

	// Ensure dir exists
	configDir := filepath.Dir(configFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("❌ Failed to create config directory for gemini: %v\n", err)
		return
	}

	var config MCPConfig

	// Read existing
	if content, err := os.ReadFile(configFile); err == nil {
		if err := json.Unmarshal(content, &config); err != nil {
			fmt.Printf("❌ Failed to parse existing gemini config. Please add manually.\n")
			return
		}
	} else {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	projectName := filepath.Base(projectDir)
	serverName := fmt.Sprintf("taskwing-%s", projectName)

	config.MCPServers[serverName] = MCPServerConfig{
		Command: binPath,
		Args:    []string{"mcp"},
		Cwd:     projectDir,
	}

	writeJSON(configFile, config)
	fmt.Printf("✅ Installed for gemini as '%s' in %s\n", serverName, configFile)
}

func writeJSON(path string, data interface{}) {
	if viper.GetBool("preview") {
		fmt.Printf("[PREVIEW] Would write JSON to %s\n", path)
		return
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("❌ Failed to marshal JSON: %v\n", err)
		return
	}

	if err := os.WriteFile(path, bytes, 0644); err != nil {
		fmt.Printf("❌ Failed to write config file: %v\n", err)
	}
}
