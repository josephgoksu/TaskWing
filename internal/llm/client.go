// Package llm provides a unified interface for LLM providers using CloudWeGo Eino.
package llm

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	geminiEmbed "github.com/cloudwego/eino-ext/components/embedding/gemini"
	ollamaEmbed "github.com/cloudwego/eino-ext/components/embedding/ollama"
	openaiEmbed "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"google.golang.org/genai"
)

// DefaultRequestTimeout is the default timeout for LLM chat requests.
// Increased from 2 minutes to 5 minutes to support bootstrap analysis of large codebases.
const DefaultRequestTimeout = 5 * time.Minute

// TimeoutEnvVar is the environment variable name for overriding the LLM request timeout.
const TimeoutEnvVar = "TASKWING_LLM_TIMEOUT"

// GetEffectiveTimeout returns the timeout to use for LLM requests.
// Priority: 1) cfg.Timeout if set, 2) TASKWING_LLM_TIMEOUT env var, 3) DefaultRequestTimeout
func GetEffectiveTimeout(cfg *Config) time.Duration {
	// 1. If config explicitly sets a timeout, use it
	if cfg != nil && cfg.Timeout > 0 {
		return cfg.Timeout
	}

	// 2. Check environment variable
	if envVal := os.Getenv(TimeoutEnvVar); envVal != "" {
		if d, err := time.ParseDuration(envVal); err == nil {
			return d
		}
		// Log warning but don't fail - fall back to default
		log.Printf("Warning: invalid %s value %q (expected duration like '5m' or '300s'), using default %v",
			TimeoutEnvVar, envVal, DefaultRequestTimeout)
	}

	// 3. Default timeout
	return DefaultRequestTimeout
}

// Provider identifies the LLM provider to use.
type Provider string

// Config holds configuration for creating an LLM client.
type Config struct {
	Provider       Provider
	Model          string        // Chat model
	EmbeddingModel string        // Embedding model (optional)
	APIKey         string        // Required for cloud providers
	BaseURL        string        // Optional custom endpoint (OpenAI-compatible/Ollama)
	ThinkingBudget int           // Token budget for extended thinking (0 = disabled, only for supported models)
	Timeout        time.Duration // Request timeout for chat completions (0 = no timeout)

	// Embedding-specific provider (optional, defaults to Provider if empty)
	EmbeddingProvider Provider
	EmbeddingAPIKey   string // API key for embedding provider (if different)
	EmbeddingBaseURL  string // Base URL for embedding provider (e.g., Ollama URL)
}

// CloseableChatModel wraps a chat model with optional cleanup.
// Call Close() when done to release resources (required for Gemini).
type CloseableChatModel struct {
	model.BaseChatModel
	closer io.Closer // nil for providers without cleanup needs
}

// Close releases underlying resources. Safe to call multiple times.
func (c *CloseableChatModel) Close() error {
	if c.closer != nil {
		return c.closer.Close()
	}
	return nil
}

// CloseableEmbedder wraps an embedder with optional cleanup.
type CloseableEmbedder struct {
	embedding.Embedder
	closer io.Closer
}

// Close releases underlying resources. Safe to call multiple times.
func (c *CloseableEmbedder) Close() error {
	if c.closer != nil {
		return c.closer.Close()
	}
	return nil
}

// genaiClientCloser wraps genai.Client to implement io.Closer
type genaiClientCloser struct {
	client *genai.Client
}

func (g *genaiClientCloser) Close() error {
	// genai.Client doesn't have a Close method in current SDK
	// but we keep this wrapper for future compatibility and explicit lifecycle
	g.client = nil
	return nil
}

func newOpenAICompatibleChatModel(ctx context.Context, cfg Config, timeout time.Duration) (*CloseableChatModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%s API key is required", cfg.Provider)
	}
	chatCfg := &openai.ChatModelConfig{
		Model:   cfg.Model,
		APIKey:  cfg.APIKey,
		Timeout: timeout,
	}
	if cfg.BaseURL != "" {
		chatCfg.BaseURL = cfg.BaseURL
	}
	m, err := openai.NewChatModel(ctx, chatCfg)
	if err != nil {
		return nil, err
	}
	return &CloseableChatModel{BaseChatModel: m, closer: nil}, nil
}

