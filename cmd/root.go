/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/logger"
	"github.com/josephgoksu/TaskWing/internal/telemetry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// version is the application version.
	version = "1.12.3"

	// telemetryClient is the global telemetry client instance.
	// Initialized in PersistentPreRunE, closed in PersistentPostRunE.
	telemetryClient telemetry.Client
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "taskwing",
	Short: "TaskWing - AI-Native Task Management",
	Long: `TaskWing - AI-Native Task Management

Generate context-aware development tasks that actually match your architecture.
No more generic AI suggestions that ignore your patterns, constraints, and decisions.`,
	PersistentPreRunE:  initTelemetry,
	PersistentPostRunE: closeTelemetry,
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
	// Set up crash handler
	initCrashHandler()
	defer logger.HandlePanic()

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

// initCrashHandler sets up the crash logging context.
func initCrashHandler() {
	// Set version
	logger.SetVersion(version)

	// Set base path for crash logs
	basePath, err := config.GetMemoryBasePath()
	if err == nil {
		// Use the parent of memory path (i.e., .taskwing)
		logger.SetBasePath(strings.TrimSuffix(basePath, "/memory"))
	}

	// Set command name (will be updated by each subcommand if needed)
	if len(os.Args) > 1 {
		logger.SetCommand(strings.Join(os.Args[1:], " "))
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
	rootCmd.PersistentFlags().Bool("json", false, "Output as JSON")
	rootCmd.PersistentFlags().Bool("quiet", false, "Minimal output")
	rootCmd.PersistentFlags().Bool("preview", false, "Dry run (no changes)")
	rootCmd.PersistentFlags().Bool("no-telemetry", false, "Disable telemetry for this command")

	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	_ = viper.BindPFlag("preview", rootCmd.PersistentFlags().Lookup("preview"))
	_ = viper.BindPFlag("no-telemetry", rootCmd.PersistentFlags().Lookup("no-telemetry"))

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

// initTelemetry initializes the telemetry client.
// It checks for:
// 1. --no-telemetry flag (disables for this command)
// 2. CI environment variable (auto-disables in CI)
// 3. Non-interactive terminal (auto-disables if not a TTY)
// 4. User's telemetry config preference
func initTelemetry(cmd *cobra.Command, args []string) error {
	// Check if telemetry is explicitly disabled via flag
	if viper.GetBool("no-telemetry") {
		telemetryClient = telemetry.NewNoopClient()
		return nil
	}

	// Check for CI environment - auto-disable in CI
	if isCI() {
		telemetryClient = telemetry.NewNoopClient()
		return nil
	}

	// Load telemetry config
	cfg, err := telemetry.Load()
	if err != nil {
		// If we can't load config, fail gracefully with noop client
		telemetryClient = telemetry.NewNoopClient()
		return nil
	}

	// If telemetry is disabled in config, use noop client
	if !cfg.IsEnabled() {
		telemetryClient = telemetry.NewNoopClient()
		return nil
	}

	// Get PostHog API key from environment
	apiKey := os.Getenv("TASKWING_POSTHOG_KEY")
	if apiKey == "" {
		// No API key configured - use noop client
		telemetryClient = telemetry.NewNoopClient()
		return nil
	}

	// Initialize the PostHog client
	client, err := telemetry.NewPostHogClient(telemetry.ClientConfig{
		APIKey:  apiKey,
		Version: version,
		Config:  cfg,
	})
	if err != nil {
		// Fail gracefully - telemetry errors should never break the CLI
		telemetryClient = telemetry.NewNoopClient()
		return nil
	}

	telemetryClient = client
	return nil
}

// closeTelemetry flushes and closes the telemetry client.
func closeTelemetry(cmd *cobra.Command, args []string) error {
	if telemetryClient != nil {
		// Ignore errors - telemetry should never affect CLI exit status
		_ = telemetryClient.Close()
	}
	return nil
}

// isCI returns true if running in a CI environment.
// Checks common CI environment variables.
func isCI() bool {
	// Common CI environment variables
	ciEnvVars := []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"TRAVIS",
		"JENKINS_URL",
		"BUILDKITE",
		"DRONE",
		"TEAMCITY_VERSION",
	}

	for _, envVar := range ciEnvVars {
		if os.Getenv(envVar) != "" {
			return true
		}
	}
	return false
}

// GetTelemetryClient returns the global telemetry client.
// This allows subcommands to track events.
func GetTelemetryClient() telemetry.Client {
	if telemetryClient == nil {
		return telemetry.NewNoopClient()
	}
	return telemetryClient
}
