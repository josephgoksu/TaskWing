// Package llm provides a unified interface for LLM providers using CloudWeGo Eino.
package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// Provider identifies the LLM provider to use.
type Provider string

const (
	ProviderOpenAI Provider = "openai"
	ProviderOllama Provider = "ollama"
)

// Config holds configuration for creating an LLM client.
type Config struct {
	Provider Provider
	Model    string
	APIKey   string // Required for OpenAI
	BaseURL  string // Required for Ollama (default: http://localhost:11434)
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
			baseURL = "http://localhost:11434"
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