// NewCloseableChatModel creates a ChatModel with proper resource management.
// Callers MUST call Close() when done to release resources.
func NewCloseableChatModel(ctx context.Context, cfg Config) (*CloseableChatModel, error) {
	timeout := GetEffectiveTimeout(&cfg)

	switch cfg.Provider {
	case ProviderOpenAI:
		return newOpenAICompatibleChatModel(ctx, cfg, timeout)

	case ProviderBedrock:
		return newOpenAICompatibleChatModel(ctx, cfg, timeout)

	case ProviderTaskWing:
		// TaskWing managed service uses OpenAI-compatible API
		if cfg.BaseURL == "" {
			cfg.BaseURL = DefaultTaskWingURL
		}
		return newOpenAICompatibleChatModel(ctx, cfg, timeout)

	case ProviderOllama:
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = DefaultOllamaURL
		}
		m, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			BaseURL: baseURL,
			Model:   cfg.Model,
			Timeout: timeout,
		})
		if err != nil {
			return nil, err
		}
		return &CloseableChatModel{BaseChatModel: m, closer: nil}, nil

	case ProviderAnthropic:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("anthropic API key is required")
		}
		claudeConfig := &claude.Config{
			APIKey: cfg.APIKey,
			Model:  cfg.Model,
		}
		if timeout > 0 {
			claudeConfig.HTTPClient = &http.Client{Timeout: timeout}
		}
		// Enable extended thinking if budget is set and model supports it
		if cfg.ThinkingBudget > 0 && ModelSupportsThinking(cfg.Model) {
			claudeConfig.Thinking = &claude.Thinking{
				Enable:       true,
				BudgetTokens: cfg.ThinkingBudget,
			}
		}
		m, err := claude.NewChatModel(ctx, claudeConfig)
		if err != nil {
			return nil, err
		}
		return &CloseableChatModel{BaseChatModel: m, closer: nil}, nil

	case ProviderGemini:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini API key is required")
		}
		var httpClient *http.Client
		if timeout > 0 {
			httpClient = &http.Client{Timeout: timeout}
		}
		// Create genai.Client with API key
		genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:     cfg.APIKey,
			Backend:    genai.BackendGeminiAPI,
			HTTPClient: httpClient,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}

		geminiConfig := &gemini.Config{
			Client: genaiClient,
			Model:  cfg.Model,
		}
		// Enable thinking mode if budget is set and model supports it
		if cfg.ThinkingBudget > 0 && ModelSupportsThinking(cfg.Model) {
			budget := int32(cfg.ThinkingBudget)
			geminiConfig.ThinkingConfig = &genai.ThinkingConfig{
				IncludeThoughts: true,
				ThinkingBudget:  &budget,
			}
		}

		chatModel, err := gemini.NewChatModel(ctx, geminiConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini chat model: %w", err)
		}
		if chatModel == nil {
			return nil, fmt.Errorf("gemini chat model initialization returned nil")
		}
		return &CloseableChatModel{
			BaseChatModel: chatModel,
			closer:        &genaiClientCloser{client: genaiClient},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: taskwing, openai, ollama, anthropic, bedrock, gemini)", cfg.Provider)
	}
}

// ValidateProvider checks if the given provider string is supported.
func ValidateProvider(p string) (Provider, error) {
	switch Provider(p) {
	case ProviderOpenAI:
		return ProviderOpenAI, nil
	case ProviderOllama:
		return ProviderOllama, nil
	case ProviderAnthropic:
		return ProviderAnthropic, nil
	case ProviderGemini:
		return ProviderGemini, nil
	case ProviderBedrock:
		return ProviderBedrock, nil
	case ProviderTEI:
		return ProviderTEI, nil
	case ProviderTaskWing:
		return ProviderTaskWing, nil
	default:
		return "", fmt.Errorf("unsupported provider: %s", p)
	}
}

