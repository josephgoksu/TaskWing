package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestDefaultRetrievalConfig(t *testing.T) {
	cfg := DefaultRetrievalConfig()

	// Test default weights
	assert.Equal(t, 0.40, cfg.FTSWeight, "FTS weight should be 0.40")
	assert.Equal(t, 0.60, cfg.VectorWeight, "Vector weight should be 0.60")

	// Weights should sum to 1.0
	assert.Equal(t, 1.0, cfg.FTSWeight+cfg.VectorWeight, "Weights should sum to 1.0")

	// Test default thresholds
	assert.Equal(t, 0.35, cfg.VectorScoreThreshold, "Vector score threshold should be 0.35")
	assert.Equal(t, 0.12, cfg.MinResultScoreThreshold, "Min result score threshold should be 0.12")

	// Test semantic linking thresholds
	assert.Equal(t, 0.55, cfg.SemanticSimilarityThreshold, "Semantic similarity threshold should be 0.55")
	assert.Equal(t, 0.92, cfg.DeduplicationThreshold, "Deduplication threshold should be 0.92")

	// Test graph expansion defaults
	assert.True(t, cfg.GraphExpansionEnabled, "Graph expansion should be enabled by default")
	assert.Equal(t, 0.8, cfg.GraphExpansionDiscount, "Graph expansion discount should be 0.8")
	assert.Equal(t, 1, cfg.GraphExpansionMaxDepth, "Graph expansion max depth should be 1")
	assert.Equal(t, 0.5, cfg.GraphExpansionMinEdgeConfidence, "Graph expansion min edge confidence should be 0.5")
	assert.Equal(t, 2, cfg.GraphExpansionReservedSlots, "Graph expansion reserved slots should be 2")

	// Test TEI defaults
	assert.Equal(t, "http://localhost:8080", cfg.TEIBaseURL, "TEI base URL should be localhost:8080")
	assert.Equal(t, "Qwen/Qwen3-Embedding-8B", cfg.TEIModelName, "TEI model name should be Qwen3-Embedding-8B")

	// Test reranking defaults
	assert.False(t, cfg.RerankingEnabled, "Reranking should be disabled by default")
	assert.Equal(t, "http://localhost:8081", cfg.RerankBaseURL, "Rerank base URL should be localhost:8081")
	assert.Equal(t, 20, cfg.RerankTopK, "Rerank top K should be 20")
	assert.Equal(t, "Qwen/Qwen3-Reranker-8B", cfg.RerankModelName, "Rerank model should be Qwen3-Reranker-8B")
}

func TestLoadRetrievalConfig_Defaults(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	cfg := LoadRetrievalConfig()

	// Should match defaults when nothing is set
	defaults := DefaultRetrievalConfig()
	assert.Equal(t, defaults.FTSWeight, cfg.FTSWeight)
	assert.Equal(t, defaults.VectorWeight, cfg.VectorWeight)
	assert.Equal(t, defaults.TEIBaseURL, cfg.TEIBaseURL)
}

func TestLoadRetrievalConfig_CustomValues(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	// Set custom values
	viper.Set("retrieval.weights.fts", 0.50)
	viper.Set("retrieval.weights.vector", 0.50)
	viper.Set("retrieval.thresholds.vector_score", 0.40)
	viper.Set("retrieval.tei.base_url", "http://custom:9090")
	viper.Set("retrieval.tei.model_name", "custom-model")
	viper.Set("retrieval.reranking.enabled", true)
	viper.Set("retrieval.graph.enabled", false)

	cfg := LoadRetrievalConfig()

	// Custom values should be used
	assert.Equal(t, 0.50, cfg.FTSWeight)
	assert.Equal(t, 0.50, cfg.VectorWeight)
	assert.Equal(t, 0.40, cfg.VectorScoreThreshold)
	assert.Equal(t, "http://custom:9090", cfg.TEIBaseURL)
	assert.Equal(t, "custom-model", cfg.TEIModelName)
	assert.True(t, cfg.RerankingEnabled)
	assert.False(t, cfg.GraphExpansionEnabled)

	// Clean up
	viper.Reset()
}

func TestLoadRetrievalConfig_PartialOverride(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	// Only override some values
	viper.Set("retrieval.weights.fts", 0.30)
	// Don't set vector weight - should use default

	cfg := LoadRetrievalConfig()
	defaults := DefaultRetrievalConfig()

	// Custom value should be used for FTS
	assert.Equal(t, 0.30, cfg.FTSWeight)

	// Default should be used for Vector (since not set)
	assert.Equal(t, defaults.VectorWeight, cfg.VectorWeight)

	// Clean up
	viper.Reset()
}

func TestRetrievalConfig_WeightsSumToOne(t *testing.T) {
	// This is a property we want to ensure in documentation
	// but don't enforce at runtime (users can set custom values)
	defaults := DefaultRetrievalConfig()
	sum := defaults.FTSWeight + defaults.VectorWeight
	assert.InDelta(t, 1.0, sum, 0.001, "Default weights should sum to 1.0")
}
