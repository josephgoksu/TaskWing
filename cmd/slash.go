/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/spf13/cobra"
)

var slashContents = map[string]string{
	"next":     slashNextContent,
	"done":     slashDoneContent,
	"status":   slashStatusContent,
	"plan":     slashPlanContent,
	"ask":      slashAskContent,
	"remember": slashRememberContent,
	"simplify": slashSimplifyContent,
	"debug":    slashDebugContent,
	"explain":  slashExplainContent,
}

var slashCmd = &cobra.Command{
	Use:          "slash",
	Short:        "Output slash command content for AI assistants",
	SilenceUsage: true,
	Long: `Outputs the full prompt content for slash commands.

This command is called dynamically by AI assistant slash commands
to ensure the content always matches the installed CLI version.

Example:
  taskwing slash next     # Output /tw-next content
  taskwing slash done     # Output /tw-done content
  taskwing slash plan     # Output /tw-plan content`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return fmt.Errorf("unknown slash command %q (available: %s)", args[0], strings.Join(availableSlashCommands(cmd), ", "))
	},
}

func availableSlashCommands(cmd *cobra.Command) []string {
	available := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() || sub.Name() == "help" {
			continue
		}
		available = append(available, sub.Name())
	}
	sort.Strings(available)
	return available
}

func init() {
	rootCmd.AddCommand(slashCmd)

	for _, slash := range bootstrap.SlashCommands {
		content, ok := slashContents[slash.SlashCmd]
		if !ok {
			continue
		}

		short := fmt.Sprintf("Output /%s command content", slash.BaseName)
		c := &cobra.Command{
			Use:   slash.SlashCmd,
			Short: short,
			Run: func(content string) func(*cobra.Command, []string) {
				return func(cmd *cobra.Command, args []string) {
					fmt.Print(content)
				}
			}(content),
		}

		slashCmd.AddCommand(c)
	}
}
