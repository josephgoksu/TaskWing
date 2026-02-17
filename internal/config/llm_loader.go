package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/viper"
)

var bedrockHostPattern = regexp.MustCompile(`^bedrock-runtime(-fips)?\.[a-z0-9-]+\.amazonaws\.com(\.cn)?$`)

// isLocalhost returns true if the URL points to a local address (localhost, 127.0.0.1, etc.)
func isLocalhost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0"
}

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
		llmProvider == llm.ProviderGemini ||
		llmProvider == llm.ProviderBedrock ||
		llmProvider == llm.ProviderTaskWing

	if requiresKey && apiKey == "" {
		return llm.Config{}, fmt.Errorf("API key required for %s: set env var %s", provider, llm.GetEnvVarForProvider(provider))
	}

	baseURL, err := ResolveProviderBaseURL(llmProvider)
	if err != nil {
		return llm.Config{}, err
	}

	// NOTE: Embedding config is NOT loaded here to match CLI behavior.
	// With empty EmbeddingProvider, client.go falls back to using Provider for embeddings.
	// This ensures consistency: if bootstrap used gemini, queries also use gemini.
	embeddingModel := viper.GetString("llm.embedding_model")

	thinkingBudget := viper.GetInt("llm.thinkingBudget")
	if thinkingBudget == 0 && llm.ModelSupportsThinking(model) {
		thinkingBudget = 8192
	}

	timeout, err := ResolveLLMTimeout()
	if err != nil {
		return llm.Config{}, err
	}

	return llm.Config{
		Provider:       llmProvider,
		Model:          model,
		EmbeddingModel: embeddingModel,
		APIKey:         apiKey,
		BaseURL:        baseURL,
		ThinkingBudget: thinkingBudget,
		Timeout:        timeout,
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

	// 4. Base URL
	baseURL, err := ResolveProviderBaseURL(llmProvider)
	if err != nil {
		return llm.Config{}, err
	}

	// 5. Embedding Model
	embeddingModel := viper.GetString("llm.embedding_model")

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
		case llm.ProviderBedrock:
			embeddingModel = llm.DefaultBedrockEmbeddingModel
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

	timeout, err := ResolveLLMTimeout()
	if err != nil {
		return llm.Config{}, err
	}

	return llm.Config{
		Provider:          llmProvider,
		Model:             model,
		EmbeddingModel:    embeddingModel,
		APIKey:            apiKey,
		BaseURL:           baseURL,
		EmbeddingProvider: embeddingProvider,
		EmbeddingAPIKey:   embeddingAPIKey,
		EmbeddingBaseURL:  embeddingBaseURL,
		Timeout:           timeout,
	}, nil
}

// ResolveProviderBaseURL returns the resolved base URL for a provider.
// For Bedrock it enforces strict Bedrock OpenAI-compatible endpoint validation.
func ResolveProviderBaseURL(provider llm.Provider) (string, error) {
	switch provider {
	case llm.ProviderOllama:
		baseURL := strings.TrimSpace(viper.GetString("llm.ollamaURL"))
		if baseURL == "" {
			baseURL = llm.DefaultOllamaURL
		}
		return baseURL, nil
	case llm.ProviderBedrock:
		return ResolveBedrockBaseURL()
	case llm.ProviderTaskWing:
		baseURL := strings.TrimSpace(viper.GetString("llm.taskwing.base_url"))
		if baseURL == "" {
			baseURL = llm.DefaultTaskWingURL
		}
		return baseURL, nil
	default:
		// For cloud providers (OpenAI, Anthropic, Gemini), only use llm.baseURL
		// if it looks like a real custom endpoint (not localhost Ollama).
		// This prevents a stale llm.baseURL from routing cloud requests to localhost.
		baseURL := strings.TrimSpace(viper.GetString("llm.baseURL"))
		if baseURL != "" && isLocalhost(baseURL) {
			return "", nil
		}
		return baseURL, nil
	}
}

// ResolveBedrockRegion returns Bedrock region from config first, then AWS env fallbacks.
func ResolveBedrockRegion() string {
	region := strings.TrimSpace(viper.GetString("llm.bedrock.region"))
	if region != "" {
		return region
	}
	region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region != "" {
		return region
	}
	return strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION"))
}

// ResolveBedrockBaseURL returns a validated Bedrock OpenAI-compatible base URL.
func ResolveBedrockBaseURL() (string, error) {
	baseURL := strings.TrimSpace(viper.GetString("llm.bedrock.base_url"))
	if baseURL == "" {
		region := ResolveBedrockRegion()
		if region == "" {
			return "", fmt.Errorf("AWS Bedrock region is required: set llm.bedrock.region or AWS_REGION")
		}
		baseURL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/openai/v1", region)
	}
	if err := ValidateBedrockBaseURL(baseURL); err != nil {
		return "", fmt.Errorf("invalid llm.bedrock.base_url: %w", err)
	}
	return strings.TrimSuffix(baseURL, "/"), nil
}

// ValidateBedrockBaseURL validates strict Bedrock OpenAI-compatible endpoint policy.
func ValidateBedrockBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("must use https")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("missing host")
	}
	if !bedrockHostPattern.MatchString(strings.ToLower(u.Hostname())) {
		return fmt.Errorf("host %q is not a Bedrock runtime endpoint", u.Hostname())
	}
	path := strings.TrimSuffix(u.Path, "/")
	if path != "/openai/v1" {
		return fmt.Errorf("path must be /openai/v1")
	}
	return nil
}

// ResolveLLMTimeout resolves LLM timeout from config or env with defaults.
func ResolveLLMTimeout() (time.Duration, error) {
	if viper.IsSet("llm.timeout") {
		raw := strings.TrimSpace(viper.GetString("llm.timeout"))
		if raw == "" {
			return llm.DefaultRequestTimeout, nil
		}
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return 0, fmt.Errorf("invalid llm.timeout: %w", err)
		}
		return dur, nil
	}
	if viper.IsSet("llm.timeout_seconds") {
		seconds := viper.GetInt("llm.timeout_seconds")
		if seconds < 0 {
			return 0, fmt.Errorf("invalid llm.timeout_seconds: %d", seconds)
		}
		return time.Duration(seconds) * time.Second, nil
	}
	return llm.DefaultRequestTimeout, nil
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
