/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up TaskWing for your project (zero API keys needed)",
	Long: `Initialize TaskWing for your project in one command.

What it does:
  1. Detects your AI CLI (Claude Code, Gemini CLI, Codex CLI)
  2. Runs deterministic code indexing (no LLM required)
  3. Registers TaskWing MCP server with your AI CLI
  4. Installs session hooks and slash commands

No API keys needed. Works with whatever AI CLI you already have.

Examples:
  taskwing init                  # Auto-detect AI CLI
  taskwing init --claude-code    # Register with Claude Code
  taskwing init --gemini         # Register with Gemini CLI
  taskwing init --codex          # Register with Codex CLI`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().Bool("claude-code", false, "Register with Claude Code")
	initCmd.Flags().Bool("gemini", false, "Register with Gemini CLI")
	initCmd.Flags().Bool("codex", false, "Register with Codex CLI")
}

func runInit(cmd *cobra.Command, _ []string) error {
	verbose := viper.GetBool("verbose")

	// Determine target AI CLI
	target := detectTarget(cmd)
	if target == "" {
		return fmt.Errorf("no AI CLI detected. Install Claude Code, Gemini CLI, or Codex CLI, or use --claude-code/--gemini/--codex")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	if !isQuiet() {
		fmt.Println()
		fmt.Printf("%s TaskWing Init → %s\n", ui.IconRocket, target)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━")
	}

	// Step 1: Initialize project structure + deterministic indexing (no LLM)
	if !isQuiet() {
		fmt.Printf("%s Initializing project and indexing codebase...\n", ui.IconPackage)
	}
	initializer := bootstrap.NewInitializer(cwd)
	if err := initializer.Run(verbose, []string{targetToAIName(target)}); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: init structure: %v\n", err)
		}
	}
	svc := bootstrap.NewService(cwd, llm.Config{})
	if _, err := svc.RunDeterministicBootstrap(cmd.Context(), isQuiet()); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: indexing: %v\n", err)
		}
	}

	// Step 2: Register MCP server
	if !isQuiet() {
		fmt.Printf("%s Registering MCP server with %s...\n", ui.IconPlug, target)
	}
	if err := registerMCP(target, verbose); err != nil {
		ui.PrintWarning(fmt.Sprintf("MCP registration: %v", err))
	}

	// Step 3: Install hooks and slash commands
	if !isQuiet() {
		fmt.Printf("%s Installing hooks and slash commands...\n", ui.IconWrench)
	}
	aiName := targetToAIName(target)
	if err := initializer.InstallHooksConfig(aiName, verbose); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: hooks install: %v\n", err)
		}
	}
	if err := initializer.CreateSlashCommands(aiName, verbose); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: slash commands: %v\n", err)
		}
	}

	if !isQuiet() {
		fmt.Println()
		ui.PrintSuccess("TaskWing ready. No API keys needed.")
		fmt.Printf("   MCP tools available in %s.\n", target)
		fmt.Println()
	}

	return nil
}

func detectTarget(cmd *cobra.Command) string {
	if b, _ := cmd.Flags().GetBool("claude-code"); b {
		return "Claude Code"
	}
	if b, _ := cmd.Flags().GetBool("gemini"); b {
		return "Gemini CLI"
	}
	if b, _ := cmd.Flags().GetBool("codex"); b {
		return "Codex CLI"
	}

	// Auto-detect
	for _, bin := range []struct {
		name   string
		target string
	}{
		{"claude", "Claude Code"},
		{"gemini", "Gemini CLI"},
		{"codex", "Codex CLI"},
	} {
		if _, err := exec.LookPath(bin.name); err == nil {
			return bin.target
		}
	}
	return ""
}

func targetToAIName(target string) string {
	switch target {
	case "Claude Code":
		return "claude"
	case "Gemini CLI":
		return "gemini"
	case "Codex CLI":
		return "codex"
	}
	return strings.ToLower(strings.Fields(target)[0])
}

func registerMCP(target string, verbose bool) error {
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("determine binary path: %w", err)
	}

	switch target {
	case "Claude Code":
		out, err := exec.Command("claude", "mcp", "add", "taskwing", "--", binPath, "mcp").CombinedOutput()
		if err != nil {
			// Try scope flag
			out, err = exec.Command("claude", "mcp", "add", "--scope", "project", "taskwing", "--", binPath, "mcp").CombinedOutput()
			if err != nil {
				return fmt.Errorf("claude mcp add: %s: %w", strings.TrimSpace(string(out)), err)
			}
		}
		if verbose {
			fmt.Printf("   %s\n", strings.TrimSpace(string(out)))
		}
	case "Gemini CLI":
		out, err := exec.Command("gemini", "mcp", "add", "taskwing", "--", binPath, "mcp").CombinedOutput()
		if err != nil {
			return fmt.Errorf("gemini mcp add: %s: %w", strings.TrimSpace(string(out)), err)
		}
		if verbose {
			fmt.Printf("   %s\n", strings.TrimSpace(string(out)))
		}
	case "Codex CLI":
		out, err := exec.Command("codex", "mcp", "add", "taskwing", "--", binPath, "mcp").CombinedOutput()
		if err != nil {
			return fmt.Errorf("codex mcp add: %s: %w", strings.TrimSpace(string(out)), err)
		}
		if verbose {
			fmt.Printf("   %s\n", strings.TrimSpace(string(out)))
		}
	default:
		return fmt.Errorf("unsupported target: %s", target)
	}
	return nil
}
