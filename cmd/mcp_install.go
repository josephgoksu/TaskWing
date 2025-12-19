/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// mcpInstallCmd represents the install command
var mcpInstallCmd = &cobra.Command{
	Use:   "install [editor]",
	Short: "Install MCP server configuration for an editor",
	Long: `Automatically configure your editor to use TaskWing's MCP server.

By default, installs locally in the current project directory.
Use --global to install in your home directory (for Claude/Gemini).

Supported editors:
  - cursor    (Creates .cursor/mcp.json in current project)
  - windsurf  (Creates .windsurf/mcp.json in current project)
  - claude    (Creates .claude/mcp.json in project, or ~/.claude/mcp.json with --global)
  - gemini    (Creates .gemini/settings.json in project, or ~/.gemini/settings.json with --global)

Examples:
  taskwing mcp install cursor
  taskwing mcp install claude
  taskwing mcp install claude --global
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
		if globalInstall {
			installType = "global"
		}
		fmt.Printf("Installing TaskWing MCP for %s (%s)...\n", target, installType)
		fmt.Printf("Binary: %s\n", binPath)
		fmt.Printf("Project: %s\n", cwd)

		switch target {
		case "cursor":
			installLocalMCP(cwd, ".cursor", binPath)
		case "windsurf":
			installLocalMCP(cwd, ".windsurf", binPath)
		case "claude":
			if globalInstall {
				installGlobalMCP("claude", binPath, cwd)
			} else {
				installLocalMCP(cwd, ".claude", binPath)
			}
		case "gemini":
			if globalInstall {
				installGlobalMCP("gemini", binPath, cwd)
			} else {
				installLocalMCP(cwd, ".gemini", binPath)
			}
		case "all":
			installLocalMCP(cwd, ".cursor", binPath)
			installLocalMCP(cwd, ".windsurf", binPath)
			if globalInstall {
				installGlobalMCP("claude", binPath, cwd)
				installGlobalMCP("gemini", binPath, cwd)
			} else {
				installLocalMCP(cwd, ".claude", binPath)
				installLocalMCP(cwd, ".gemini", binPath)
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
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Cwd     string   `json:"cwd,omitempty"`
}

type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

func installLocalMCP(projectDir, configDirName, binPath string) {
	configDir := filepath.Join(projectDir, configDirName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("❌ Failed to create %s directory: %v\n", configDirName, err)
		return
	}

	configFile := filepath.Join(configDir, "mcp.json")

	// Check if file exists to avoid overwriting other servers (though unlikely in a pure .cursor/mcp.json)
	// For Cursor/Windsurf, it's usually safe to read/modify.

	var config MCPConfig
	if content, err := os.ReadFile(configFile); err == nil {
		if err := json.Unmarshal(content, &config); err != nil {
			// If invalid JSON, start fresh but warn
			fmt.Printf("⚠️  Existing %s/mcp.json was invalid, creating new one.\n", configDirName)
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
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("❌ Could not find home directory: %v\n", err)
		return
	}

	var configFile string
	switch app {
	case "claude":
		configFile = filepath.Join(home, ".claude", "mcp.json")
	case "gemini":
		configFile = filepath.Join(home, ".gemini", "settings.json")
	}

	// Ensure dir exists
	configDir := filepath.Dir(configFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("❌ Failed to create config directory for %s: %v\n", app, err)
		return
	}

	var config MCPConfig

	// Read existing
	if content, err := os.ReadFile(configFile); err == nil {
		if err := json.Unmarshal(content, &config); err != nil {
			fmt.Printf("❌ Failed to parse existing %s config (it might contain comments or be invalid JSON).\n   Please add manually.\n", app)
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
	fmt.Printf("✅ Installed for %s as '%s' in %s\n", app, serverName, configFile)
}

func writeJSON(path string, data interface{}) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("❌ Failed to marshal JSON: %v\n", err)
		return
	}

	if err := os.WriteFile(path, bytes, 0644); err != nil {
		fmt.Printf("❌ Failed to write config file: %v\n", err)
	}
}
