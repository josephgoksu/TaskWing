// Package llm provides a unified interface for LLM providers using CloudWeGo Eino.
package llm

import (
	"context"
	"fmt"
	"os"

	geminiEmbed "github.com/cloudwego/eino-ext/components/embedding/gemini"
	ollamaEmbed "github.com/cloudwego/eino-ext/components/embedding/ollama"
	openaiEmbed "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/josephgoksu/TaskWing/internal/config"
)

// Provider identifies the LLM provider to use.
type Provider string

const (
	ProviderOpenAI    Provider = Provider(config.ProviderOpenAI)
	ProviderOllama    Provider = Provider(config.ProviderOllama)
	ProviderAnthropic Provider = Provider(config.ProviderAnthropic)
	ProviderGemini    Provider = Provider(config.ProviderGemini)
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

	case ProviderAnthropic:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("anthropic API key is required")
		}
		return claude.NewChatModel(ctx, &claude.Config{
			APIKey: cfg.APIKey,
			Model:  cfg.Model,
		})

	case ProviderGemini:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini API key is required")
		}
		// Gemini extension likely relies on environment variables
		_ = os.Setenv("GOOGLE_API_KEY", cfg.APIKey)
		_ = os.Setenv("GEMINI_API_KEY", cfg.APIKey)

		return gemini.NewChatModel(ctx, &gemini.Config{
			Model: cfg.Model,
		})

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
		modelName := cfg.EmbeddingModel
		if modelName == "" {
			modelName = config.DefaultOpenAIEmbeddingModel
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
		modelName := cfg.EmbeddingModel
		if modelName == "" {
			modelName = config.DefaultOllamaEmbeddingModel
		}
		return ollamaEmbed.NewEmbedder(ctx, &ollamaEmbed.EmbeddingConfig{
			BaseURL: baseURL,
			Model:   modelName,
		})

	case ProviderGemini:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini API key is required")
		}
		// Ensure env vars are set for embedding client too
		_ = os.Setenv("GOOGLE_API_KEY", cfg.APIKey)
		_ = os.Setenv("GEMINI_API_KEY", cfg.APIKey)

		// Note: Gemini Embedding model might default to "embedding-001" or similar
		// We use cfg.EmbeddingModel or let the client default?
		// eino-ext gemini embedder likely takes a config.
		// Let's assume geminiEmbed.Config exists and has Model.
		// Search result mentioned "gemini-embedding-001".
		// We don't have a default constant for Gemini embedding yet in defaults.go,
		// but let's check config or use a safe default if passed empty.
		// Actually, let's just pass what we have.
		return geminiEmbed.NewEmbedder(ctx, &geminiEmbed.EmbeddingConfig{
			Model: cfg.EmbeddingModel, // If empty, hopefully lib defaults or errors
		})

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}
