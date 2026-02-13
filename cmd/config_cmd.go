/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/telemetry"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configShowCmd shows current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigShow()
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigSet(args[0], args[1])
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigGet(args[0])
	},
}

// Telemetry subcommands
var configTelemetryCmd = &cobra.Command{
	Use:   "telemetry",
	Short: "Manage telemetry settings",
	Long: `View and manage anonymous usage telemetry settings.

TaskWing collects anonymous usage data to improve the product:
  - Command names and execution duration
  - Success/failure status
  - OS and architecture
  - CLI version

No code, file paths, or personal data is ever collected.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTelemetryStatus()
	},
}

var configTelemetryStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current telemetry status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTelemetryStatus()
	},
}

var configTelemetryEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable anonymous telemetry",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTelemetryEnable()
	},
}

var configTelemetryDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable anonymous telemetry",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTelemetryDisable()
	},
}

// configCmd is the parent config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage TaskWing configuration",
	Long: `View and manage TaskWing configuration settings.

Running without a subcommand launches an interactive configuration menu
where you can view and edit all model settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigInteractive()
	},
}

func init() {
	// Add config command to root
	rootCmd.AddCommand(configCmd)

	// Add subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)

	// Add telemetry subcommand with its subcommands
	configCmd.AddCommand(configTelemetryCmd)
	configTelemetryCmd.AddCommand(configTelemetryStatusCmd)
	configTelemetryCmd.AddCommand(configTelemetryEnableCmd)
	configTelemetryCmd.AddCommand(configTelemetryDisableCmd)
}

// HooksSettings represents the hooks configuration in a user-friendly format
type HooksSettings struct {
	Enabled    bool `json:"enabled"`
	MaxTasks   int  `json:"max_tasks"`
	MaxMinutes int  `json:"max_minutes"`
}

func runConfigShow() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	if isJSON() {
		type configFiles struct {
			Claude string `json:"claude,omitempty"`
			Codex  string `json:"codex,omitempty"`
		}
		type configStatus struct {
			Hooks       HooksSettings `json:"hooks"`
			ConfigFiles configFiles   `json:"config_files"`
		}

		settings := loadHooksSettings(cwd)
		files := configFiles{}
		claudePath := filepath.Join(cwd, ".claude", "settings.json")
		codexPath := filepath.Join(cwd, ".codex", "settings.json")
		if _, err := os.Stat(claudePath); err == nil {
			files.Claude = claudePath
		}
		if _, err := os.Stat(codexPath); err == nil {
			files.Codex = codexPath
		}

		return printJSON(configStatus{
			Hooks:       settings,
			ConfigFiles: files,
		})
	}

	fmt.Println("TaskWing Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Show hooks configuration
	fmt.Println("## Hooks")
	settings := loadHooksSettings(cwd)
	fmt.Printf("  enabled:     %v\n", settings.Enabled)
	fmt.Printf("  max-tasks:   %d\n", settings.MaxTasks)
	fmt.Printf("  max-minutes: %d\n", settings.MaxMinutes)

	// Show config files
	fmt.Println()
	fmt.Println("## Config Files")
	claudePath := filepath.Join(cwd, ".claude", "settings.json")
	codexPath := filepath.Join(cwd, ".codex", "settings.json")

	if _, err := os.Stat(claudePath); err == nil {
		fmt.Printf("  Claude: %s\n", claudePath)
	}
	if _, err := os.Stat(codexPath); err == nil {
		fmt.Printf("  Codex:  %s\n", codexPath)
	}

	return nil
}

