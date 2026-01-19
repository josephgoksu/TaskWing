/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/logger"
	"github.com/josephgoksu/TaskWing/internal/telemetry"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// version is the application version.
	// Set via ldflags at build time: -ldflags "-X github.com/josephgoksu/TaskWing/cmd.version=1.0.0"
	// Defaults to "dev" for local development builds.
	version = "dev"

	// postHogAPIKey is the PostHog project API key.
	// Set via ldflags at build time: -ldflags "-X github.com/josephgoksu/TaskWing/cmd.postHogAPIKey=phc_xxx"
	// Falls back to embedded default for development builds.
	postHogAPIKey = "phc_itYzXvsW73J37ivErHAdSbNhyF1GjGar5lKl5UITEoz"

	// postHogEndpoint is the PostHog API endpoint.
	// Set via ldflags at build time: -ldflags "-X github.com/josephgoksu/TaskWing/cmd.postHogEndpoint=https://..."
	postHogEndpoint = "https://us.i.posthog.com"

	// telemetryClient is the global telemetry client instance.
	// Initialized in PersistentPreRunE, closed in Execute().
	telemetryClient telemetry.Client

	// commandStartTime tracks when the current command started.
	// Used to calculate command duration for telemetry.
	commandStartTime time.Time

	// executedCmd and executedArgs store the command being executed.
	// Used for telemetry tracking in Execute() after command completes.
	executedCmd  *cobra.Command
	executedArgs []string
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

	// Track command execution (success or failure) and close telemetry
	trackAndCloseTelemetry(err)

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
// 4. First-run consent prompt (if needed)
// 5. User's telemetry config preference
func initTelemetry(cmd *cobra.Command, args []string) error {
	// Store cmd and args for tracking in Execute() (do this first)
	executedCmd = cmd
	executedArgs = args
	commandStartTime = time.Now()

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

	// First-run consent prompt (only in interactive terminals)
	if cfg.NeedsConsent() && ui.IsInteractive() {
		enabled := promptTelemetryConsent()
		if enabled {
			cfg.Enable()
		} else {
			cfg.Disable()
		}
		// Save the user's choice immediately
		if err := cfg.Save(); err != nil {
			// Log but don't fail - we'll just ask again next time
			if viper.GetBool("verbose") {
				fmt.Fprintf(os.Stderr, "Warning: could not save telemetry preference: %v\n", err)
			}
		}
	}

	// If telemetry is disabled in config (or user just declined), use noop client
	if !cfg.IsEnabled() {
		telemetryClient = telemetry.NewNoopClient()
		return nil
	}

	// Initialize the PostHog client (credentials set via ldflags or defaults)
	client, err := telemetry.NewPostHogClient(telemetry.ClientConfig{
		APIKey:   postHogAPIKey,
		Endpoint: postHogEndpoint,
		Version:  version,
		Config:   cfg,
	})
	if err != nil {
		// Fail gracefully - telemetry errors should never break the CLI
		telemetryClient = telemetry.NewNoopClient()
		return nil
	}

	telemetryClient = client
	return nil
}

// promptTelemetryConsent displays a consent prompt and returns true if user accepts.
// Uses [y/N] format where Enter defaults to No (opt-in consent per GDPR).
func promptTelemetryConsent() bool {
	fmt.Println()
	fmt.Println("  TaskWing can collect anonymous usage statistics to improve the product.")
	fmt.Println("  This includes: command names, success/failure, duration, OS, and CLI version.")
	fmt.Println("  No code, file paths, or personal data is collected.")
	fmt.Println()
	fmt.Print("  Enable anonymous telemetry? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		// On error (e.g., piped input), default to disabled for safety
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))

	// Only enable if user explicitly says yes (opt-in)
	if response == "y" || response == "yes" {
		fmt.Println("  Telemetry enabled. Thank you for helping improve TaskWing!")
		fmt.Println()
		return true
	}

	fmt.Println("  Telemetry disabled. You can enable it later with: taskwing config telemetry enable")
	fmt.Println()
	return false
}

// closeTelemetry is kept for Cobra hook compatibility but does nothing.
// Actual tracking and close happens in Execute() to capture both success and failure.
func closeTelemetry(cmd *cobra.Command, args []string) error {
	// Tracking moved to Execute() - see trackAndCloseTelemetry()
	return nil
}

// trackAndCloseTelemetry tracks command execution and closes the telemetry client.
// Called from Execute() to capture both successful and failed commands.
func trackAndCloseTelemetry(cmdErr error) {
	if telemetryClient == nil {
		return
	}

	// Track the command execution
	success := cmdErr == nil
	errorType := classifyError(cmdErr)
	trackCommandExecutionWithError(executedCmd, executedArgs, success, errorType)

	// Flush pending events and close
	// Ignore errors - telemetry should never affect CLI exit status
	_ = telemetryClient.Close()
}

// classifyError returns a category for the error type (not the message, for privacy).
// Returns empty string if no error.
func classifyError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// Classify by error pattern (never include actual error message)
	switch {
	case strings.Contains(errStr, "unknown command"):
		return "unknown_command"
	case strings.Contains(errStr, "unknown flag"):
		return "unknown_flag"
	case strings.Contains(errStr, "required flag"):
		return "missing_required_flag"
	case strings.Contains(errStr, "invalid argument"):
		return "invalid_argument"
	case strings.Contains(errStr, "not found"):
		return "not_found"
	case strings.Contains(errStr, "permission denied"):
		return "permission_denied"
	case strings.Contains(errStr, "timeout"):
		return "timeout"
	case strings.Contains(errStr, "connection"):
		return "connection_error"
	case strings.Contains(errStr, "API"):
		return "api_error"
	default:
		return "other"
	}
}

// trackCommandExecutionWithError tracks a command execution event with error type.
func trackCommandExecutionWithError(cmd *cobra.Command, args []string, success bool, errorType string) {
	if telemetryClient == nil || cmd == nil {
		return
	}

	// Calculate duration in milliseconds
	durationMs := time.Since(commandStartTime).Milliseconds()

	// Get full command path (e.g., "plan new", "task list")
	commandPath := getCommandPath(cmd)

	// Build properties
	props := telemetry.Properties{
		"command":     commandPath,
		"duration_ms": durationMs,
		"success":     success,
		"args_count":  len(args),
	}

	// Add error type if present (never the actual error message)
	if errorType != "" {
		props["error_type"] = errorType
	}

	// Track the event
	telemetryClient.Track("command_executed", props)
}

// getCommandPath returns the full command path (e.g., "plan new", "task list").
func getCommandPath(cmd *cobra.Command) string {
	if cmd == nil {
		return "unknown"
	}

	// Build path from command to root
	var parts []string
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() != "" && c.Name() != "taskwing" {
			parts = append([]string{c.Name()}, parts...)
		}
	}

	if len(parts) == 0 {
		return "root"
	}
	return strings.Join(parts, " ")
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
