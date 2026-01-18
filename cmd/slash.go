/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// slashCmd is the parent command for dynamic slash command content
var slashCmd = &cobra.Command{
	Use:   "slash",
	Short: "Output slash command content for AI assistants",
	Long: `Outputs the full prompt content for slash commands.

This command is called dynamically by AI assistant slash commands
to ensure the content always matches the installed CLI version.

Example:
  taskwing slash next     # Output /tw-next content
  taskwing slash done     # Output /tw-done content
  taskwing slash plan     # Output /tw-plan content`,
}

func init() {
	rootCmd.AddCommand(slashCmd)
	slashCmd.AddCommand(slashNextCmd)
	slashCmd.AddCommand(slashDoneCmd)
	slashCmd.AddCommand(slashStatusCmd)
	slashCmd.AddCommand(slashPlanCmd)
	slashCmd.AddCommand(slashTaskwingCmd)
	slashCmd.AddCommand(slashSimplifyCmd)
	slashCmd.AddCommand(slashDebugCmd)
	slashCmd.AddCommand(slashExplainCmd)
}

// slashNextCmd outputs the /tw-next prompt content
var slashNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Output /tw-next command content",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(slashNextContent)
	},
}

// slashDoneCmd outputs the /tw-done prompt content
var slashDoneCmd = &cobra.Command{
	Use:   "done",
	Short: "Output /tw-done command content",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(slashDoneContent)
	},
}

// slashStatusCmd outputs the /tw-status prompt content
var slashStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Output /tw-status command content",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(slashStatusContent)
	},
}

// slashPlanCmd outputs the /tw-plan prompt content
var slashPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Output /tw-plan command content",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(slashPlanContent)
	},
}

// slashTaskwingCmd outputs the /taskwing prompt content
var slashTaskwingCmd = &cobra.Command{
	Use:   "taskwing",
	Short: "Output /taskwing command content",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(slashTaskwingContent)
	},
}

// slashSimplifyCmd outputs the /tw-simplify prompt content
var slashSimplifyCmd = &cobra.Command{
	Use:   "simplify",
	Short: "Output /tw-simplify command content",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(slashSimplifyContent)
	},
}

// slashDebugCmd outputs the /tw-debug prompt content
var slashDebugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Output /tw-debug command content",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(slashDebugContent)
	},
}

// slashExplainCmd outputs the /tw-explain prompt content
var slashExplainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Output /tw-explain command content",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(slashExplainContent)
	},
}
