/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
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

func init() {
	// Add subcommands to existing configCmd (defined in telemetry_config.go)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
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

	default:
		return fmt.Errorf("unknown config key: %s", key)
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
		fmt.Println(settings.Enabled)
	case "hooks.max-tasks":
		fmt.Println(settings.MaxTasks)
	case "hooks.max-minutes":
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
	var current string
	for _, c := range cmd {
		if c == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func setHooksEnabled(cwd string, enabled bool) error {
	if enabled {
		// Install hooks config
		if err := installHooksConfig(cwd, "claude", true); err != nil {
			return err
		}
		if err := installHooksConfig(cwd, "codex", true); err != nil {
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

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += " "
		}
		result += part
	}
	return result
}