// NewCloseableEmbedder creates an Embedder with proper resource management.
// Callers MUST call Close() when done to release resources.
// If EmbeddingProvider is set, it uses that provider for embeddings; otherwise falls back to Provider.
func NewCloseableEmbedder(ctx context.Context, cfg Config) (*CloseableEmbedder, error) {
	// Determine which provider to use for embeddings
	embeddingProvider := cfg.EmbeddingProvider
	if embeddingProvider == "" {
		embeddingProvider = cfg.Provider
	}

	// Resolve API key and base URL for embedding provider
	apiKey := cfg.EmbeddingAPIKey
	if apiKey == "" {
		apiKey = cfg.APIKey
	}
	baseURL := cfg.EmbeddingBaseURL
	if baseURL == "" {
		baseURL = cfg.BaseURL
	}

	switch embeddingProvider {
	case ProviderOpenAI:
		if apiKey == "" {
			return nil, fmt.Errorf("%s API key is required", embeddingProvider)
		}
		modelName := cfg.EmbeddingModel
		if modelName == "" {
			modelName = DefaultOpenAIEmbeddingModel
		}
		embeddingCfg := &openaiEmbed.EmbeddingConfig{
			Model:  modelName,
			APIKey: apiKey,
		}
		if baseURL != "" {
			embeddingCfg.BaseURL = baseURL
		}
		e, err := openaiEmbed.NewEmbedder(ctx, embeddingCfg)
		if err != nil {
			return nil, err
		}
		return &CloseableEmbedder{Embedder: e, closer: nil}, nil

	case ProviderBedrock:
		if apiKey == "" {
			return nil, fmt.Errorf("%s API key is required", embeddingProvider)
		}
		modelName := cfg.EmbeddingModel
		if modelName == "" {
			modelName = DefaultBedrockEmbeddingModel
		}
		embeddingCfg := &openaiEmbed.EmbeddingConfig{
			Model:  modelName,
			APIKey: apiKey,
		}
		if baseURL != "" {
			embeddingCfg.BaseURL = baseURL
		}
		e, err := openaiEmbed.NewEmbedder(ctx, embeddingCfg)
		if err != nil {
			return nil, err
		}
		return &CloseableEmbedder{Embedder: e, closer: nil}, nil

	case ProviderTaskWing:
		if apiKey == "" {
			return nil, fmt.Errorf("TASKWING_API_KEY is required for embeddings")
		}
		modelName := cfg.EmbeddingModel
		if modelName == "" {
			modelName = DefaultOpenAIEmbeddingModel // TaskWing uses OpenAI-compatible embeddings
		}
		taskwingBaseURL := baseURL
		if taskwingBaseURL == "" {
			taskwingBaseURL = DefaultTaskWingURL
		}
		embeddingCfg := &openaiEmbed.EmbeddingConfig{
			Model:   modelName,
			APIKey:  apiKey,
			BaseURL: taskwingBaseURL,
		}
		e, err := openaiEmbed.NewEmbedder(ctx, embeddingCfg)
		if err != nil {
			return nil, err
		}
		return &CloseableEmbedder{Embedder: e, closer: nil}, nil

	case ProviderOllama:
		if baseURL == "" {
			baseURL = DefaultOllamaURL
		}
		modelName := cfg.EmbeddingModel
		if modelName == "" {
			modelName = DefaultOllamaEmbeddingModel
		}
		e, err := ollamaEmbed.NewEmbedder(ctx, &ollamaEmbed.EmbeddingConfig{
			BaseURL: baseURL,
			Model:   modelName,
		})
		if err != nil {
			return nil, err
		}
		return &CloseableEmbedder{Embedder: e, closer: nil}, nil

	case ProviderGemini:
		if apiKey == "" {
			return nil, fmt.Errorf("gemini API key is required")
		}
		// Create genai.Client with API key
		genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}

		embeddingModel := cfg.EmbeddingModel
		if embeddingModel == "" {
			embeddingModel = "text-embedding-004" // Gemini default
		}

		embedder, err := geminiEmbed.NewEmbedder(ctx, &geminiEmbed.EmbeddingConfig{
			Client: genaiClient,
			Model:  embeddingModel,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini embedder: %w", err)
		}
		return &CloseableEmbedder{
			Embedder: embedder,
			closer:   &genaiClientCloser{client: genaiClient},
		}, nil

	case ProviderTEI:
		teiBaseURL := baseURL
		if teiBaseURL == "" {
			teiBaseURL = DefaultTEIURL
		}
		modelName := cfg.EmbeddingModel
		// TEI doesn't require a model name - it uses whatever model the server was started with

		e, err := NewTeiEmbedder(ctx, &TeiConfig{
			BaseURL: teiBaseURL,
			Model:   modelName,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create TEI embedder: %w", err)
		}
		return &CloseableEmbedder{Embedder: e, closer: e}, nil

	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", embeddingProvider)
	}
}
