// Package llm provides a unified interface for LLM providers using CloudWeGo Eino.
package llm

import (
	"context"
	"fmt"

	ollamaEmbed "github.com/cloudwego/eino-ext/components/embedding/ollama"
	openaiEmbed "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/josephgoksu/TaskWing/internal/config"
)

// Provider identifies the LLM provider to use.
type Provider string

const (
	ProviderOpenAI Provider = Provider(config.ProviderOpenAI)
	ProviderOllama Provider = Provider(config.ProviderOllama)
)

// Config holds configuration for creating an LLM client.
type Config struct {
	Provider       Provider
	Model          string // Chat model
	EmbeddingModel string // Embedding model (optional)
	APIKey         string // Required for OpenAI
	BaseURL        string // Required for Ollama (default: http://localhost:11434)
}

// NewChatModel creates a ChatModel instance based on the provider configuration.
// It returns an Eino BaseChatModel that can be used for Generate() or Stream() calls.
func NewChatModel(ctx context.Context, cfg Config) (model.BaseChatModel, error) {
	switch cfg.Provider {
	case ProviderOpenAI:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required")
		}
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{
			Model:  cfg.Model,
			APIKey: cfg.APIKey,
		})

	case ProviderOllama:
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = config.DefaultOllamaURL
		}
		return ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			BaseURL: baseURL,
			Model:   cfg.Model,
		})

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: openai, ollama)", cfg.Provider)
	}
}

// ValidateProvider checks if the given provider string is supported.
func ValidateProvider(p string) (Provider, error) {
	switch Provider(p) {
	case ProviderOpenAI:
		return ProviderOpenAI, nil
	case ProviderOllama:
		return ProviderOllama, nil
	default:
		return "", fmt.Errorf("unsupported provider: %s", p)
	}
}

// DefaultModelForProvider returns the default model for a given provider.
// This is a convenience wrapper around config.DefaultModelForProvider.
func DefaultModelForProvider(p Provider) string {
	return config.DefaultModelForProvider(string(p))
}

// NewEmbeddingModel creates an EmbeddingModel instance based on the provider configuration.
func NewEmbeddingModel(ctx context.Context, cfg Config) (embedding.Embedder, error) {
	switch cfg.Provider {
	case ProviderOpenAI:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required")
		}
		// Default to text-embedding-3-small if not specified
		modelName := cfg.EmbeddingModel
		if modelName == "" {
			modelName = "text-embedding-3-small"
		}
		return openaiEmbed.NewEmbedder(ctx, &openaiEmbed.EmbeddingConfig{
			Model:  modelName,
			APIKey: cfg.APIKey,
		})

	case ProviderOllama:
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = config.DefaultOllamaURL
		}
		// Default to mxbai-embed-large or similar if not specified, but usually caller specifies
		modelName := cfg.EmbeddingModel
		if modelName == "" {
			modelName = "nomic-embed-text" // Common default for Ollama
		}
		return ollamaEmbed.NewEmbedder(ctx, &ollamaEmbed.EmbeddingConfig{
			BaseURL: baseURL,
			Model:   modelName,
		})

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}
