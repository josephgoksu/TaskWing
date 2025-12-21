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
)

// Default model constants for each provider
const (
	// DefaultOpenAIModel is the default model for OpenAI provider
	DefaultOpenAIModel = "gpt-5-mini-2025-08-07"

	// DefaultOllamaModel is the default model for Ollama provider
	DefaultOllamaModel = "llama3.2"
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
	default:
		return ""
	}
}
