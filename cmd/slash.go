/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/skills"
	"github.com/spf13/cobra"
)

var slashCmd = &cobra.Command{
	Use:          "slash",
	Short:        "Output slash command content for AI assistants",
	SilenceUsage: true,
	Long: `Outputs the full prompt content for slash commands.

This command is called dynamically by AI assistant slash commands
to ensure the content always matches the installed CLI version.

Example:
  taskwing slash next     # Output /taskwing:next content
  taskwing slash done     # Output /taskwing:done content
  taskwing slash plan     # Output /taskwing:plan content`,
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
		name := slash.SlashCmd
		short := fmt.Sprintf("Output /%s command content", slash.BaseName)
		c := &cobra.Command{
			Use:   name,
			Short: short,
			RunE: func(cmd *cobra.Command, args []string) error {
				body, err := skills.GetBody(name)
				if err != nil {
					return fmt.Errorf("load skill content: %w", err)
				}
				fmt.Print(body)
				return nil
			},
		}

		slashCmd.AddCommand(c)
	}
}
