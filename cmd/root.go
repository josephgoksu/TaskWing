/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/josephgoksu/TaskWing/internal/telemetry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// version is the application version.
	version = "2.0.0"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "taskwing",
	Short: "TaskWing - Knowledge Graph for Engineering Teams",
	Long: `
████████╗ █████╗ ███████╗██╗  ██╗██╗    ██╗██╗███╗   ██╗ ██████╗
╚══██╔══╝██╔══██╗██╔════╝██║ ██╔╝██║    ██║██║████╗  ██║██╔════╝
   ██║   ███████║███████╗█████╔╝ ██║ █╗ ██║██║██╔██╗ ██║██║  ███╗
   ██║   ██╔══██║╚════██║██╔═██╗ ██║███╗██║██║██║╚██╗██║██║   ██║
   ██║   ██║  ██║███████║██║  ██╗╚███╔███╔╝██║██║ ╚████║╚██████╔╝
   ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝ ╚═╝╚═╝  ╚═══╝ ╚═════╝

TaskWing captures the decisions, context, and rationale behind your codebase—
making it queryable by humans and AI.

Key Commands:
  taskwing bootstrap      Auto-generate knowledge from your repo
  taskwing add "text"     Add knowledge (AI classifies it)
  taskwing list           View all knowledge by type
  taskwing context "q"    Semantic search
  taskwing mcp            Start MCP server for AI integration
  taskwing mcp install    Setup MCP for Cursor/Claude/Gemini`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip telemetry for completion and help commands
		if cmd.Name() == "completion" || cmd.Name() == "help" || cmd.Name() == "__complete" {
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
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().Bool("json", false, "Output as JSON")
	rootCmd.PersistentFlags().Bool("quiet", false, "Minimal output")
	rootCmd.PersistentFlags().Bool("preview", false, "Dry run (no changes)")
	rootCmd.PersistentFlags().Bool("no-telemetry", false, "Disable anonymous telemetry")

	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	_ = viper.BindPFlag("preview", rootCmd.PersistentFlags().Lookup("preview"))
	_ = viper.BindPFlag("telemetry.disabled", rootCmd.PersistentFlags().Lookup("no-telemetry"))
}

// GetVersion returns the application version
func GetVersion() string {
	return version
}

// GetMemoryBasePath returns the path to the .taskwing/memory directory
func GetMemoryBasePath() string {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	basePath := viper.GetString("memory.path")
	if basePath == "" {
		basePath = ".taskwing/memory"
	}
	if filepath.IsAbs(basePath) {
		return basePath
	}
	return filepath.Join(cwd, basePath)
}