func runConfigSet(key, value string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	switch key {
	case "hooks.enabled":
		enabled := value == "true" || value == "1" || value == "yes"
		return setHooksEnabled(cwd, enabled)

	case "hooks.max-tasks":
		maxTasks, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for max-tasks: %s", value)
		}
		return setHooksMaxTasks(cwd, maxTasks)

	case "hooks.max-minutes":
		maxMinutes, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for max-minutes: %s", value)
		}
		return setHooksMaxMinutes(cwd, maxMinutes)

	case "llm.provider", "llm.model":
		fmt.Println("Use 'taskwing config' (interactive menu) to configure LLM settings.")
		fmt.Println("Or set environment variables:")
		fmt.Println("  TASKWING_LLM_PROVIDER=ollama")
		fmt.Println("  TASKWING_LLM_MODEL=llama3.2")
		fmt.Println("  TASKWING_LLM_PROVIDER=bedrock")
		fmt.Println("  TASKWING_LLM_MODEL=us.anthropic.claude-sonnet-4-5-20250929-v1:0")
		fmt.Println("  TASKWING_LLM_BEDROCK_REGION=us-east-1")
		fmt.Println("Or edit ~/.taskwing/config.yaml directly.")
		return nil

	default:
		return fmt.Errorf("unknown config key: %s\n\nAvailable keys:\n  hooks.enabled\n  hooks.max-tasks\n  hooks.max-minutes\n\nFor LLM config, run: taskwing config", key)
	}
}

