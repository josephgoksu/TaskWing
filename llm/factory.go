package llm

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/types"
)

// NewProvider is a factory function that returns an instance of an llm.Provider
// based on the provided LLM configuration.
func NewProvider(config *types.LLMConfig) (Provider, error) {
	if config == nil {
		return nil, fmt.Errorf("LLM configuration cannot be nil")
	}

	apiKey := config.APIKey

	switch config.Provider {
	case "openai":
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI provider selected but API key is missing")
		}
		return NewOpenAIProvider(apiKey), nil
	case "": // No provider specified
		return nil, fmt.Errorf("no LLM provider specified in configuration")
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}
}
