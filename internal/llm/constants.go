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

	// ProviderTaskWing represents the TaskWing managed inference service.
	// Uses fine-tuned models optimized for architecture extraction.
	// OpenAI-compatible API; requires TASKWING_API_KEY.
	ProviderTaskWing = "taskwing"
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

// DefaultTaskWingURL is the default base URL for the TaskWing managed inference service.
// Served via RunPod Serverless vLLM (OpenAI-compatible).
// Override per-project via llm.taskwing.base_url in .taskwing.yaml.
const DefaultTaskWingURL = "https://api.runpod.ai/v2/taskwing-brain/openai/v1"

// TaskWing Brain model constants (fine-tuned for architecture extraction)
const (
	// ModelTaskWingBrain is the primary fine-tuned model (Qwen2.5-Coder-7B based)
	ModelTaskWingBrain = "taskwing-brain"

	// ModelTaskWingBrainLite is the lightweight variant (Phi-4-Mini based)
	ModelTaskWingBrainLite = "taskwing-brain-lite"
)

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