func runConfigGet(key string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	settings := loadHooksSettings(cwd)

	switch key {
	case "hooks.enabled":
		if isJSON() {
			return printJSON(map[string]any{"key": key, "value": settings.Enabled})
		}
		fmt.Println(settings.Enabled)
	case "hooks.max-tasks":
		if isJSON() {
			return printJSON(map[string]any{"key": key, "value": settings.MaxTasks})
		}
		fmt.Println(settings.MaxTasks)
	case "hooks.max-minutes":
		if isJSON() {
			return printJSON(map[string]any{"key": key, "value": settings.MaxMinutes})
		}
		fmt.Println(settings.MaxMinutes)
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return nil
}

func loadHooksSettings(cwd string) HooksSettings {
	settings := HooksSettings{
		Enabled:    false,
		MaxTasks:   DefaultMaxTasksPerSession,
		MaxMinutes: DefaultMaxSessionMinutes,
	}

	// Try Claude settings first
	claudePath := filepath.Join(cwd, ".claude", "settings.json")
	if config, err := loadSettingsFile(claudePath); err == nil {
		parseHooksConfig(config, &settings)
		return settings
	}

	// Try Codex settings
	codexPath := filepath.Join(cwd, ".codex", "settings.json")
	if config, err := loadSettingsFile(codexPath); err == nil {
		parseHooksConfig(config, &settings)
		return settings
	}

	return settings
}

func loadSettingsFile(path string) (map[string]any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config map[string]any
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	return config, nil
}

func parseHooksConfig(config map[string]any, settings *HooksSettings) {
	hooks, ok := config["hooks"].(map[string]any)
	if !ok {
		return
	}

	// Check if Stop hook exists (indicates enabled)
	if _, hasStop := hooks["Stop"]; hasStop {
		settings.Enabled = true

		// Try to extract max-tasks and max-minutes from command
		if stopArr, ok := hooks["Stop"].([]any); ok && len(stopArr) > 0 {
			if stopObj, ok := stopArr[0].(map[string]any); ok {
				if hooksArr, ok := stopObj["hooks"].([]any); ok && len(hooksArr) > 0 {
					if hookObj, ok := hooksArr[0].(map[string]any); ok {
						if cmd, ok := hookObj["command"].(string); ok {
							parseHookCommand(cmd, settings)
						}
					}
				}
			}
		}
	}
}

func parseHookCommand(cmd string, settings *HooksSettings) {
	// Parse: "taskwing hook continue-check --max-tasks=5 --max-minutes=30"
	parts := splitCommand(cmd)
	for i, part := range parts {
		if len(part) > 12 && part[:12] == "--max-tasks=" {
			if v, err := strconv.Atoi(part[12:]); err == nil {
				settings.MaxTasks = v
			}
		}
		if len(part) > 14 && part[:14] == "--max-minutes=" {
			if v, err := strconv.Atoi(part[14:]); err == nil {
				settings.MaxMinutes = v
			}
		}
		// Also handle space-separated format
		if part == "--max-tasks" && i+1 < len(parts) {
			if v, err := strconv.Atoi(parts[i+1]); err == nil {
				settings.MaxTasks = v
			}
		}
		if part == "--max-minutes" && i+1 < len(parts) {
			if v, err := strconv.Atoi(parts[i+1]); err == nil {
				settings.MaxMinutes = v
			}
		}
	}
}

func splitCommand(cmd string) []string {
	var parts []string
	var current strings.Builder
	for _, c := range cmd {
		if c == ' ' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func setHooksEnabled(cwd string, enabled bool) error {
	if enabled {
		// Install hooks config
		initializer := bootstrap.NewInitializer(cwd)
		if err := initializer.InstallHooksConfig("claude", true); err != nil {
			return err
		}
		if err := initializer.InstallHooksConfig("codex", true); err != nil {
			return err
		}
		fmt.Println("✅ Hooks enabled")
	} else {
		// Remove hooks from settings files
		if err := removeHooksFromSettings(filepath.Join(cwd, ".claude", "settings.json")); err != nil {
			fmt.Printf("⚠️  Could not update Claude settings: %v\n", err)
		}
		if err := removeHooksFromSettings(filepath.Join(cwd, ".codex", "settings.json")); err != nil {
			fmt.Printf("⚠️  Could not update Codex settings: %v\n", err)
		}
		fmt.Println("✅ Hooks disabled")
	}
	return nil
}

func removeHooksFromSettings(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil // File doesn't exist, nothing to do
	}

	var config map[string]any
	if err := json.Unmarshal(content, &config); err != nil {
		return err
	}

	delete(config, "hooks")

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func setHooksMaxTasks(cwd string, maxTasks int) error {
	return updateHookCommand(cwd, func(cmd string) string {
		return updateCommandFlag(cmd, "--max-tasks", maxTasks)
	})
}

func setHooksMaxMinutes(cwd string, maxMinutes int) error {
	return updateHookCommand(cwd, func(cmd string) string {
		return updateCommandFlag(cmd, "--max-minutes", maxMinutes)
	})
}

func updateHookCommand(cwd string, modifier func(string) string) error {
	// Update both Claude and Codex settings
	paths := []string{
		filepath.Join(cwd, ".claude", "settings.json"),
		filepath.Join(cwd, ".codex", "settings.json"),
	}

	updated := false
	for _, path := range paths {
		if err := updateHookCommandInFile(path, modifier); err == nil {
			updated = true
		}
	}

	if !updated {
		return fmt.Errorf("no hooks config found. Run: taskwing config set hooks.enabled true")
	}

	fmt.Println("✅ Configuration updated")
	return nil
}

func updateHookCommandInFile(path string, modifier func(string) string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config map[string]any
	if err := json.Unmarshal(content, &config); err != nil {
		return err
	}

	hooks, ok := config["hooks"].(map[string]any)
	if !ok {
		return fmt.Errorf("no hooks in config")
	}

	stopArr, ok := hooks["Stop"].([]any)
	if !ok || len(stopArr) == 0 {
		return fmt.Errorf("no Stop hook")
	}

	stopObj, ok := stopArr[0].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid Stop hook format")
	}

	hooksArr, ok := stopObj["hooks"].([]any)
	if !ok || len(hooksArr) == 0 {
		return fmt.Errorf("no hooks array")
	}

	hookObj, ok := hooksArr[0].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid hook format")
	}

	cmd, ok := hookObj["command"].(string)
	if !ok {
		return fmt.Errorf("no command")
	}

	// Apply modifier
	hookObj["command"] = modifier(cmd)

	// Write back
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func updateCommandFlag(cmd, flag string, value int) string {
	parts := splitCommand(cmd)
	found := false

	for i, part := range parts {
		prefix := flag + "="
		if len(part) >= len(prefix) && part[:len(prefix)] == prefix {
			parts[i] = fmt.Sprintf("%s=%d", flag, value)
			found = true
			break
		}
	}

	if !found {
		parts = append(parts, fmt.Sprintf("%s=%d", flag, value))
	}

	return strings.Join(parts, " ")
}

// runConfigInteractive runs the interactive configuration menu
func runConfigInteractive() error {
	for {
		result, err := ui.RunConfigMenu()
		if err != nil {
			return err
		}

		if result.Action == "quit" {
			return nil
		}

		// Handle selection
		switch result.Selected {
		case "bootstrap":
			if err := configureBootstrapModel(); err != nil {
				fmt.Printf("⚠️  %v\n", err)
			}
		case "query":
			if err := configureQueryModel(); err != nil {
				fmt.Printf("⚠️  %v\n", err)
			}
		case "embedding":
			if err := configureEmbedding(); err != nil {
				fmt.Printf("⚠️  %v\n", err)
			}
		case "reranking":
			if err := configureReranking(); err != nil {
				fmt.Printf("⚠️  %v\n", err)
			}
		}
	}
}

func configureBootstrapModel() error {
	fmt.Println("\n🧠 Configure Complex Tasks Model")
	fmt.Println("   Used for: bootstrap, planning, deep analysis")
	fmt.Println()

	selection, err := ui.PromptLLMSelection()
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			return nil
		}
		return err
	}

	// Save as bootstrap model
	configValue := fmt.Sprintf("%s:%s", selection.Provider, selection.Model)
	viper.Set("llm.models.bootstrap", configValue)
	if selection.Provider == llm.ProviderBedrock {
		if err := ensureBedrockRegionConfigured(); err != nil {
			return err
		}
	}

	// Also set as default if not set
	if !viper.IsSet("llm.provider") {
		viper.Set("llm.provider", selection.Provider)
		viper.Set("llm.model", selection.Model)
	}

	if err := writeConfig(); err != nil {
		return err
	}

	fmt.Printf("\n✅ Complex tasks: %s/%s\n", selection.Provider, selection.Model)
	return nil
}

