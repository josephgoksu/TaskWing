/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
 _____         _   __        ___
|_   _|_ _ ___| | _\ \      / (_)_ __   __ _
  | |/ _' / __| |/ /\ \ /\ / /| | '_ \ / _' |
  | | (_| \__ \   <  \ V  V / | | | | | (_| |
  |_|\__,_|___/_|\_\  \_/\_/  |_|_| |_|\__, |
                                       |___/

TaskWing captures the decisions, context, and rationale behind your codebase—
making it queryable by humans and AI.

Key Commands:
  taskwing bootstrap      Auto-generate knowledge from your repo
  taskwing add "text"     Add knowledge (AI classifies it)
  taskwing list           View all knowledge by type
  taskwing context "q"    Semantic search
  taskwing mcp            Start MCP server for AI integration`,
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

	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	_ = viper.BindPFlag("preview", rootCmd.PersistentFlags().Lookup("preview"))
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
