package cmd

import (
	"fmt"
	"os"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/types"
)

// validateAndGuideLLMConfig ensures essential provider settings are present and helpful errors are returned
func validateAndGuideLLMConfig(cfg *types.LLMConfig) error {
	if cfg == nil {
		return fmt.Errorf("LLM config missing")
	}
	if cfg.Provider == "" {
		return fmt.Errorf("llm.provider not set; set in .taskwing.yaml or env TASKWING_LLM_PROVIDER")
	}
	if cfg.ModelName == "" {
		return fmt.Errorf("llm.modelName not set; set in .taskwing.yaml or env TASKWING_LLM_MODELNAME")
	}
	switch cfg.Provider {
	case "openai":
		if cfg.APIKey == "" {
			if v := os.Getenv("OPENAI_API_KEY"); v != "" {
				cfg.APIKey = v
			}
			if cfg.APIKey == "" {
				return fmt.Errorf("OPENAI_API_KEY not set; export OPENAI_API_KEY or TASKWING_LLM_APIKEY")
			}
		}
	default:
		// Keep strict for now; only openai supported per repo defaults
		return fmt.Errorf("unsupported llm.provider: %s (supported: openai)", cfg.Provider)
	}
	// Provide sane defaults if absent
	if cfg.RequestTimeoutSeconds <= 0 {
		cfg.RequestTimeoutSeconds = 120
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 1
	}
	return nil
}

// createLLMProvider creates an LLM provider with proper configuration validation and environment variable resolution
func createLLMProvider(cfg *types.LLMConfig) (llm.Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("LLM config missing")
	}

	// Create a copy to avoid modifying the original config
	resolvedConfig := *cfg

	// Resolve API key from environment if needed
	if resolvedConfig.APIKey == "" && resolvedConfig.Provider == "openai" {
		if apiKeyEnv := os.Getenv("OPENAI_API_KEY"); apiKeyEnv != "" {
			resolvedConfig.APIKey = apiKeyEnv
		} else if apiKeyEnv := os.Getenv(envPrefix + "_LLM_APIKEY"); apiKeyEnv != "" {
			resolvedConfig.APIKey = apiKeyEnv
		}
	}

	// Validate the resolved config
	if err := validateAndGuideLLMConfig(&resolvedConfig); err != nil {
		return nil, err
	}

	// Create the provider using the configurable factory function (for test overrides)
	return newLLMProvider(&resolvedConfig)
}
