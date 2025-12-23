package config

import (
	"fmt"
	"os"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/viper"
)

// LoadLLMConfig loads LLM configuration from Viper and Environment variables.
// It handles precedence: Explicit Viper Config > Environment Variables > Defaults.
// It does NOT handle interactive prompts (that belongs in the CLI layer).
func LoadLLMConfig() (llm.Config, error) {
	// 1. Provider
	provider := viper.GetString("llm.provider")
	if provider == "" {
		provider = llm.DefaultProvider
	}

	llmProvider, err := llm.ValidateProvider(provider)
	if err != nil {
		return llm.Config{}, fmt.Errorf("invalid provider: %w", err)
	}

	// 2. Model
	model := viper.GetString("llm.model")
	if model == "" {
		model = llm.DefaultModelForProvider(string(llmProvider))
	}

	// 3. API Key
	apiKey := viper.GetString("llm.apiKey")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	// Note: We don't error on missing API key here, as interactive mode might ask for it later.
	// Or non-auth providers (Ollama) might not need it.

	// 4. Base URL (Ollama or Custom)
	baseURL := viper.GetString("llm.baseURL")
	if baseURL == "" {
		baseURL = viper.GetString("llm.ollamaURL") // Legacy
	}
	if baseURL == "" && llmProvider == llm.ProviderOllama {
		baseURL = llm.DefaultOllamaURL
	}

	// 5. Embedding Model
	embeddingModel := viper.GetString("llm.embeddingModel")
	if embeddingModel == "" {
		// Set defaults for embeddings if not specified
		switch llmProvider {
		case llm.ProviderOpenAI:
			embeddingModel = llm.DefaultOpenAIEmbeddingModel
		case llm.ProviderOllama:
			embeddingModel = llm.DefaultOllamaEmbeddingModel
		}
	}

	return llm.Config{
		Provider:       llmProvider,
		Model:          model,
		EmbeddingModel: embeddingModel,
		APIKey:         apiKey,
		BaseURL:        baseURL,
	}, nil
}
