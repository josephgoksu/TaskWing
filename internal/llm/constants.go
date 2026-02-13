package llm

// Provider constants
const (
	// DefaultProvider is the default LLM provider
	DefaultProvider = ProviderOpenAI

	// ProviderOpenAI represents the OpenAI provider
	ProviderOpenAI = "openai"

	// ProviderOllama represents the Ollama provider
	ProviderOllama = "ollama"

	// ProviderAnthropic represents the Anthropic provider
	ProviderAnthropic = "anthropic"

	// ProviderGemini represents the Google Gemini provider
	ProviderGemini = "gemini"

	// ProviderBedrock represents AWS Bedrock OpenAI-compatible runtime
	ProviderBedrock = "bedrock"

	// ProviderTEI represents Text Embeddings Inference (embeddings only)
	// TEI is a high-performance embedding server from Hugging Face
	// See: https://github.com/huggingface/text-embeddings-inference
	ProviderTEI = "tei"
)

// DefaultTEIURL is the default URL for TEI server
const DefaultTEIURL = "http://localhost:8080"

// Embedding model constants
const (
	// DefaultOpenAIEmbeddingModel is the default embedding model for OpenAI
	DefaultOpenAIEmbeddingModel = "text-embedding-3-small"

	// DefaultOllamaEmbeddingModel is the default embedding model for Ollama
	DefaultOllamaEmbeddingModel = "nomic-embed-text"

	// DefaultBedrockEmbeddingModel is the default embedding model for AWS Bedrock
	DefaultBedrockEmbeddingModel = "amazon.titan-embed-text-v2:0"
)

// DefaultOllamaURL is the default URL for Ollama server
const DefaultOllamaURL = "http://localhost:11434"

// DefaultModelForProvider returns the default model ID for a given provider.
// This is a convenience wrapper around GetDefaultModelID in models.go.
func DefaultModelForProvider(provider string) string {
	return GetDefaultModelID(provider)
}

// InferProviderFromModel attempts to determine the provider from a model name.
// This is a convenience wrapper around InferProvider in models.go.
func InferProviderFromModel(model string) (string, bool) {
	return InferProvider(model)
}
