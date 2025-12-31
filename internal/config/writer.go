package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/llm"
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
  apiKeys:
    %s: %s
`, provider, provider, key)
		if provider == string(llm.ProviderOpenAI) {
			content += fmt.Sprintf("  apiKey: %s\n", key) // Legacy OpenAI key
		}
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
	updatedProviderKey := false
	inLLM := false
	inAPIKeys := false
	apiKeysFound := false
	llmIndent := ""
	apiKeysIndent := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track if we are inside the 'llm:' block
		if trimmed == "llm:" {
			inLLM = true
			inAPIKeys = false
			llmIndent = line[:strings.Index(line, "llm:")]
			newLines = append(newLines, line)
			continue
		}

		if inLLM && inAPIKeys {
			// Still inside apiKeys block if indented beyond apiKeys line
			if strings.HasPrefix(line, apiKeysIndent+"  ") || strings.HasPrefix(line, apiKeysIndent+"\t") {
				keyTrim := strings.TrimSpace(line)
				if strings.HasPrefix(keyTrim, provider+":") {
					newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, key))
					updatedProviderKey = true
					continue
				}
				newLines = append(newLines, line)
				continue
			}
			// Exiting apiKeys block
			if !updatedProviderKey && provider != "" {
				newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, key))
				updatedProviderKey = true
			}
			inAPIKeys = false
			// Fall through to process current line
		}

		if inLLM && strings.HasPrefix(trimmed, "apiKeys:") {
			apiKeysFound = true
			inAPIKeys = true
			apiKeysIndent = line[:strings.Index(line, "apiKeys:")]
			newLines = append(newLines, line)
			continue
		}

		// If we find apiKey inside llm block, replace it (OpenAI only)
		if inLLM && strings.HasPrefix(trimmed, "apiKey:") {
			if provider == string(llm.ProviderOpenAI) {
				indent := line[:strings.Index(line, "apiKey:")]
				newLines = append(newLines, fmt.Sprintf("%sapiKey: %s", indent, key))
				updatedKey = true
				continue
			}
			newLines = append(newLines, line)
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
			if !apiKeysFound && provider != "" {
				newLines = append(newLines, fmt.Sprintf("%s  apiKeys:", llmIndent))
				newLines = append(newLines, fmt.Sprintf("%s    %s: %s", llmIndent, provider, key))
				apiKeysFound = true
				updatedProviderKey = true
			} else if apiKeysFound && !updatedProviderKey && provider != "" {
				newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, key))
				updatedProviderKey = true
			}
			inLLM = false
			inAPIKeys = false
		}

		newLines = append(newLines, line)
	}

	if inAPIKeys && !updatedProviderKey && provider != "" {
		newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, key))
		updatedProviderKey = true
	}
	if inLLM && !apiKeysFound && provider != "" {
		newLines = append(newLines, fmt.Sprintf("%s  apiKeys:", llmIndent))
		newLines = append(newLines, fmt.Sprintf("%s    %s: %s", llmIndent, provider, key))
		updatedProviderKey = true
	}

	if (provider == string(llm.ProviderOpenAI) && !updatedKey) || !updatedProvider || !updatedProviderKey {
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

	v.Set(fmt.Sprintf("llm.apiKeys.%s", provider), key)
	if provider == string(llm.ProviderOpenAI) {
		v.Set("llm.apiKey", key) // Legacy OpenAI key
	}
	if provider != "" {
		v.Set("llm.provider", provider)
	}
	return v.WriteConfig()
}
