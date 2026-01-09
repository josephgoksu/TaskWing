package config

import (
	"fmt"
	"strings"

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
	apiKey := ResolveAPIKey(llmProvider)
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
		embeddingModel = viper.GetString("llm.embedding_model") // snake_case variant
	}

	// 6. Embedding Provider (optional, defaults to main provider)
	var embeddingProvider llm.Provider
	embeddingProviderStr := viper.GetString("llm.embedding_provider")
	if embeddingProviderStr != "" {
		ep, err := llm.ValidateProvider(embeddingProviderStr)
		if err != nil {
			return llm.Config{}, fmt.Errorf("invalid embedding_provider: %w", err)
		}
		embeddingProvider = ep
	}

	// Set embedding model defaults based on embedding provider (or main provider)
	effectiveEmbeddingProvider := embeddingProvider
	if effectiveEmbeddingProvider == "" {
		effectiveEmbeddingProvider = llmProvider
	}
	if embeddingModel == "" {
		switch effectiveEmbeddingProvider {
		case llm.ProviderOpenAI:
			embeddingModel = llm.DefaultOpenAIEmbeddingModel
		case llm.ProviderOllama:
			embeddingModel = llm.DefaultOllamaEmbeddingModel
		}
	}

	// 7. Embedding Base URL (for Ollama/TEI embeddings with different endpoint)
	embeddingBaseURL := viper.GetString("llm.embedding_base_url")
	if embeddingBaseURL == "" {
		switch embeddingProvider {
		case llm.ProviderOllama:
			embeddingBaseURL = llm.DefaultOllamaURL
		case llm.ProviderTEI:
			embeddingBaseURL = llm.DefaultTEIURL
		}
	}

	// 8. Embedding API Key (resolve for embedding provider if different from main)
	var embeddingAPIKey string
	if embeddingProvider != "" && embeddingProvider != llmProvider {
		// Separate embedding provider: resolve its own API key
		embeddingAPIKey = ResolveAPIKey(embeddingProvider)
	}
	// If same provider or empty, client.go will fallback to main APIKey

	return llm.Config{
		Provider:          llmProvider,
		Model:             model,
		EmbeddingModel:    embeddingModel,
		APIKey:            apiKey,
		BaseURL:           baseURL,
		EmbeddingProvider: embeddingProvider,
		EmbeddingAPIKey:   embeddingAPIKey,
		EmbeddingBaseURL:  embeddingBaseURL,
	}, nil
}

// ResolveAPIKey returns the best API key for the given provider using
// per-provider config keys, provider-specific env vars, then legacy config.
func ResolveAPIKey(provider llm.Provider) string {
	keyFromViper := func(path string) string {
		if viper.IsSet(path) {
			return strings.TrimSpace(viper.GetString(path))
		}
		return ""
	}

	// 1) Per-provider config key (llm.apiKeys.<provider>)
	perProviderKey := keyFromViper(fmt.Sprintf("llm.apiKeys.%s", provider))
	if perProviderKey != "" {
		return perProviderKey
	}

	// 2) Provider-specific env vars (centralized in llm.GetEnvValueForProvider)
	envKey := llm.GetEnvValueForProvider(string(provider))

	// OpenAI: allow legacy key; others: ignore legacy to avoid wrong-key usage.
	if provider == llm.ProviderOpenAI {
		legacyKey := keyFromViper("llm.apiKey")
		if legacyKey != "" {
			return legacyKey
		}
	}

	return envKey
}
