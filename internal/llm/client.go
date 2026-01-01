// Package llm provides a unified interface for LLM providers using CloudWeGo Eino.
package llm

import (
	"context"
	"fmt"
	"io"

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

// Provider identifies the LLM provider to use.
type Provider string

// Config holds configuration for creating an LLM client.
type Config struct {
	Provider       Provider
	Model          string // Chat model
	EmbeddingModel string // Embedding model (optional)
	APIKey         string // Required for OpenAI
	BaseURL        string // Required for Ollama (default: http://localhost:11434)
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

// NewChatModel creates a ChatModel instance based on the provider configuration.
// It returns an Eino BaseChatModel that can be used for Generate() or Stream() calls.
// Deprecated: Use NewCloseableChatModel instead and call Close() when done.
func NewChatModel(ctx context.Context, cfg Config) (model.BaseChatModel, error) {
	cm, err := NewCloseableChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return cm.BaseChatModel, nil
}

// NewCloseableChatModel creates a ChatModel with proper resource management.
// Callers MUST call Close() when done to release resources.
func NewCloseableChatModel(ctx context.Context, cfg Config) (*CloseableChatModel, error) {
	switch cfg.Provider {
	case ProviderOpenAI:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required")
		}
		m, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
			Model:  cfg.Model,
			APIKey: cfg.APIKey,
		})
		if err != nil {
			return nil, err
		}
		return &CloseableChatModel{BaseChatModel: m, closer: nil}, nil

	case ProviderOllama:
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = DefaultOllamaURL
		}
		m, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			BaseURL: baseURL,
			Model:   cfg.Model,
		})
		if err != nil {
			return nil, err
		}
		return &CloseableChatModel{BaseChatModel: m, closer: nil}, nil

	case ProviderAnthropic:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("anthropic API key is required")
		}
		m, err := claude.NewChatModel(ctx, &claude.Config{
			APIKey: cfg.APIKey,
			Model:  cfg.Model,
		})
		if err != nil {
			return nil, err
		}
		return &CloseableChatModel{BaseChatModel: m, closer: nil}, nil

	case ProviderGemini:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini API key is required")
		}
		// Create genai.Client with API key
		genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  cfg.APIKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}

		chatModel, err := gemini.NewChatModel(ctx, &gemini.Config{
			Client: genaiClient,
			Model:  cfg.Model,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini chat model: %w", err)
		}
		if chatModel == nil {
			return nil, fmt.Errorf("Gemini chat model initialization returned nil")
		}
		return &CloseableChatModel{
			BaseChatModel: chatModel,
			closer:        &genaiClientCloser{client: genaiClient},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: openai, ollama, anthropic, gemini)", cfg.Provider)
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
	default:
		return "", fmt.Errorf("unsupported provider: %s", p)
	}
}

// NewEmbeddingModel creates an EmbeddingModel instance based on the provider configuration.
// Deprecated: Use NewCloseableEmbedder instead and call Close() when done.
func NewEmbeddingModel(ctx context.Context, cfg Config) (embedding.Embedder, error) {
	ce, err := NewCloseableEmbedder(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return ce.Embedder, nil
}

// NewCloseableEmbedder creates an Embedder with proper resource management.
// Callers MUST call Close() when done to release resources.
func NewCloseableEmbedder(ctx context.Context, cfg Config) (*CloseableEmbedder, error) {
	switch cfg.Provider {
	case ProviderOpenAI:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required")
		}
		modelName := cfg.EmbeddingModel
		if modelName == "" {
			modelName = DefaultOpenAIEmbeddingModel
		}
		e, err := openaiEmbed.NewEmbedder(ctx, &openaiEmbed.EmbeddingConfig{
			Model:  modelName,
			APIKey: cfg.APIKey,
		})
		if err != nil {
			return nil, err
		}
		return &CloseableEmbedder{Embedder: e, closer: nil}, nil

	case ProviderOllama:
		baseURL := cfg.BaseURL
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
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini API key is required")
		}
		// Create genai.Client with API key
		genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  cfg.APIKey,
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

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}
