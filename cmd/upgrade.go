/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// upgradeCmd represents the upgrade command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade slash commands and MCP config to latest version",
	Long: `Regenerate slash commands for all detected AI assistants.

Run this after upgrading TaskWing to get the latest slash commands.

Detects and upgrades:
  - .claude/commands/    (Claude Code)
  - .codex/commands/     (OpenAI Codex)
  - .gemini/commands/    (Gemini CLI)
  - .cursor/rules/       (Cursor)
  - .github/copilot-instructions/ (GitHub Copilot)

Example:
  taskwing upgrade           # Upgrade all detected assistants`,
	RunE: runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	verbose := true
	upgraded := false

	fmt.Println("🔄 TaskWing Upgrade")
	fmt.Printf("   Version: %s\n", GetVersion())
	fmt.Println()

	// Detect and upgrade each AI assistant
	assistants := []struct {
		name       string
		configDir  string
		aiCfg      aiConfig
	}{
		{"Claude Code", ".claude", aiConfigs["claude"]},
		{"Codex", ".codex", aiConfigs["codex"]},
		{"Gemini CLI", ".gemini", aiConfigs["gemini"]},
		{"Cursor", ".cursor", aiConfigs["cursor"]},
		{"GitHub Copilot", ".github", aiConfigs["copilot"]},
	}

	for _, a := range assistants {
		dirPath := filepath.Join(cwd, a.configDir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			continue
		}

		fmt.Printf("📝 Upgrading %s commands...\n", a.name)

		// Regenerate slash commands
		if err := createSingleSlashCommand(cwd, a.aiCfg, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  Failed to upgrade %s: %v\n", a.name, err)
			continue
		}

		if err := createTaskSlashCommands(cwd, a.aiCfg, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  Failed to upgrade task commands for %s: %v\n", a.name, err)
			continue
		}

		upgraded = true
	}

	if !upgraded {
		fmt.Println("No AI assistant configurations found.")
		fmt.Println("Run 'taskwing bootstrap' first to set up your project.")
		return nil
	}

	fmt.Println()
	fmt.Println("✅ Upgrade complete!")
	fmt.Println()
	fmt.Println("Updated slash commands:")
	fmt.Println("  /taskwing    - Fetch project context")
	fmt.Println("  /tw-next     - Start next task")
	fmt.Println("  /tw-done     - Complete current task")
	fmt.Println("  /tw-status   - Show task status")
	fmt.Println("  /tw-block    - Mark task as blocked")
	fmt.Println("  /tw-context  - Fetch additional context")
	fmt.Println("  /tw-plan     - Create development plan")

	return nil
}
