/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/telemetry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// version is the application version.
	version = "1.4.2"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "taskwing",
	Short: "TaskWing - AI-Native Task Management",
	Long: `TaskWing - AI-Native Task Management

Generate context-aware development tasks that actually match your architecture.
No more generic AI suggestions that ignore your patterns, constraints, and decisions.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip telemetry for completion and help commands
		if cmd.Name() == "completion" || cmd.Name() == "help" || cmd.Name() == "__complete" || cmd.Name() == "mcp" {
			return nil
		}
		// Skip telemetry init for telemetry config commands (to avoid recursion)
		if cmd.Parent() != nil && cmd.Parent().Name() == "telemetry" {
			return nil
		}

		// Check if telemetry is disabled via flag or config
		disabled := viper.GetBool("telemetry.disabled")
		if disabled {
			return telemetry.Init(version, true)
		}

		// Check if we need consent and prompt if necessary
		_, err := telemetry.CheckAndPromptConsent(version)
		if err != nil {
			// Don't fail the command if telemetry setup fails
			// Just log if verbose
			if viper.GetBool("verbose") {
				fmt.Fprintf(os.Stderr, "Warning: telemetry setup failed: %v\n", err)
			}
		}

		// Initialize telemetry client
		return telemetry.Init(version, false)
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// Flush any pending telemetry events
		telemetry.Shutdown()
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Enable Cobra's built-in suggestions
	rootCmd.SuggestionsMinimumDistance = 2

	err := rootCmd.Execute()
	if err != nil {
		// Check if it's an unknown command error and provide helpful hints
		errStr := err.Error()
		if strings.Contains(errStr, "unknown command") {
			// Extract the unknown command
			parts := strings.Split(errStr, "\"")
			if len(parts) >= 2 {
				unknownCmd := parts[1]
				suggestion := getCommandHint(unknownCmd)
				if suggestion != "" {
					fmt.Fprintf(os.Stderr, "\n%s\n", suggestion)
				}
			}
		}
		os.Exit(1)
	}
}

// getCommandHint returns a helpful hint for common command mistakes
func getCommandHint(cmd string) string {
	hints := map[string]string{
		"export":  "Hint: To export a plan, use: tw plan export <plan-id>",
		"search":  "Hint: To search knowledge, use: tw context \"<query>\"",
		"query":   "Hint: To query knowledge, use: tw context \"<query>\"",
		"find":    "Hint: To find knowledge, use: tw context \"<query>\"",
		"plans":   "Hint: To list plans, use: tw plan list",
		"tasks":   "Hint: To list tasks, use: tw task list",
		"nodes":   "Hint: To list knowledge nodes, use: tw list",
		"status":  "Hint: To check project status, use: tw memory check",
		"check":   "Hint: To check memory integrity, use: tw memory check",
		"doctor":  "Hint: To diagnose issues, use: tw memory check",
		"create":  "Hint: To create a plan, use: tw plan new \"<goal>\"",
		"new":     "Hint: To create a plan, use: tw plan new \"<goal>\"",
		"embed":   "Hint: To generate embeddings, use: tw memory generate-embeddings",
		"reset":   "Hint: To reset memory, use: tw memory reset",
		"install": "Hint: To install MCP, use: tw mcp install",
	}

	if hint, ok := hints[cmd]; ok {
		return hint
	}
	return ""
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().Bool("no-telemetry", false, "Disable anonymous telemetry")
	rootCmd.PersistentFlags().Bool("json", false, "Output as JSON")
	rootCmd.PersistentFlags().Bool("quiet", false, "Minimal output")
	rootCmd.PersistentFlags().Bool("preview", false, "Dry run (no changes)")

	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("telemetry.disabled", rootCmd.PersistentFlags().Lookup("no-telemetry"))
	_ = viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	_ = viper.BindPFlag("preview", rootCmd.PersistentFlags().Lookup("preview"))

	// Custom Help Template
	rootCmd.SetHelpTemplate(`{{if .Long}}
{{.Long}}
{{else}}
  {{.Short}}
{{end}}
  Usage: {{.UseLine}}
{{if .HasAvailableSubCommands}}
  Commands:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}    {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
  Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

  Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}
`)
}

// GetVersion returns the application version
func GetVersion() string {
	return version
}