func configureQueryModel() error {
	fmt.Println("\n⚡ Configure Fast Queries Model")
	fmt.Println("   Used for: context lookups, recall (cheaper, faster)")
	fmt.Println()

	selection, err := ui.PromptLLMSelection()
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			return nil
		}
		return err
	}

	// Save as query model
	configValue := fmt.Sprintf("%s:%s", selection.Provider, selection.Model)
	viper.Set("llm.models.query", configValue)
	if selection.Provider == llm.ProviderBedrock {
		if err := ensureBedrockRegionConfigured(); err != nil {
			return err
		}
	}

	if err := writeConfig(); err != nil {
		return err
	}

	fmt.Printf("\n✅ Fast queries: %s/%s\n", selection.Provider, selection.Model)
	return nil
}

func configureEmbedding() error {
	fmt.Println("\n📐 Configure Embeddings")
	fmt.Println("   Used for: semantic search in knowledge base")
	fmt.Println("   Tip: Ollama is free and runs locally")
	fmt.Println()

	selection, err := ui.PromptEmbeddingSelection()
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			return nil
		}
		return err
	}

	// Save embedding config
	viper.Set("llm.embedding_provider", selection.Provider)
	viper.Set("llm.embedding_model", selection.Model)

	// Set default base URL for local providers
	switch selection.Provider {
	case llm.ProviderOllama:
		if !viper.IsSet("llm.embedding_base_url") {
			viper.Set("llm.embedding_base_url", llm.DefaultOllamaURL)
		}
	case llm.ProviderTEI:
		if !viper.IsSet("llm.embedding_base_url") {
			viper.Set("llm.embedding_base_url", llm.DefaultTEIURL)
		}
	}

	if err := writeConfig(); err != nil {
		return err
	}

	fmt.Printf("\n✅ Embeddings: %s/%s\n", selection.Provider, selection.Model)
	return nil
}

func configureReranking() error {
	fmt.Println("\n🔄 Configure Reranking")
	fmt.Println("   Optional: improves search result quality")
	fmt.Println("   Requires: TEI server with reranker model")
	fmt.Println()

	// Simple toggle for now
	enabled := viper.GetBool("retrieval.reranking.enabled")
	if enabled {
		fmt.Println("   Currently: ENABLED")
		fmt.Println()
		fmt.Print("   Disable reranking? [y/N]: ")
		var input string
		_, _ = fmt.Scanln(&input)
		if strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
			viper.Set("retrieval.reranking.enabled", false)
			if err := writeConfig(); err != nil {
				return err
			}
			fmt.Println("\n✅ Reranking disabled")
		}
	} else {
		fmt.Println("   Currently: DISABLED")
		fmt.Println()
		fmt.Print("   Enable reranking? [y/N]: ")
		var input string
		_, _ = fmt.Scanln(&input)
		if strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
			fmt.Print("   TEI server URL [http://localhost:8081]: ")
			var url string
			_, _ = fmt.Scanln(&url)
			if url == "" {
				url = "http://localhost:8081"
			}
			viper.Set("retrieval.reranking.enabled", true)
			viper.Set("retrieval.reranking.base_url", url)
			if err := writeConfig(); err != nil {
				return err
			}
			fmt.Println("\n✅ Reranking enabled")
		}
	}
	return nil
}

