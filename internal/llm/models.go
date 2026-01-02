package llm

import (
	"fmt"
	"sort"
)

// Model represents a complete model definition including metadata and pricing.
// This is the single source of truth for all model information.
type Model struct {
	ID               string   // Canonical model ID (e.g., "gpt-5-mini")
	Provider         string   // Provider display name (e.g., "OpenAI")
	ProviderID       string   // Internal provider ID (e.g., "openai")
	Aliases          []string // Alternative IDs including dated versions (e.g., "gpt-5-mini-2025-08-07")
	InputPer1M       float64  // $ per 1M input tokens
	OutputPer1M      float64  // $ per 1M output tokens
	IsDefault        bool     // Whether this is the default model for its provider
	SupportsThinking bool     // Whether the model supports extended thinking mode
}

// ModelRegistry is the single source of truth for all supported models.
// Add new models here - everything else derives from this registry.
// Prices last updated: 2025-12
var ModelRegistry = []Model{
	// ============================================
	// OpenAI Models
	// ============================================
	{
		ID:          "gpt-5-mini",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		Aliases:     []string{"gpt-5-mini-2025-08-07"},
		InputPer1M:  0.22,
		OutputPer1M: 1.80,
		IsDefault:   true,
	},
	{
		ID:          "gpt-5.1",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		InputPer1M:  1.10,
		OutputPer1M: 9.00,
	},
	{
		ID:          "gpt-5-nano",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		Aliases:     []string{"gpt-5-nano-2025-09-25"},
		InputPer1M:  0.04,
		OutputPer1M: 0.36,
	},
	{
		ID:          "gpt-4.1-mini",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		Aliases:     []string{"gpt-4.1-mini-2025-04-14"},
		InputPer1M:  0.15,
		OutputPer1M: 0.60,
	},
	{
		ID:          "gpt-4o",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		Aliases:     []string{"gpt-4o-2024-08-06"},
		InputPer1M:  2.50,
		OutputPer1M: 10.00,
	},
	{
		ID:          "gpt-4o-mini",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		Aliases:     []string{"gpt-4o-mini-2024-07-18"},
		InputPer1M:  0.15,
		OutputPer1M: 0.60,
	},
	{
		ID:          "gpt-4-turbo",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		InputPer1M:  10.00,
		OutputPer1M: 30.00,
	},
	{
		ID:          "gpt-3.5-turbo",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		InputPer1M:  0.50,
		OutputPer1M: 1.50,
	},

	// ============================================
	// Anthropic Models
	// ============================================
	{
		ID:               "claude-3-5-sonnet-latest",
		Provider:         "Anthropic",
		ProviderID:       ProviderAnthropic,
		Aliases:          []string{"claude-3-5-sonnet-20241022"},
		InputPer1M:       3.00,
		OutputPer1M:      15.00,
		IsDefault:        true,
		SupportsThinking: true,
	},
	{
		ID:               "claude-3-5-haiku-latest",
		Provider:         "Anthropic",
		ProviderID:       ProviderAnthropic,
		Aliases:          []string{"claude-3-5-haiku-20241022"},
		InputPer1M:       0.80,
		OutputPer1M:      4.00,
		SupportsThinking: true,
	},
	{
		ID:               "claude-3-opus-latest",
		Provider:         "Anthropic",
		ProviderID:       ProviderAnthropic,
		Aliases:          []string{"claude-3-opus-20240229"},
		InputPer1M:       15.00,
		OutputPer1M:      75.00,
		SupportsThinking: true,
	},
	{
		ID:          "claude-3-sonnet-20240229",
		Provider:    "Anthropic",
		ProviderID:  ProviderAnthropic,
		InputPer1M:  3.00,
		OutputPer1M: 15.00,
		// Note: Extended thinking requires Claude 3.5+ models
	},
	{
		ID:          "claude-3-haiku-20240307",
		Provider:    "Anthropic",
		ProviderID:  ProviderAnthropic,
		InputPer1M:  0.25,
		OutputPer1M: 1.25,
		// Note: Extended thinking requires Claude 3.5+ models
	},

	// ============================================
	// Google Gemini Models
	// ============================================
	{
		ID:          "gemini-2.0-flash",
		Provider:    "Google",
		ProviderID:  ProviderGemini,
		InputPer1M:  0.10,
		OutputPer1M: 0.40,
		IsDefault:   true,
	},
	{
		ID:               "gemini-3-pro-preview",
		Provider:         "Google",
		ProviderID:       ProviderGemini,
		InputPer1M:       2.00,
		OutputPer1M:      12.00,
		SupportsThinking: true,
	},
	{
		ID:               "gemini-3-flash-preview",
		Provider:         "Google",
		ProviderID:       ProviderGemini,
		InputPer1M:       0.50,
		OutputPer1M:      3.00,
		SupportsThinking: true,
	},
	{
		ID:               "gemini-2.5-pro",
		Provider:         "Google",
		ProviderID:       ProviderGemini,
		InputPer1M:       1.25,
		OutputPer1M:      10.00,
		SupportsThinking: true,
	},
	{
		ID:               "gemini-2.5-flash",
		Provider:         "Google",
		ProviderID:       ProviderGemini,
		InputPer1M:       0.30,
		OutputPer1M:      2.50,
		SupportsThinking: true,
	},
	{
		ID:               "gemini-2.5-flash-lite",
		Provider:         "Google",
		ProviderID:       ProviderGemini,
		InputPer1M:       0.10,
		OutputPer1M:      0.40,
		SupportsThinking: true,
	},
	{
		ID:          "gemini-2.0-flash-lite",
		Provider:    "Google",
		ProviderID:  ProviderGemini,
		InputPer1M:  0.075,
		OutputPer1M: 0.30,
	},
	{
		ID:          "gemini-1.5-pro",
		Provider:    "Google",
		ProviderID:  ProviderGemini,
		InputPer1M:  1.25,
		OutputPer1M: 5.00,
	},
	{
		ID:          "gemini-1.5-flash",
		Provider:    "Google",
		ProviderID:  ProviderGemini,
		InputPer1M:  0.075,
		OutputPer1M: 0.30,
	},

	// ============================================
	// Ollama Models (local, no pricing)
	// ============================================
	{
		ID:         "llama3.2",
		Provider:   "Ollama",
		ProviderID: ProviderOllama,
		IsDefault:  true,
	},

	// ============================================
	// Mistral Models (not fully supported yet)
	// ============================================
	{
		ID:          "mistral-large",
		Provider:    "Mistral",
		ProviderID:  ProviderMistral,
		InputPer1M:  4.00,
		OutputPer1M: 12.00,
	},
	{
		ID:          "mistral-medium",
		Provider:    "Mistral",
		ProviderID:  ProviderMistral,
		InputPer1M:  2.70,
		OutputPer1M: 8.10,
	},
	{
		ID:          "mistral-small",
		Provider:    "Mistral",
		ProviderID:  ProviderMistral,
		InputPer1M:  1.00,
		OutputPer1M: 3.00,
	},
}

