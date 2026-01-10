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
// IMPORTANT: If a model is specified, the provider is inferred from the model name
// to enable seamless cross-provider usage (e.g., switching from gemini to gpt models).
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

	// Track if we need to prompt for interactive setup
	providerFromPrompt := false
	modelFromPrompt := false

	// 1. Get model first (before provider) - this enables model-based provider inference
	if model == "" {
		if viper.IsSet("llm.model") {
			model = viper.GetString("llm.model")
		}
	}

	// 2. Provider - with model-based inference
	providerInferredFromModel := false
	if provider == "" {
		// If model is specified, try to infer provider from model name
		// This is KEY for cross-provider usage (e.g., -m gpt-5-mini when config says gemini)
		if model != "" {
			if inferredProvider, ok := llm.InferProviderFromModel(model); ok {
				provider = inferredProvider
				providerInferredFromModel = true
			}
		}
	}

	// Fall back to config if not inferred
	if provider == "" {
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
			providerFromPrompt = true
		}
	}

	if provider == "" {
		provider = "openai" // Default fallback if non-interactive or prompt ignored
	}

	llmProvider, err := llm.ValidateProvider(provider)
	if err != nil {
		return llm.Config{}, fmt.Errorf("invalid provider: %w", err)
	}

	// Interactive Model Selection (if provider was just selected or model not configured)
	if model == "" && ui.IsInteractive() {
		selectedModel, err := ui.PromptModelSelection(provider)
		if err == nil && selectedModel != "" {
			model = selectedModel
			modelFromPrompt = true
		}
	}

	// Final fallback to provider default
	if model == "" {
		model = llm.DefaultModelForProvider(string(llmProvider))
	}

	// 3. API Key - resolve for the ACTUAL provider being used
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
				fmt.Fprintf(os.Stderr, "Warning: API key prompt failed: %v\n", err)
			} else if inputKey != "" {
				apiKey = inputKey
				// Save Config Globally (Provider + Model + Key)
				// Only save if NOT inferred from model (to avoid overwriting user's default config)
				if !providerInferredFromModel {
					if err := config.SaveGlobalLLMConfigWithModel(string(llmProvider), model, apiKey); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to save config: %v\n", err)
					} else {
						fmt.Println("✓ Configuration saved to ~/.taskwing/config.yaml")
					}
				} else {
					// Just save the API key for this provider, don't change default provider
					if err := config.SaveAPIKeyForProvider(string(llmProvider), apiKey); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to save API key: %v\n", err)
					} else {
						fmt.Printf("✓ API key for %s saved to ~/.taskwing/config.yaml\n", provider)
					}
				}
				// Also update Viper's in-memory config so subsequent calls in this process find the key
				viper.Set(fmt.Sprintf("llm.apiKeys.%s", llmProvider), apiKey)
			}
		}
	} else if (providerFromPrompt || modelFromPrompt) && ui.IsInteractive() {
		// Save if we interactively selected provider or model
		if err := config.SaveGlobalLLMConfigWithModel(provider, model, apiKey); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save config: %v\n", err)
		}
	}

	if requiresKey && apiKey == "" {
		return llm.Config{}, fmt.Errorf("API key required for %s: use --api-key, set config 'llm.apiKeys.%s', or set env var (%s)", provider, provider, llm.GetEnvVarForProvider(string(llmProvider)))
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

	// 6. Thinking Budget (for models that support extended thinking)
	thinkingBudget := viper.GetInt("llm.thinkingBudget")
	if thinkingBudget == 0 && llm.ModelSupportsThinking(model) {
		// Default thinking budget for supported models (8K tokens)
		thinkingBudget = 8192
	}

	return llm.Config{
		Provider:       llmProvider,
		Model:          model,
		EmbeddingModel: embeddingModel,
		APIKey:         apiKey,
		BaseURL:        ollamaURL,
		ThinkingBudget: thinkingBudget,
	}, nil
}

// getLLMConfigForRole returns the appropriate LLM config for a specific role.
// It respects precedence: Role-specific config > Default config > Environment.
//
// Role-specific config keys:
//   - llm.models.bootstrap: "provider:model" for bootstrap/planning tasks
//   - llm.models.query: "provider:model" for context/recall queries
//
// If no role-specific config is set, falls back to getLLMConfig().
func getLLMConfigForRole(cmd *cobra.Command, role llm.ModelRole) (llm.Config, error) {
	// Check for role-specific config
	roleConfigKey := fmt.Sprintf("llm.models.%s", role)
	if viper.IsSet(roleConfigKey) {
		spec := viper.GetString(roleConfigKey)
		// Use shared implementation from config package (single source of truth)
		return config.ParseModelSpec(spec, role)
	}

	// Fall back to default config (handles interactive prompts and flags)
	return getLLMConfig(cmd)
}

