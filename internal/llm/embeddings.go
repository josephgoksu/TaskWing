package llm

import (
	"fmt"
	"sort"
)

// EmbeddingModel represents an embedding model definition.
type EmbeddingModel struct {
	ID         string  // Canonical model ID
	Provider   string  // Provider display name
	ProviderID string  // Internal provider ID
	Dimensions int     // Output embedding dimensions
	MaxTokens  int     // Max input tokens
	PricePer1M float64 // $ per 1M tokens (0 for local/free)
	IsDefault  bool    // Default model for this provider
}

// EmbeddingRegistry is the single source of truth for embedding models.
var EmbeddingRegistry = []EmbeddingModel{
	// ============================================
	// OpenAI Embedding Models
	// https://platform.openai.com/docs/guides/embeddings
	// ============================================
	{
		ID:         "text-embedding-3-large",
		Provider:   "OpenAI",
		ProviderID: ProviderOpenAI,
		Dimensions: 3072,
		MaxTokens:  8191,
		PricePer1M: 0.13,
	},
	{
		ID:         "text-embedding-3-small",
		Provider:   "OpenAI",
		ProviderID: ProviderOpenAI,
		Dimensions: 1536,
		MaxTokens:  8191,
		PricePer1M: 0.02,
		IsDefault:  true,
	},

	// ============================================
	// Google Gemini Embedding Models
	// https://ai.google.dev/gemini-api/docs/embeddings
	// ============================================
	{
		ID:         "text-embedding-004",
		Provider:   "Google",
		ProviderID: ProviderGemini,
		Dimensions: 768,
		MaxTokens:  2048,
		PricePer1M: 0.00, // Free in AI Studio
		IsDefault:  true,
	},

	// ============================================
	// Ollama Embedding Models (local)
	// ============================================
	{
		ID:         "nomic-embed-text",
		Provider:   "Ollama",
		ProviderID: ProviderOllama,
		Dimensions: 768,
		MaxTokens:  8192,
		PricePer1M: 0.00,
		IsDefault:  true,
	},
	{
		ID:         "mxbai-embed-large",
		Provider:   "Ollama",
		ProviderID: ProviderOllama,
		Dimensions: 1024,
		MaxTokens:  512,
		PricePer1M: 0.00,
	},
	{
		ID:         "all-minilm",
		Provider:   "Ollama",
		ProviderID: ProviderOllama,
		Dimensions: 384,
		MaxTokens:  256,
		PricePer1M: 0.00,
	},
	{
		ID:         "snowflake-arctic-embed",
		Provider:   "Ollama",
		ProviderID: ProviderOllama,
		Dimensions: 1024,
		MaxTokens:  512,
		PricePer1M: 0.00,
	},

	// ============================================
	// TEI (Text Embeddings Inference) - Custom endpoint
	// ============================================
	{
		ID:         "custom",
		Provider:   "TEI",
		ProviderID: ProviderTEI,
		Dimensions: 0, // Depends on model loaded in TEI
		MaxTokens:  0,
		PricePer1M: 0.00, // Self-hosted
		IsDefault:  true,
	},
}

// embeddingIndex is built at init time for fast lookups
var embeddingIndex map[string]*EmbeddingModel

func init() {
	buildEmbeddingIndex()
}

func buildEmbeddingIndex() {
	embeddingIndex = make(map[string]*EmbeddingModel)
	for i := range EmbeddingRegistry {
		m := &EmbeddingRegistry[i]
		embeddingIndex[m.ID] = m
	}
}

// GetEmbeddingModel returns the embedding model for a given ID.
func GetEmbeddingModel(modelID string) *EmbeddingModel {
	return embeddingIndex[modelID]
}

// GetDefaultEmbeddingModel returns the default embedding model for a provider.
func GetDefaultEmbeddingModel(providerID string) *EmbeddingModel {
	for i := range EmbeddingRegistry {
		m := &EmbeddingRegistry[i]
		if m.ProviderID == providerID && m.IsDefault {
			return m
		}
	}
	return nil
}

// EmbeddingModelOption represents an embedding model choice for selection UI.
type EmbeddingModelOption struct {
	ID          string
	DisplayName string
	Info        string // Dimensions, price info
	IsDefault   bool
}

// GetEmbeddingModelsForProvider returns available embedding models for a provider.
func GetEmbeddingModelsForProvider(providerID string) []EmbeddingModelOption {
	var models []EmbeddingModelOption

	for _, m := range EmbeddingRegistry {
		if m.ProviderID != providerID {
			continue
		}

		info := formatEmbeddingInfo(m)
		models = append(models, EmbeddingModelOption{
			ID:          m.ID,
			DisplayName: m.ID,
			Info:        info,
			IsDefault:   m.IsDefault,
		})
	}

	// Sort: default first, then by name
	sort.Slice(models, func(i, j int) bool {
		if models[i].IsDefault != models[j].IsDefault {
			return models[i].IsDefault
		}
		return models[i].ID < models[j].ID
	})

	return models
}

func formatEmbeddingInfo(m EmbeddingModel) string {
	if m.Dimensions == 0 {
		return "custom endpoint"
	}
	if m.PricePer1M == 0 {
		return fmt.Sprintf("%d dims • free", m.Dimensions)
	}
	return fmt.Sprintf("%d dims • $%.2f/1M tokens", m.Dimensions, m.PricePer1M)
}

// EmbeddingProviderInfo contains metadata about an embedding provider.
type EmbeddingProviderInfo struct {
	ID          string
	DisplayName string
	ModelCount  int
	IsLocal     bool
	IsFree      bool
}

// GetEmbeddingProviders returns all providers that support embeddings.
func GetEmbeddingProviders() []EmbeddingProviderInfo {
	providerMap := make(map[string]*EmbeddingProviderInfo)

	for _, m := range EmbeddingRegistry {
		if _, exists := providerMap[m.ProviderID]; !exists {
			providerMap[m.ProviderID] = &EmbeddingProviderInfo{
				ID:          m.ProviderID,
				DisplayName: m.Provider,
				IsLocal:     m.ProviderID == ProviderOllama || m.ProviderID == ProviderTEI,
				IsFree:      m.PricePer1M == 0,
			}
		}
		providerMap[m.ProviderID].ModelCount++
	}

	// Return in consistent order
	var providers []EmbeddingProviderInfo
	providerOrder := []string{ProviderOllama, ProviderOpenAI, ProviderGemini, ProviderTEI}
	for _, id := range providerOrder {
		if p, exists := providerMap[id]; exists {
			providers = append(providers, *p)
		}
	}

	return providers
}
