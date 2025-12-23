package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// SaveGlobalLLMConfig saves the LLM provider and API key to global config.
func SaveGlobalLLMConfig(provider, key string) error {
	configDir, err := GetGlobalConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configFile := filepath.Join(configDir, "config.yaml")

	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Create new file
		content := fmt.Sprintf(`# TaskWing Global Configuration
version: "1"

llm:
  provider: %s
  apiKey: %s
`, provider, key)
		return os.WriteFile(configFile, []byte(content), 0600)
	}

	// Update existing
	return updateExistingConfigFile(configFile, provider, key)
}

func updateExistingConfigFile(path string, provider, key string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(bytes)

	lines := strings.Split(content, "\n")
	newLines := make([]string, 0, len(lines))
	updatedKey := false
	updatedProvider := false
	inLLM := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track if we are inside the 'llm:' block
		if trimmed == "llm:" {
			inLLM = true
			newLines = append(newLines, line)
			continue
		}

		// If we find apiKey inside llm block, replace it
		if inLLM && strings.HasPrefix(trimmed, "apiKey:") {
			indent := line[:strings.Index(line, "apiKey:")]
			newLines = append(newLines, fmt.Sprintf("%sapiKey: %s", indent, key))
			updatedKey = true
			continue
		}

		if inLLM && strings.HasPrefix(trimmed, "provider:") {
			indent := line[:strings.Index(line, "provider:")]
			newLines = append(newLines, fmt.Sprintf("%sprovider: %s", indent, provider))
			updatedProvider = true
			continue
		}

		// Detect exit of llm block (a line that is not indented, not a comment, and has a key)
		if inLLM && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "#") && strings.Contains(line, ":") {
			inLLM = false
		}

		newLines = append(newLines, line)
	}

	if !updatedKey || (provider != "" && !updatedProvider) {
		return updateConfigWithViper(path, provider, key)
	}

	return os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0600)
}

func updateConfigWithViper(path string, provider, key string) error {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Read existing if any to preserve other settings
	if err := v.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return err
	}

	v.Set("llm.apiKey", key)
	if provider != "" {
		v.Set("llm.provider", provider)
	}
	return v.WriteConfig()
}
