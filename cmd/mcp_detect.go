/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"os/exec"
	"time"

	"github.com/josephgoksu/TaskWing/internal/mcpcfg"
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
	return mcpcfg.ContainsCanonicalServerName(string(output))
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
	return mcpcfg.ContainsCanonicalServerName(string(output))
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
	return mcpcfg.ContainsCanonicalServerName(string(output))
}
