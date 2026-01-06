package llm

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// ModelRole defines the purpose of a model configuration.
// Different roles have different cost/capability tradeoffs.
type ModelRole string

const (
	RoleBootstrap ModelRole = "bootstrap" // Deep analysis (expensive, capable)
	RoleQuery     ModelRole = "query"     // Quick lookups (cheap, fast)
	RoleEmbed     ModelRole = "embed"     // Embeddings
)

// ModelCategory classifies models by their capability/cost tradeoff.
type ModelCategory string

const (
	CategoryReasoning ModelCategory = "reasoning" // o3, opus, gpt-5, 2.5-pro - expensive, most capable
	CategoryBalanced  ModelCategory = "balanced"  // gpt-5-mini, sonnet, flash - good balance
	CategoryFast      ModelCategory = "fast"      // nano, haiku, flash-lite - cheap, fast
)

// Model represents a complete model definition including metadata and pricing.
// This is the single source of truth for all model information.
type Model struct {
	ID               string        // Canonical model ID (e.g., "gpt-5-mini")
	Provider         string        // Provider display name (e.g., "OpenAI")
	ProviderID       string        // Internal provider ID (e.g., "openai")
	Aliases          []string      // Alternative IDs including dated versions (e.g., "gpt-5-mini-2025-08-07")
	InputPer1M       float64       // $ per 1M input tokens
	OutputPer1M      float64       // $ per 1M output tokens
	IsDefault        bool          // Whether this is the default model for its provider
	SupportsThinking bool          // Whether the model supports extended thinking mode
	Category         ModelCategory // Capability category: reasoning, balanced, fast
}

// ModelRegistry is the single source of truth for all supported models.
// Add new models here - everything else derives from this registry.
// Prices last updated: 2025-12 (via web research)
var ModelRegistry = []Model{
	// ============================================
	// OpenAI Models (2025)
	// https://platform.openai.com/docs/models
	// ============================================
	{
		ID:               "o3",
		Provider:         "OpenAI",
		ProviderID:       ProviderOpenAI,
		InputPer1M:       0.40,
		OutputPer1M:      1.60,
		SupportsThinking: true,
		Category:         CategoryReasoning,
	},
	{
		ID:               "o4-mini",
		Provider:         "OpenAI",
		ProviderID:       ProviderOpenAI,
		InputPer1M:       1.10,
		OutputPer1M:      4.40,
		SupportsThinking: true,
		Category:         CategoryReasoning,
	},
	{
		ID:          "gpt-5",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		InputPer1M:  1.25,
		OutputPer1M: 10.00,
		Category:    CategoryReasoning,
	},
	{
		ID:          "gpt-5-mini",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		InputPer1M:  0.25,
		OutputPer1M: 2.00,
		IsDefault:   true,
		Category:    CategoryBalanced,
	},
	{
		ID:          "gpt-5-nano",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		InputPer1M:  0.05,
		OutputPer1M: 0.40,
		Category:    CategoryFast,
	},
	{
		ID:          "gpt-4.1",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		InputPer1M:  2.00,
		OutputPer1M: 8.00,
		Category:    CategoryReasoning,
	},
	{
		ID:          "gpt-4.1-mini",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		InputPer1M:  0.40,
		OutputPer1M: 1.60,
		Category:    CategoryBalanced,
	},
	{
		ID:          "gpt-4.1-nano",
		Provider:    "OpenAI",
		ProviderID:  ProviderOpenAI,
		InputPer1M:  0.10,
		OutputPer1M: 0.40,
		Category:    CategoryFast,
	},

	// ============================================
	// Anthropic Claude 4.x Models (2025)
	// https://docs.anthropic.com/en/docs/about-claude/models
	// ============================================
	{
		ID:               "claude-sonnet-4-5",
		Provider:         "Anthropic",
		ProviderID:       ProviderAnthropic,
		Aliases:          []string{"claude-sonnet-4-5-20250929"},
		InputPer1M:       3.00,
		OutputPer1M:      15.00,
		IsDefault:        true,
		SupportsThinking: true,
		Category:         CategoryBalanced,
	},
	{
		ID:               "claude-opus-4-5",
		Provider:         "Anthropic",
		ProviderID:       ProviderAnthropic,
		Aliases:          []string{"claude-opus-4-5-20251101"},
		InputPer1M:       5.00,
		OutputPer1M:      25.00,
		SupportsThinking: true,
		Category:         CategoryReasoning,
	},
	{
		ID:               "claude-haiku-4-5",
		Provider:         "Anthropic",
		ProviderID:       ProviderAnthropic,
		Aliases:          []string{"claude-haiku-4-5-20251001"},
		InputPer1M:       1.00,
		OutputPer1M:      5.00,
		SupportsThinking: true,
		Category:         CategoryFast,
	},
	{
		ID:               "claude-sonnet-4",
		Provider:         "Anthropic",
		ProviderID:       ProviderAnthropic,
		Aliases:          []string{"claude-sonnet-4-20250514"},
		InputPer1M:       3.00,
		OutputPer1M:      15.00,
		SupportsThinking: true,
		Category:         CategoryBalanced,
	},
	{
		ID:               "claude-opus-4-1",
		Provider:         "Anthropic",
		ProviderID:       ProviderAnthropic,
		Aliases:          []string{"claude-opus-4-1-20250805"},
		InputPer1M:       15.00,
		OutputPer1M:      75.00,
		SupportsThinking: true,
		Category:         CategoryReasoning,
	},
	// Legacy model for compatibility
	{
		ID:          "claude-3-haiku-20240307",
		Provider:    "Anthropic",
		ProviderID:  ProviderAnthropic,
		InputPer1M:  0.25,
		OutputPer1M: 1.25,
		Category:    CategoryFast,
	},

	// ============================================
	// Google Gemini Models (2025)
	// https://ai.google.dev/gemini-api/docs/models
	// Note: Gemini 1.5 retired April 2025
	// ============================================
	{
		ID:               "gemini-3-pro-preview",
		Provider:         "Google",
		ProviderID:       ProviderGemini,
		InputPer1M:       2.00,
		OutputPer1M:      12.00,
		SupportsThinking: true,
		Category:         CategoryReasoning,
	},
	{
		ID:               "gemini-3-flash-preview",
		Provider:         "Google",
		ProviderID:       ProviderGemini,
		InputPer1M:       0.50,
		OutputPer1M:      3.00,
		SupportsThinking: true,
		Category:         CategoryBalanced,
	},
	{
		ID:               "gemini-2.5-pro",
		Provider:         "Google",
		ProviderID:       ProviderGemini,
		InputPer1M:       1.25,
		OutputPer1M:      10.00,
		SupportsThinking: true,
		Category:         CategoryReasoning,
	},
	{
		ID:               "gemini-2.5-flash",
		Provider:         "Google",
		ProviderID:       ProviderGemini,
		InputPer1M:       0.30,
		OutputPer1M:      2.50,
		SupportsThinking: true,
		Category:         CategoryBalanced,
	},
	{
		ID:               "gemini-2.5-flash-lite",
		Provider:         "Google",
		ProviderID:       ProviderGemini,
		InputPer1M:       0.10,
		OutputPer1M:      0.40,
		SupportsThinking: true,
		Category:         CategoryFast,
	},
	{
		ID:          "gemini-2.0-flash",
		Provider:    "Google",
		ProviderID:  ProviderGemini,
		InputPer1M:  0.10,
		OutputPer1M: 0.40,
		IsDefault:   true,
		Category:    CategoryBalanced,
	},
	{
		ID:          "gemini-2.0-flash-lite",
		Provider:    "Google",
		ProviderID:  ProviderGemini,
		InputPer1M:  0.075,
		OutputPer1M: 0.30,
		Category:    CategoryFast,
	},

	// ============================================
	// Ollama Models (local, no pricing)
	// ============================================
	{
		ID:         "llama3.2",
		Provider:   "Ollama",
		ProviderID: ProviderOllama,
		IsDefault:  true,
		Category:   CategoryBalanced,
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
		return m.ID
	}
	return ""
}

