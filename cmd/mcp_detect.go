/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// mcpDetectTimeout is the maximum time to wait for CLI commands during detection.
// Keep this short to avoid blocking bootstrap if a CLI is broken/hanging.
const mcpDetectTimeout = 3 * time.Second

// detectExistingMCPConfigs checks for existing TaskWing MCP configurations.
// Returns a list of AI assistants that have TaskWing already configured globally.
// This is used during bootstrap to avoid re-prompting for AI selection.
func detectExistingMCPConfigs() []string {
	var found []string

	if detectClaudeMCP() {
		found = append(found, "claude")
	}
	if detectGeminiMCP() {
		found = append(found, "gemini")
	}
	if detectCodexMCP() {
		found = append(found, "codex")
	}
	// Note: Cursor and Copilot are project-local (.cursor/mcp.json, .vscode/mcp.json)
	// so they don't need global detection

	return found
}

// detectClaudeMCP checks if Claude Code CLI has taskwing-mcp configured.
func detectClaudeMCP() bool {
	// First check if claude CLI is available
	_, err := exec.LookPath("claude")
	if err != nil {
		return false
	}

	// Run: claude mcp list (with timeout to prevent hanging)
	ctx, cancel := context.WithTimeout(context.Background(), mcpDetectTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "mcp", "list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "taskwing-mcp")
}

// detectGeminiMCP checks if Gemini CLI has taskwing-mcp configured.
func detectGeminiMCP() bool {
	// First check if gemini CLI is available
	_, err := exec.LookPath("gemini")
	if err != nil {
		return false
	}

	// Run: gemini mcp list (with timeout to prevent hanging)
	// Note: Gemini stores MCP config in project-level .gemini/settings.json,
	// but the CLI can report all configured servers
	ctx, cancel := context.WithTimeout(context.Background(), mcpDetectTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gemini", "mcp", "list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "taskwing-mcp")
}

// detectCodexMCP checks if Codex CLI has taskwing-mcp configured.
func detectCodexMCP() bool {
	// First check if codex CLI is available
	_, err := exec.LookPath("codex")
	if err != nil {
		return false
	}

	// Run: codex mcp list (with timeout to prevent hanging)
	ctx, cancel := context.WithTimeout(context.Background(), mcpDetectTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "codex", "mcp", "list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "taskwing-mcp")
}

// aiConfigDirs maps AI names to their local config directory names
var aiConfigDirs = map[string]string{
	"claude":  ".claude",
	"codex":   ".codex",
	"gemini":  ".gemini",
	"cursor":  ".cursor",
	"copilot": ".github", // Copilot uses .github/copilot-instructions
}

// aiConfigFiles maps AI names to their required config files (relative to config dir).
// Used to validate that a config directory has valid TaskWing configuration.
var aiConfigFiles = map[string][]string{
	"claude":  {"commands/taskwing.md"},
	"codex":   {"commands/taskwing.md"},
	"gemini":  {"commands/taskwing.toml"},
	"cursor":  {"rules/taskwing.md", "mcp.json"}, // Either slash command or MCP config
	"copilot": {"copilot-instructions.md"},       // Main instructions file
}

// hasValidAIConfig checks if the AI config directory has valid TaskWing config files.
// Returns true if at least one expected config file exists.
func hasValidAIConfig(basePath string, ai string) bool {
	dir, ok := aiConfigDirs[ai]
	if !ok {
		return false
	}

	files, ok := aiConfigFiles[ai]
	if !ok {
		// No specific files defined, fall back to directory existence
		configPath := basePath + "/" + dir
		_, err := os.Stat(configPath)
		return err == nil
	}

	// Check if ANY of the expected files exist
	for _, file := range files {
		filePath := basePath + "/" + dir + "/" + file
		if _, err := os.Stat(filePath); err == nil {
			return true
		}
	}
	return false
}

// findMissingAIConfigs checks which AI assistants are missing valid local config.
// It compares against the provided list of AIs (typically from global MCP detection).
// Returns a list of AI names that have global MCP but missing/invalid local configs.
func findMissingAIConfigs(basePath string, aiList []string) []string {
	var missing []string
	for _, ai := range aiList {
		if !hasValidAIConfig(basePath, ai) {
			missing = append(missing, ai)
		}
	}
	return missing
}

// findExistingAIConfigs checks which AI configs have valid TaskWing configuration locally.
// Returns a list of AI names that have valid local config files present.
func findExistingAIConfigs(basePath string) []string {
	var existing []string
	for ai := range aiConfigDirs {
		if hasValidAIConfig(basePath, ai) {
			existing = append(existing, ai)
		}
	}
	return existing
}
