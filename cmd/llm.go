/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

var llmCmd = &cobra.Command{
	Use:   "llm",
	Short: "Manage LLM provider and model configuration",
	Long: `Configure which LLM provider and model TaskWing uses for AI operations.

Running without a subcommand launches interactive provider/model selection.

Examples:
  taskwing llm                         # Interactive selection (recommended)
  taskwing llm show                    # Show current configuration
  taskwing llm use openai/gpt-5-mini   # Switch to OpenAI GPT-5 Mini
  taskwing llm use anthropic/claude-sonnet-4-5  # Switch to Claude
  taskwing llm list                    # List available providers`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLLMInteractive()
	},
}

var llmShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current LLM configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLLMShow()
	},
}

var llmUseCmd = &cobra.Command{
	Use:   "use <provider/model>",
	Short: "Switch to a different LLM provider and model",
	Long: `Quickly switch between LLM providers and models.

Format: provider/model or just provider (uses default model)

Examples:
  taskwing llm use openai/gpt-5-mini
  taskwing llm use openai/o3
  taskwing llm use anthropic/claude-sonnet-4-5
  taskwing llm use ollama/llama3.2
  taskwing llm use gemini/gemini-2.5-pro
  taskwing llm use openai              # Uses default model (gpt-5-mini)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLLMUse(args[0])
	},
}

var llmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available LLM providers and models",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLLMList()
	},
}

func init() {
	rootCmd.AddCommand(llmCmd)
	llmCmd.AddCommand(llmShowCmd)
	llmCmd.AddCommand(llmUseCmd)
	llmCmd.AddCommand(llmListCmd)
}

func runLLMInteractive() error {
	// Launch interactive provider + model selection
	selection, err := ui.PromptLLMSelection()
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			return nil // User cancelled, not an error
		}
		return err
	}

	// Check API key
	apiKey := config.ResolveAPIKey(llm.Provider(selection.Provider))
	if apiKey == "" && selection.Provider != "ollama" {
		providers := llm.GetProviders()
		var envVar string
		for _, p := range providers {
			if p.ID == selection.Provider {
				envVar = p.EnvVar
				break
			}
		}
		fmt.Printf("⚠️  No API key found for %s\n", selection.Provider)
		fmt.Printf("Set via: export %s='your-key'\n", envVar)
		fmt.Println()
	}

	// Save configuration
	if err := config.SaveGlobalLLMConfigWithModel(selection.Provider, selection.Model, apiKey); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("✅ Switched to %s/%s\n", selection.Provider, selection.Model)
	return nil
}

func runLLMShow() error {
	cfg, err := config.LoadLLMConfig()
	if err != nil {
		return fmt.Errorf("load LLM config: %w", err)
	}

	fmt.Println("LLM Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("  Provider: %s\n", cfg.Provider)
	fmt.Printf("  Model:    %s\n", cfg.Model)
	if cfg.EmbeddingModel != "" {
		fmt.Printf("  Embeddings: %s\n", cfg.EmbeddingModel)
	}
	if cfg.BaseURL != "" {
		fmt.Printf("  Base URL: %s\n", cfg.BaseURL)
	}

	// Show API key status (masked) - check both env and config
	fmt.Println()
	fmt.Println("  API Keys:")
	showAPIKeyStatus("openai", config.ResolveAPIKey(llm.ProviderOpenAI))
	showAPIKeyStatus("anthropic", config.ResolveAPIKey(llm.ProviderAnthropic))
	showAPIKeyStatus("gemini", config.ResolveAPIKey(llm.ProviderGemini))

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("To switch: taskwing llm use <provider/model>")
	fmt.Println("Examples:  taskwing llm use openai/gpt-5-mini")
	fmt.Println("           taskwing llm use anthropic/claude-sonnet-4-5")

	return nil
}

func showAPIKeyStatus(provider, key string) {
	if key == "" {
		fmt.Printf("    %s: (not set)\n", provider)
		return
	}
	// Safely mask key - handle short keys
	var masked string
	switch {
	case len(key) >= 8:
		masked = key[:4] + "..." + key[len(key)-4:]
	case len(key) >= 4:
		masked = key[:2] + "..." + key[len(key)-2:]
	default:
		masked = "***"
	}
	fmt.Printf("    %s: %s\n", provider, masked)
}

func runLLMUse(spec string) error {
	var provider, model string

	// Support both / and : as separators
	spec = strings.Replace(spec, ":", "/", 1)

	// Parse provider/model or try to infer provider from model name
	if strings.Contains(spec, "/") {
		parts := strings.SplitN(spec, "/", 2)
		provider = strings.ToLower(parts[0])
		model = parts[1]
	} else {
		// Try to infer provider from model name
		inferredProvider, ok := llm.InferProviderFromModel(spec)
		if ok {
			provider = inferredProvider
			model = spec
			fmt.Printf("ℹ️  Detected provider: %s\n", provider)
		} else {
			// Assume it's a provider name, use default model
			provider = strings.ToLower(spec)
			model = llm.DefaultModelForProvider(provider)
		}
	}

	// Validate provider
	validProvider, err := llm.ValidateProvider(provider)
	if err != nil {
		fmt.Printf("❌ Unknown provider: %s\n", provider)
		fmt.Println()
		fmt.Println("Available providers: openai, anthropic, gemini, ollama")
		fmt.Println()
		fmt.Println("Usage: taskwing llm use <provider>/<model>")
		fmt.Println("   or: taskwing llm use <model>  (auto-detect provider)")
		return err
	}

	// Check if API key is available for this provider
	apiKey := config.ResolveAPIKey(validProvider)
	if apiKey == "" && provider != "ollama" {
		fmt.Printf("⚠️  No API key found for %s\n", provider)
		fmt.Println()
		if envVar := llm.GetEnvVarForProvider(provider); envVar != "" {
			fmt.Printf("Set via: export %s='your-key'\n", envVar)
		}
		fmt.Println()
		// Still save the config so the provider is set
		if err := config.SaveGlobalLLMConfigWithModel(provider, model, ""); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Printf("✅ Switched to %s/%s (API key needed before use)\n", provider, model)
		return nil
	}

	// Save configuration
	if err := config.SaveGlobalLLMConfigWithModel(provider, model, apiKey); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("✅ Switched to %s/%s\n", provider, model)

	return nil
}

func runLLMList() error {
	// Load current config to show active selection
	cfg, _ := config.LoadLLMConfig()
	currentProvider := cfg.Provider
	currentModel := cfg.Model

	fmt.Println("Available LLM Providers")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if currentProvider != "" {
		fmt.Printf("  Current: %s/%s\n", currentProvider, currentModel)
	}
	fmt.Println()

	// Get providers from ModelRegistry (single source of truth)
	providers := llm.GetProviders()

	for _, p := range providers {
		status := "❌"
		if p.IsLocal {
			status = "🏠"
		} else if config.ResolveAPIKey(llm.Provider(p.ID)) != "" {
			status = "✅"
		}

		fmt.Printf("%s %s\n", status, p.DisplayName)
		fmt.Printf("   Default: %s\n", p.DefaultModel)

		// Get model IDs from ModelRegistry
		models := llm.GetModelsForProvider(p.ID)
		modelIDs := make([]string, len(models))
		for i, m := range models {
			modelIDs[i] = m.ID
		}
		fmt.Printf("   Models:  %s\n", strings.Join(modelIDs, ", "))
		fmt.Printf("   Env:     %s\n", p.EnvVar)
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Tip: Run 'taskwing llm' for interactive selection")
	fmt.Println("  or: taskwing llm use openai/gpt-5-mini")

	return nil
}
