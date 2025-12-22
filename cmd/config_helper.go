/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getLLMConfig unifies LLM configuration loading across all CLI commands.
// It respects precedence: Flag > Config File > Environment Variable.
func getLLMConfig(cmd *cobra.Command) (llm.Config, error) {
	// Flags support (if the command defined them)
	provider, _ := cmd.Flags().GetString("provider")
	model, _ := cmd.Flags().GetString("model")
	apiKey, _ := cmd.Flags().GetString("api-key")
	ollamaURL, _ := cmd.Flags().GetString("ollama-url")

	// 1. Provider
	if provider == "" {
		provider = viper.GetString("llm.provider")
	}
	if provider == "" {
		provider = "openai" // Default
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
		model = llm.DefaultModelForProvider(llmProvider)
	}

	// 3. API Key
	if apiKey == "" {
		apiKey = viper.GetString("llm.apiKey")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if llmProvider == llm.ProviderOpenAI && apiKey == "" {
		return llm.Config{}, fmt.Errorf("OpenAI API key required: use --api-key, set config 'llm.apiKey', or set OPENAI_API_KEY env var")
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