// modelIndex is built at init time for fast lookups
var modelIndex map[string]*Model

func init() {
	buildModelIndex()
}

func buildModelIndex() {
	modelIndex = make(map[string]*Model)
	for i := range ModelRegistry {
		m := &ModelRegistry[i]
		// Index by canonical ID
		modelIndex[m.ID] = m
		// Index by aliases
		for _, alias := range m.Aliases {
			modelIndex[alias] = m
		}
	}
}

// GetModel returns the model definition for a given model ID or alias.
// Returns nil if the model is not found.
func GetModel(modelID string) *Model {
	return modelIndex[modelID]
}

// GetDefaultModel returns the default model for a provider.
func GetDefaultModel(providerID string) *Model {
	for i := range ModelRegistry {
		m := &ModelRegistry[i]
		if m.ProviderID == providerID && m.IsDefault {
			return m
		}
	}
	return nil
}

// GetDefaultModelID returns the default model ID for a provider.
func GetDefaultModelID(providerID string) string {
	m := GetDefaultModel(providerID)
	if m != nil {
		// Return the dated version for OpenAI (API compatibility)
		if providerID == ProviderOpenAI && len(m.Aliases) > 0 {
			return m.Aliases[0]
		}
		return m.ID
	}
	return ""
}

// InferProvider attempts to determine the provider from a model name.
// Returns the provider ID and true if inference succeeded.
func InferProvider(modelID string) (string, bool) {
	// Check model registry first (most accurate)
	if m := GetModel(modelID); m != nil {
		return m.ProviderID, true
	}

	// Fallback to prefix-based inference for unknown models
	switch {
	case hasPrefix(modelID, "gpt-"), hasPrefix(modelID, "o1-"), hasPrefix(modelID, "o3-"):
		return ProviderOpenAI, true
	case hasPrefix(modelID, "claude-"):
		return ProviderAnthropic, true
	case hasPrefix(modelID, "gemini-"):
		return ProviderGemini, true
	case hasPrefix(modelID, "llama"), hasPrefix(modelID, "mistral"), hasPrefix(modelID, "codellama"), hasPrefix(modelID, "phi"):
		return ProviderOllama, true
	}

	return "", false
}

// ModelOption represents a model choice for selection UI
type ModelOption struct {
	ID          string
	DisplayName string
	PriceInfo   string
	IsDefault   bool
}

// GetModelsForProvider returns available models for a provider (for UI selection).
func GetModelsForProvider(providerID string) []ModelOption {
	var options []ModelOption

	for _, m := range ModelRegistry {
		if m.ProviderID != providerID {
			continue
		}

		options = append(options, ModelOption{
			ID:          m.ID,
			DisplayName: m.ID,
			PriceInfo:   formatPriceInfo(m.InputPer1M, m.OutputPer1M),
			IsDefault:   m.IsDefault,
		})
	}

	// Sort: default first, then alphabetically
	sort.Slice(options, func(i, j int) bool {
		if options[i].IsDefault != options[j].IsDefault {
			return options[i].IsDefault
		}
		return options[i].ID < options[j].ID
	})

	return options
}

func formatPriceInfo(input, output float64) string {
	if input == 0 && output == 0 {
		return "local/free"
	}
	return fmt.Sprintf("$%.2f/$%.2f per 1M tokens", input, output)
}

// CalculateCost calculates cost in USD for token usage.
func CalculateCost(modelID string, inputTokens, outputTokens int) float64 {
	m := GetModel(modelID)
	if m == nil {
		return 0
	}
	inputCost := float64(inputTokens) / 1_000_000 * m.InputPer1M
	outputCost := float64(outputTokens) / 1_000_000 * m.OutputPer1M
	return inputCost + outputCost
}

// ModelSupportsThinking returns true if the model supports extended thinking mode.
func ModelSupportsThinking(modelID string) bool {
	m := GetModel(modelID)
	if m == nil {
		return false
	}
	return m.SupportsThinking
}
