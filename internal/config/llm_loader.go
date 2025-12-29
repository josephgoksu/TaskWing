package config

import (
	"fmt"
	"os"
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

	// 2) Provider-specific env vars
	envKey := providerEnvKey(provider)

	// OpenAI: allow legacy key; others: ignore legacy to avoid wrong-key usage.
	if provider == llm.ProviderOpenAI {
		legacyKey := keyFromViper("llm.apiKey")
		if legacyKey != "" {
			return legacyKey
		}
		if envKey != "" {
			return envKey
		}
	} else if envKey != "" {
		return envKey
	}

	return ""
}

func providerEnvKey(provider llm.Provider) string {
	switch provider {
	case llm.ProviderOpenAI:
		return strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	case llm.ProviderAnthropic:
		return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	case llm.ProviderGemini:
		key := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
		if key == "" {
			key = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
		}
		return key
	default:
		return ""
	}
}
