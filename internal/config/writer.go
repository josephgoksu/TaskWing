package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/viper"
)

// quoteYAMLValue quotes a string value for safe YAML serialization.
// Handles special characters: :, #, ", ', newlines, etc.
func quoteYAMLValue(value string) string {
	// If value contains any YAML special characters, wrap in double quotes
	// and escape internal double quotes
	needsQuoting := strings.ContainsAny(value, ":{}[]&*#?|-<>=!%@`\"'\n\r\t ")
	if !needsQuoting {
		return value
	}
	// Escape backslashes first, then double quotes
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	escaped = strings.ReplaceAll(escaped, "\r", `\r`)
	escaped = strings.ReplaceAll(escaped, "\t", `\t`)
	return `"` + escaped + `"`
}

// SaveGlobalLLMConfig saves the LLM provider, API key, and default model to global config.
func SaveGlobalLLMConfig(provider, key string) error {
	// Input validation
	if provider == "" {
		return fmt.Errorf("provider cannot be empty")
	}
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	configDir, err := GetGlobalConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configFile := filepath.Join(configDir, "config.yaml")

	// Get the default model for this provider
	defaultModel := llm.DefaultModelForProvider(provider)

	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Create new file with provider, model, and API key
		// Quote API key to handle special YAML characters (:, #, ", etc.)
		quotedKey := quoteYAMLValue(key)
		content := fmt.Sprintf(`# TaskWing Global Configuration
version: "1"

llm:
  provider: %s
  model: %s
  apiKeys:
    %s: %s
`, provider, defaultModel, provider, quotedKey)
		if provider == string(llm.ProviderOpenAI) {
			content += fmt.Sprintf("  apiKey: %s\n", quotedKey) // Legacy OpenAI key
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
	updatedModel := false
	inLLM := false
	inAPIKeys := false
	apiKeysFound := false
	llmIndent := ""
	apiKeysIndent := ""

	// Get default model for the new provider
	defaultModel := llm.DefaultModelForProvider(provider)
	// Quote API key for safe YAML serialization
	quotedKey := quoteYAMLValue(key)

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
					newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, quotedKey))
					updatedProviderKey = true
					continue
				}
				newLines = append(newLines, line)
				continue
			}
			// Exiting apiKeys block
			if !updatedProviderKey && provider != "" {
				newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, quotedKey))
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
				newLines = append(newLines, fmt.Sprintf("%sapiKey: %s", indent, quotedKey))
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

		// Update model when provider changes
		if inLLM && strings.HasPrefix(trimmed, "model:") {
			indent := line[:strings.Index(line, "model:")]
			newLines = append(newLines, fmt.Sprintf("%smodel: %s", indent, defaultModel))
			updatedModel = true
			continue
		}

		// Detect exit of llm block (a line that is not indented, not a comment, and has a key)
		if inLLM && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "#") && strings.Contains(line, ":") {
			if !apiKeysFound && provider != "" {
				newLines = append(newLines, fmt.Sprintf("%s  apiKeys:", llmIndent))
				newLines = append(newLines, fmt.Sprintf("%s    %s: %s", llmIndent, provider, quotedKey))
				apiKeysFound = true
				updatedProviderKey = true
			} else if apiKeysFound && !updatedProviderKey && provider != "" {
				newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, quotedKey))
				updatedProviderKey = true
			}
			inLLM = false
			inAPIKeys = false
		}

		newLines = append(newLines, line)
	}

	if inAPIKeys && !updatedProviderKey && provider != "" {
		newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, quotedKey))
		updatedProviderKey = true
	}
	if inLLM && !apiKeysFound && provider != "" {
		newLines = append(newLines, fmt.Sprintf("%s  apiKeys:", llmIndent))
		newLines = append(newLines, fmt.Sprintf("%s    %s: %s", llmIndent, provider, quotedKey))
		updatedProviderKey = true
	}

	if (provider == string(llm.ProviderOpenAI) && !updatedKey) || !updatedProvider || !updatedProviderKey || !updatedModel {
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
		// Also update model to match provider's default
		v.Set("llm.model", llm.DefaultModelForProvider(provider))
	}
	return v.WriteConfig()
}