func writeConfig() error {
	// Try to write to existing config file
	if err := viper.WriteConfig(); err != nil {
		// If no config file exists yet, create one
		if err := viper.SafeWriteConfig(); err != nil {
			// SafeWriteConfig fails if file exists, so try WriteConfigAs
			configPath := viper.ConfigFileUsed()
			if configPath == "" {
				// No config file loaded, write to default location
				home, homeErr := os.UserHomeDir()
				if homeErr != nil {
					return fmt.Errorf("failed to get home directory: %w", homeErr)
				}
				configPath = filepath.Join(home, ".taskwing", "config.yaml")
				// Ensure directory exists
				if mkErr := os.MkdirAll(filepath.Dir(configPath), 0755); mkErr != nil {
					return fmt.Errorf("failed to create config directory: %w", mkErr)
				}
			}
			if err := viper.WriteConfigAs(configPath); err != nil {
				return fmt.Errorf("failed to write config to %s: %w", configPath, err)
			}
		}
	}
	return nil
}

func ensureBedrockRegionConfigured() error {
	region := config.ResolveBedrockRegion()
	if region == "" {
		fmt.Print("AWS Bedrock region [us-east-1]: ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("read region: %w", err)
		}
		region = strings.TrimSpace(line)
		if region == "" {
			region = "us-east-1"
		}
	}
	viper.Set("llm.bedrock.region", region)
	return nil
}

// runTelemetryStatus displays the current telemetry configuration.
func runTelemetryStatus() error {
	cfg, err := telemetry.Load()
	if err != nil {
		return fmt.Errorf("load telemetry config: %w", err)
	}

	configPath, _ := telemetry.GetConfigPath()

	if isJSON() {
		return printJSON(map[string]any{
			"enabled":       cfg.IsEnabled(),
			"consent_asked": !cfg.NeedsConsent(),
			"anonymous_id":  cfg.AnonymousID,
			"config_path":   configPath,
		})
	}

	fmt.Println("Telemetry Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	status := "Disabled"
	statusIcon := "❌"
	if cfg.IsEnabled() {
		status = "Enabled"
		statusIcon = "✅"
	}

	fmt.Printf("  Status:       %s %s\n", statusIcon, status)
	fmt.Printf("  Anonymous ID: %s\n", cfg.AnonymousID)
	fmt.Printf("  Config file:  %s\n", configPath)
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  taskwing config telemetry enable   Enable telemetry")
	fmt.Println("  taskwing config telemetry disable  Disable telemetry")
	fmt.Println()

	return nil
}

// runTelemetryEnable enables telemetry and saves the config.
func runTelemetryEnable() error {
	cfg, err := telemetry.Load()
	if err != nil {
		return fmt.Errorf("load telemetry config: %w", err)
	}

	cfg.Enable()

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save telemetry config: %w", err)
	}

	if isJSON() {
		return printJSON(map[string]any{
			"enabled": true,
			"message": "Telemetry enabled",
		})
	}

	fmt.Println("✅ Telemetry enabled")
	fmt.Println()
	fmt.Println("Thank you for helping improve TaskWing!")
	fmt.Println("We collect: command names, duration, success/failure, OS, CLI version")
	fmt.Println("We never collect: code, file paths, or personal data")
	return nil
}

// runTelemetryDisable disables telemetry and saves the config.
func runTelemetryDisable() error {
	cfg, err := telemetry.Load()
	if err != nil {
		return fmt.Errorf("load telemetry config: %w", err)
	}

	cfg.Disable()

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save telemetry config: %w", err)
	}

	if isJSON() {
		return printJSON(map[string]any{
			"enabled": false,
			"message": "Telemetry disabled",
		})
	}

	fmt.Println("✅ Telemetry disabled")
	fmt.Println()
	fmt.Println("You can re-enable anytime with: taskwing config telemetry enable")
	return nil
}
