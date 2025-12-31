/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getLLMConfig unifies LLM configuration loading across all CLI commands.
// It respects precedence: Flag > Config File > Environment Variable.
func getLLMConfig(cmd *cobra.Command) (llm.Config, error) {
	// Flags support (if the command defined them)
	provider, _ := cmd.Flags().GetString("provider")
	model, err := cmd.Flags().GetString("model")
	if err != nil {
		// Fallback: If "model" is a StringSlice (e.g. eval command), take the first one
		if s, err2 := cmd.Flags().GetStringSlice("model"); err2 == nil && len(s) > 0 {
			model = s[0]
		}
	}
	apiKey, _ := cmd.Flags().GetString("api-key")
	ollamaURL, _ := cmd.Flags().GetString("ollama-url")

	// 1. Provider
	if provider == "" {
		// Only use viper value if explicitly set in config/env, ignoring defaults for now
		if viper.IsSet("llm.provider") {
			provider = viper.GetString("llm.provider")
		}
	}

	// Interactive Provider Selection
	// If provider is still empty (not in flag, not in config) and we are interactive, ask the user!
	if provider == "" && ui.IsInteractive() {
		selectedProvider, err := ui.PromptLLMProvider()
		if err == nil && selectedProvider != "" {
			provider = selectedProvider
			// We will save this preference implicitly when saving the key/config later
		}
	}

	if provider == "" {
		provider = "openai" // Default fallback if non-interactive or prompt ignored
	}

	llmProvider, err := llm.ValidateProvider(provider)
	if err != nil {
		return llm.Config{}, fmt.Errorf("invalid provider: %w", err)
	}

	// 2. Chat Model
	if model == "" {
		model = viper.GetString("llm.model")
	}
	if model == "" {
		model = llm.DefaultModelForProvider(string(llmProvider))
	}

	// 3. API Key
	if apiKey == "" {
		apiKey = config.ResolveAPIKey(llmProvider)
	}

	// Interactive Prompt for API Key (Only if needed for the selected provider)
	requiresKey := llmProvider == llm.ProviderOpenAI ||
		llmProvider == llm.ProviderAnthropic ||
		llmProvider == llm.ProviderGemini

	if requiresKey && apiKey == "" {
		// Only prompt if we are in an interactive terminal
		if ui.IsInteractive() {
			fmt.Printf("No API key found for %s.\n", provider)
			inputKey, err := ui.PromptAPIKey()
			if err != nil {
				// Don't fail hard here, let validation catch it
			} else if inputKey != "" {
				apiKey = inputKey
				// Save Config Globally (Provider + Key)
				if err := config.SaveGlobalLLMConfig(string(llmProvider), apiKey); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to save config: %v\n", err)
				} else {
					fmt.Println("✓ Configuration saved to ~/.taskwing/config.yaml")
				}
			}
		}
	} else if provider != "" && apiKey != "" {
		// If we have both (e.g. from prompt above or env), nice.
		// If user selected Ollama, we might want to save that preference too if it wasn't saved!
		// But let's only save if we prompted for something?
		// Actually, if they selected a provider interactively but didn't need a key (Ollama), we should save that choice!

		// Wait, if provider was selected via prompt, we should save it.
		// I need to track if we prompted.
		// Or simpler: Just always verify config consistency?
		// If Viper doesn't have it, save it?
		if !viper.IsSet("llm.provider") && ui.IsInteractive() {
			// This means it wasn't in config/flags, so it came from our prompt (or default fallback).
			// Re-save is cheap, ignore error.
			_ = config.SaveGlobalLLMConfig(provider, apiKey)
		}
	}

	if requiresKey && apiKey == "" {
		return llm.Config{}, fmt.Errorf("API key required for %s: use --api-key, set config 'llm.apiKeys.%s' (or legacy 'llm.apiKey'), or set provider env var (OPENAI_API_KEY/ANTHROPIC_API_KEY/GEMINI_API_KEY)", provider, provider)
	}

	// 4. Base URL (Ollama)
	if ollamaURL == "" {
		ollamaURL = viper.GetString("llm.baseURL")
	}
	if ollamaURL == "" {
		ollamaURL = viper.GetString("llm.ollamaURL") // Legacy support
	}

	// 5. Embedding Model
	embeddingModel := viper.GetString("llm.embeddingModel")

	return llm.Config{
		Provider:       llmProvider,
		Model:          model,
		EmbeddingModel: embeddingModel,
		APIKey:         apiKey,
		BaseURL:        ollamaURL,
	}, nil
}