// GetRecommendedModelForRole returns the best model for a specific role within a provider.
// For RoleBootstrap, it prefers CategoryReasoning models.
// For RoleQuery, it prefers CategoryFast models.
// Falls back to CategoryBalanced, then provider default if no match found.
func GetRecommendedModelForRole(providerID string, role ModelRole) *Model {
	var targetCategory ModelCategory
	switch role {
	case RoleBootstrap:
		targetCategory = CategoryReasoning
	case RoleQuery:
		targetCategory = CategoryFast
	default:
		targetCategory = CategoryBalanced
	}

	// First pass: find exact category match
	for i := range ModelRegistry {
		m := &ModelRegistry[i]
		if m.ProviderID == providerID && m.Category == targetCategory {
			return m
		}
	}

	// Second pass: fall back to balanced
	if targetCategory != CategoryBalanced {
		for i := range ModelRegistry {
			m := &ModelRegistry[i]
			if m.ProviderID == providerID && m.Category == CategoryBalanced {
				return m
			}
		}
	}

	// Final fallback: provider default
	return GetDefaultModel(providerID)
}

// GetCategoryBadge returns an emoji badge for the model category.
func GetCategoryBadge(category ModelCategory) string {
	switch category {
	case CategoryReasoning:
		return "🧠"
	case CategoryBalanced:
		return "⚡"
	case CategoryFast:
		return "🚀"
	default:
		return ""
	}
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
	case strings.HasPrefix(modelID, "gpt-"),
		strings.HasPrefix(modelID, "o1-"),
		strings.HasPrefix(modelID, "o3"),
		strings.HasPrefix(modelID, "o4-"):
		return ProviderOpenAI, true
	case strings.HasPrefix(modelID, "claude-"):
		return ProviderAnthropic, true
	case strings.HasPrefix(modelID, "gemini-"):
		return ProviderGemini, true
	case strings.HasPrefix(modelID, "llama"), strings.HasPrefix(modelID, "mistral"), strings.HasPrefix(modelID, "codellama"), strings.HasPrefix(modelID, "phi"):
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

// ModelOption represents a model choice for selection UI
type modelWithPrice struct {
	option     ModelOption
	totalPrice float64 // input + output for sorting
}

// GetModelsForProvider returns available models for a provider (for UI selection).
// Models are sorted: default first, then by total price (cheapest to most expensive).
func GetModelsForProvider(providerID string) []ModelOption {
	var models []modelWithPrice

	for _, m := range ModelRegistry {
		if m.ProviderID != providerID {
			continue
		}

		models = append(models, modelWithPrice{
			option: ModelOption{
				ID:          m.ID,
				DisplayName: m.ID,
				PriceInfo:   formatPriceInfo(m.InputPer1M, m.OutputPer1M),
				IsDefault:   m.IsDefault,
			},
			totalPrice: m.InputPer1M + m.OutputPer1M,
		})
	}

	// Sort: default first, then by price descending (most capable/expensive first)
	// Price is a reasonable proxy for capability - users want latest/best models first
	sort.Slice(models, func(i, j int) bool {
		if models[i].option.IsDefault != models[j].option.IsDefault {
			return models[i].option.IsDefault
		}
		return models[i].totalPrice > models[j].totalPrice // descending
	})

	options := make([]ModelOption, len(models))
	for i, m := range models {
		options[i] = m.option
	}
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

// ProviderInfo contains metadata about a provider for UI display.
type ProviderInfo struct {
	ID           string  // Internal provider ID (e.g., "openai")
	DisplayName  string  // Human-readable name (e.g., "OpenAI")
	EnvVar       string  // Environment variable for API key
	DefaultModel string  // Default model ID for this provider
	ModelCount   int     // Number of models available
	IsLocal      bool    // Whether this is a local provider (no API key needed)
	MinPrice     float64 // Minimum total price (input+output) per 1M tokens
	MaxPrice     float64 // Maximum total price (input+output) per 1M tokens
}

// GetProviders returns a list of all available providers with metadata.
// This is the single source of truth for provider information.
func GetProviders() []ProviderInfo {
	providerMap := make(map[string]*ProviderInfo)

	for _, m := range ModelRegistry {
		totalPrice := m.InputPer1M + m.OutputPer1M

		if _, exists := providerMap[m.ProviderID]; !exists {
			providerMap[m.ProviderID] = &ProviderInfo{
				ID:          m.ProviderID,
				DisplayName: m.Provider,
				IsLocal:     m.ProviderID == ProviderOllama,
				MinPrice:    totalPrice,
				MaxPrice:    totalPrice,
			}
		}

		p := providerMap[m.ProviderID]
		p.ModelCount++
		if m.IsDefault {
			p.DefaultModel = m.ID
		}
		if totalPrice > 0 && (p.MinPrice == 0 || totalPrice < p.MinPrice) {
			p.MinPrice = totalPrice
		}
		if totalPrice > p.MaxPrice {
			p.MaxPrice = totalPrice
		}
	}

	var providers []ProviderInfo
	// Return in consistent order
	providerOrder := []string{ProviderOpenAI, ProviderAnthropic, ProviderGemini, ProviderOllama}
	for _, id := range providerOrder {
		if p, exists := providerMap[id]; exists {
			envVar := GetEnvVarForProvider(id)
			if envVar == "" {
				envVar = "(local)"
			}
			p.EnvVar = envVar
			providers = append(providers, *p)
		}
	}

	return providers
}

// providerEnvVars is the SINGLE SOURCE OF TRUTH for provider environment variable names.
// All code needing env var names MUST use GetEnvVarForProvider() or GetEnvValueForProvider().
var providerEnvVars = map[string]string{
	ProviderOpenAI:    "OPENAI_API_KEY",
	ProviderAnthropic: "ANTHROPIC_API_KEY",
	ProviderGemini:    "GEMINI_API_KEY",
	ProviderOllama:    "", // Local, no API key needed
}

// GetEnvVarForProvider returns the environment variable name for a provider's API key.
// Returns empty string for local providers (Ollama) or unknown providers.
func GetEnvVarForProvider(providerID string) string {
	return providerEnvVars[providerID]
}

// GetEnvValueForProvider returns the API key value from environment variables.
// Handles provider-specific fallbacks (e.g., GOOGLE_API_KEY for Gemini).
func GetEnvValueForProvider(providerID string) string {
	envVar := providerEnvVars[providerID]
	if envVar == "" {
		return ""
	}

	value := strings.TrimSpace(os.Getenv(envVar))

	// Gemini fallback: also check GOOGLE_API_KEY
	if value == "" && providerID == ProviderGemini {
		value = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	}

	return value
}
