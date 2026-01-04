/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/cobra"
)

var llmCmd = &cobra.Command{
	Use:   "llm",
	Short: "Manage LLM provider and model configuration",
	Long: `Configure which LLM provider and model TaskWing uses for AI operations.

Examples:
  taskwing llm show                    # Show current configuration
  taskwing llm use openai/gpt-4o       # Switch to OpenAI GPT-4o
  taskwing llm use anthropic/claude-sonnet-4-20250514  # Switch to Claude
  taskwing llm use ollama/llama3       # Switch to local Ollama
  taskwing llm list                    # List available providers`,
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
  taskwing llm use openai/gpt-4o
  taskwing llm use openai/gpt-4-turbo
  taskwing llm use anthropic/claude-sonnet-4-20250514
  taskwing llm use ollama/llama3
  taskwing llm use gemini/gemini-2.0-flash
  taskwing llm use openai              # Uses default model (gpt-4o)`,
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

	// Show API key status (masked)
	fmt.Println()
	fmt.Println("  API Keys:")
	showAPIKeyStatus("openai", os.Getenv("OPENAI_API_KEY"))
	showAPIKeyStatus("anthropic", os.Getenv("ANTHROPIC_API_KEY"))
	showAPIKeyStatus("gemini", os.Getenv("GEMINI_API_KEY"))

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("To switch: taskwing llm use <provider/model>")
	fmt.Println("Examples:  taskwing llm use openai/gpt-4o")
	fmt.Println("           taskwing llm use anthropic/claude-sonnet-4-20250514")

	return nil
}

func showAPIKeyStatus(provider, key string) {
	if key != "" {
		masked := key[:4] + "..." + key[len(key)-4:]
		fmt.Printf("    %s: %s\n", provider, masked)
	} else {
		fmt.Printf("    %s: (not set)\n", provider)
	}
}

func runLLMUse(spec string) error {
	var provider, model string

	// Parse provider/model or just provider
	if strings.Contains(spec, "/") {
		parts := strings.SplitN(spec, "/", 2)
		provider = strings.ToLower(parts[0])
		model = parts[1]
	} else {
		provider = strings.ToLower(spec)
		model = llm.DefaultModelForProvider(provider)
	}

	// Validate provider
	validProvider, err := llm.ValidateProvider(provider)
	if err != nil {
		fmt.Printf("❌ Unknown provider: %s\n", provider)
		fmt.Println()
		fmt.Println("Available providers: openai, anthropic, gemini, ollama")
		return err
	}

	// Check if API key is available for this provider
	apiKey := config.ResolveAPIKey(validProvider)
	if apiKey == "" && provider != "ollama" {
		fmt.Printf("⚠️  No API key found for %s\n", provider)
		fmt.Println()
		switch provider {
		case "openai":
			fmt.Println("Set via: export OPENAI_API_KEY='your-key'")
		case "anthropic":
			fmt.Println("Set via: export ANTHROPIC_API_KEY='your-key'")
		case "gemini":
			fmt.Println("Set via: export GEMINI_API_KEY='your-key'")
		}
		fmt.Println()
		fmt.Println("Or add to ~/.taskwing/config.yaml:")
		fmt.Printf("  llm:\n    apiKeys:\n      %s: your-key\n", provider)
		return fmt.Errorf("API key required for %s", provider)
	}

	// Save configuration
	if err := config.SaveGlobalLLMConfigWithModel(provider, model, apiKey); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("✅ Switched to %s/%s\n", provider, model)

	return nil
}

func runLLMList() error {
	fmt.Println("Available LLM Providers")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	providers := []struct {
		name         string
		envVar       string
		defaultModel string
		models       []string
	}{
		{
			name:         "openai",
			envVar:       "OPENAI_API_KEY",
			defaultModel: llm.DefaultModelForProvider("openai"),
			models:       []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "o1", "o1-mini", "o3-mini"},
		},
		{
			name:         "anthropic",
			envVar:       "ANTHROPIC_API_KEY",
			defaultModel: llm.DefaultModelForProvider("anthropic"),
			models:       []string{"claude-sonnet-4-20250514", "claude-opus-4-20250514", "claude-3-5-sonnet-20241022", "claude-3-haiku-20240307"},
		},
		{
			name:         "gemini",
			envVar:       "GEMINI_API_KEY",
			defaultModel: llm.DefaultModelForProvider("gemini"),
			models:       []string{"gemini-2.0-flash", "gemini-1.5-pro", "gemini-1.5-flash"},
		},
		{
			name:         "ollama",
			envVar:       "(local)",
			defaultModel: llm.DefaultModelForProvider("ollama"),
			models:       []string{"llama3", "llama3.2", "mistral", "codellama", "deepseek-coder"},
		},
	}

	for _, p := range providers {
		status := "❌"
		if p.name == "ollama" {
			status = "🏠"
		} else if os.Getenv(p.envVar) != "" {
			status = "✅"
		}

		fmt.Printf("%s %s\n", status, p.name)
		fmt.Printf("   Default: %s\n", p.defaultModel)
		fmt.Printf("   Models:  %s\n", strings.Join(p.models, ", "))
		fmt.Printf("   Env:     %s\n", p.envVar)
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Usage: taskwing llm use <provider>/<model>")
	fmt.Println("  e.g. taskwing llm use openai/gpt-4o")

	return nil
}
