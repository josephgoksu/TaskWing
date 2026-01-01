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
// Deprecated: Use SaveGlobalLLMConfigWithModel to also specify the model.
func SaveGlobalLLMConfig(provider, key string) error {
	if provider == "" {
		return fmt.Errorf("provider cannot be empty")
	}
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	model := llm.DefaultModelForProvider(provider)
	return SaveGlobalLLMConfigWithModel(provider, model, key)
}

// SaveGlobalLLMConfigWithModel saves the LLM provider, model, and API key to global config.
func SaveGlobalLLMConfigWithModel(provider, model, key string) error {
	// Input validation
	if provider == "" {
		return fmt.Errorf("provider cannot be empty")
	}
	// Key can be empty for providers like Ollama
	if model == "" {
		model = llm.DefaultModelForProvider(provider)
	}

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
		// Create new file with provider, model, and API key
		// Quote API key to handle special YAML characters (:, #, ", etc.)
		quotedKey := quoteYAMLValue(key)
		content := fmt.Sprintf(`# TaskWing Global Configuration
version: "1"

llm:
  provider: %s
  model: %s
`, provider, model)
		if key != "" {
			content += fmt.Sprintf(`  apiKeys:
    %s: %s
`, provider, quotedKey)
			// Note: No longer writing legacy llm.apiKey - read path handles migration
		}
		return os.WriteFile(configFile, []byte(content), 0600)
	}

	// Update existing
	return updateExistingConfigFileWithModel(configFile, provider, model, key)
}

func updateExistingConfigFileWithModel(path string, provider, model, key string) error {
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
					if key != "" {
						newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, quotedKey))
					}
					updatedProviderKey = true
					continue
				}
				newLines = append(newLines, line)
				continue
			}
			// Exiting apiKeys block
			if !updatedProviderKey && provider != "" && key != "" {
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
			if provider == string(llm.ProviderOpenAI) && key != "" {
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

		// Update model to user-specified model
		if inLLM && strings.HasPrefix(trimmed, "model:") {
			indent := line[:strings.Index(line, "model:")]
			newLines = append(newLines, fmt.Sprintf("%smodel: %s", indent, model))
			updatedModel = true
			continue
		}

		// Detect exit of llm block (a line that is not indented, not a comment, and has a key)
		if inLLM && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, "#") && strings.Contains(line, ":") {
			if !apiKeysFound && provider != "" && key != "" {
				newLines = append(newLines, fmt.Sprintf("%s  apiKeys:", llmIndent))
				newLines = append(newLines, fmt.Sprintf("%s    %s: %s", llmIndent, provider, quotedKey))
				apiKeysFound = true
				updatedProviderKey = true
			} else if apiKeysFound && !updatedProviderKey && provider != "" && key != "" {
				newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, quotedKey))
				updatedProviderKey = true
			}
			inLLM = false
			inAPIKeys = false
		}

		newLines = append(newLines, line)
	}

	if inAPIKeys && !updatedProviderKey && provider != "" && key != "" {
		newLines = append(newLines, fmt.Sprintf("%s  %s: %s", apiKeysIndent, provider, quotedKey))
		updatedProviderKey = true
	}
	if inLLM && !apiKeysFound && provider != "" && key != "" {
		newLines = append(newLines, fmt.Sprintf("%s  apiKeys:", llmIndent))
		newLines = append(newLines, fmt.Sprintf("%s    %s: %s", llmIndent, provider, quotedKey))
		updatedProviderKey = true
	}

	// Skip key validation for Ollama (no key needed)
	needsKey := key != "" || provider == string(llm.ProviderOllama)
	if (provider == string(llm.ProviderOpenAI) && key != "" && !updatedKey) || !updatedProvider || (needsKey && key != "" && !updatedProviderKey) || !updatedModel {
		return updateConfigWithViperAndModel(path, provider, model, key)
	}

	return os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0600)
}

func updateConfigWithViperAndModel(path string, provider, model, key string) error {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Read existing if any to preserve other settings
	if err := v.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return err
	}

	if key != "" {
		v.Set(fmt.Sprintf("llm.apiKeys.%s", provider), key)
		// Note: No longer writing to legacy llm.apiKey - read path handles migration
	}
	if provider != "" {
		v.Set("llm.provider", provider)
		v.Set("llm.model", model)
	}
	return v.WriteConfig()
}

// SaveAPIKeyForProvider saves only the API key for a specific provider without
// changing the default provider or model. This is used when auto-detecting provider
// from model name - we want to save the key but not change the user's preferred defaults.
func SaveAPIKeyForProvider(provider, key string) error {
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

	configFile := filepath.Join(configDir, "config.yaml")

	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")

	// Read existing config
	if err := v.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Only set the API key, don't touch provider or model
	v.Set(fmt.Sprintf("llm.apiKeys.%s", provider), key)
	// Note: No longer writing to legacy llm.apiKey - read path handles migration

	return v.WriteConfig()
}
