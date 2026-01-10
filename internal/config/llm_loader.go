package config

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/viper"
)

// LoadLLMConfigForRole loads LLM configuration with role-based overrides.
// It checks llm.models.<role> first (e.g., llm.models.query for RoleQuery),
// then falls back to the default llm.provider/model.
// This ensures MCP and CLI both respect role-specific configurations.
func LoadLLMConfigForRole(role llm.ModelRole) (llm.Config, error) {
	// Check for role-specific config first (e.g., llm.models.query: "gemini:gemini-3-flash")
	roleConfigKey := fmt.Sprintf("llm.models.%s", role)
	if viper.IsSet(roleConfigKey) {
		spec := viper.GetString(roleConfigKey)
		return ParseModelSpec(spec, role)
	}
	// Fall back to default config
	return LoadLLMConfig()
}

// ParseModelSpec parses a "provider:model" or "provider/model" spec into an LLM config.
// If only provider is specified, auto-selects the recommended model for the role.
// NOTE: This intentionally does NOT load embedding config (embedding_provider, embedding_model)
// to match CLI behavior. Embeddings fall back to using the main provider. If you need
// embedding config, use LoadLLMConfig() which loads all embedding settings.
// This is exported so CLI can use it directly, avoiding duplicate implementations.
func ParseModelSpec(spec string, role llm.ModelRole) (llm.Config, error) {
	var provider, model string

	// Support both : and / as separators
	spec = strings.Replace(spec, "/", ":", 1)

	if strings.Contains(spec, ":") {
		parts := strings.SplitN(spec, ":", 2)
		provider = strings.ToLower(parts[0])
		model = parts[1]
	} else {
		// Try to infer provider from model name
		if inferredProvider, ok := llm.InferProviderFromModel(spec); ok {
			provider = inferredProvider
			model = spec
		} else {
			// Assume it's a provider name, auto-select model for role
			provider = strings.ToLower(spec)
			if recommended := llm.GetRecommendedModelForRole(provider, role); recommended != nil {
				model = recommended.ID
			} else {
				model = llm.DefaultModelForProvider(provider)
			}
		}
	}

	llmProvider, err := llm.ValidateProvider(provider)
	if err != nil {
		return llm.Config{}, fmt.Errorf("invalid provider in role config: %w", err)
	}

	apiKey := ResolveAPIKey(llmProvider)

	// Validate API key requirement (matching CLI behavior)
	requiresKey := llmProvider == llm.ProviderOpenAI ||
		llmProvider == llm.ProviderAnthropic ||
		llmProvider == llm.ProviderGemini

	if requiresKey && apiKey == "" {
		return llm.Config{}, fmt.Errorf("API key required for %s: set env var %s", provider, llm.GetEnvVarForProvider(provider))
	}

	// Get other config values from viper (matching CLI behavior)
	baseURL := viper.GetString("llm.baseURL")
	if baseURL == "" {
		baseURL = viper.GetString("llm.ollamaURL") // Legacy
	}

	// NOTE: Embedding config is NOT loaded here to match CLI behavior.
	// With empty EmbeddingProvider, client.go falls back to using Provider for embeddings.
	// This ensures consistency: if bootstrap used gemini, queries also use gemini.
	embeddingModel := viper.GetString("llm.embeddingModel") // camelCase only, like CLI

	thinkingBudget := viper.GetInt("llm.thinkingBudget")
	if thinkingBudget == 0 && llm.ModelSupportsThinking(model) {
		thinkingBudget = 8192
	}

	return llm.Config{
		Provider:       llmProvider,
		Model:          model,
		EmbeddingModel: embeddingModel,
		APIKey:         apiKey,
		BaseURL:        baseURL,
		ThinkingBudget: thinkingBudget,
		// EmbeddingProvider, EmbeddingAPIKey, EmbeddingBaseURL left empty
		// client.go will fallback to main Provider for embeddings
	}, nil
}

// LoadLLMConfig loads LLM configuration from Viper and Environment variables.
// It handles precedence: Explicit Viper Config > Environment Variables > Defaults.
// It does NOT handle interactive prompts (that belongs in the CLI layer).
// NOTE: For role-aware config (e.g., query vs bootstrap), use LoadLLMConfigForRole instead.
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
