// Package config provides centralized configuration constants for TaskWing.
// All default values should be defined here to ensure a single source of truth.
package config

// LLM provider constants
const (
	// DefaultProvider is the default LLM provider
	DefaultProvider = "openai"

	// ProviderOpenAI represents the OpenAI provider
	ProviderOpenAI = "openai"

	// ProviderOllama represents the Ollama provider
	ProviderOllama = "ollama"

	// ProviderAnthropic represents the Anthropic provider
	ProviderAnthropic = "anthropic"

	// ProviderGemini represents the Google Gemini provider
	ProviderGemini = "gemini"
)

// Default model constants for each provider
const (
	// DefaultOpenAIModel is the default model for OpenAI provider
	DefaultOpenAIModel = "gpt-5-mini-2025-08-07"

	// DefaultOllamaModel is the default model for Ollama provider
	DefaultOllamaModel = "llama3.2"

	// DefaultAnthropicModel is the default model for Anthropic provider
	DefaultAnthropicModel = "claude-3-5-sonnet-latest"

	// DefaultGeminiModel is the default model for Gemini provider
	DefaultGeminiModel = "gemini-2.0-flash"

	// DefaultOpenAIEmbeddingModel is the default embedding model for OpenAI
	DefaultOpenAIEmbeddingModel = "text-embedding-3-small"

	// DefaultOllamaEmbeddingModel is the default embedding model for Ollama
	DefaultOllamaEmbeddingModel = "nomic-embed-text"
)

// DefaultOllamaURL is the default URL for Ollama server
const DefaultOllamaURL = "http://localhost:11434"

// DefaultModelForProvider returns the default model for a given provider string.
func DefaultModelForProvider(provider string) string {
	switch provider {
	case ProviderOpenAI:
		return DefaultOpenAIModel
	case ProviderOllama:
		return DefaultOllamaModel
	case ProviderAnthropic:
		return DefaultAnthropicModel
	case ProviderGemini:
		return DefaultGeminiModel
	default:
		return ""
	}
}
