package llm

import (
	"fmt"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/types"
)

// NewProvider is a factory function that returns an instance of an llm.Provider
// based on the provided LLM configuration.
func NewProvider(config *types.LLMConfig) (Provider, error) {
	if config == nil {
		return nil, fmt.Errorf("LLM configuration cannot be nil")
	}

	apiKey := config.APIKey
	provider := strings.ToLower(strings.TrimSpace(config.Provider))

	switch provider {
	case "openai":
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI provider selected but API key is missing")
		}
		// Determine timeout and debug settings
		timeout := time.Duration(60) * time.Second
		if config.RequestTimeoutSeconds > 0 {
			timeout = time.Duration(config.RequestTimeoutSeconds) * time.Second
		}
		debug := config.Debug
		return NewOpenAIProvider(apiKey, timeout, debug), nil
	case "": // No provider specified
		return nil, fmt.Errorf("no LLM provider specified in configuration")
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}
}
