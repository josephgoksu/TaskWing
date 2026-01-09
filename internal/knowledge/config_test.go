package knowledge

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoadRetrievalConfig_ReturnsValidConfig(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	cfg := LoadRetrievalConfig()

	// Should have sensible defaults
	assert.Greater(t, cfg.FTSWeight, 0.0, "FTS weight should be positive")
	assert.Greater(t, cfg.VectorWeight, 0.0, "Vector weight should be positive")
	assert.Greater(t, cfg.VectorScoreThreshold, 0.0, "Vector threshold should be positive")
	assert.NotEmpty(t, cfg.TEIBaseURL, "TEI base URL should not be empty")
}

func TestDefaultRetrievalConfig_HasNewDefaults(t *testing.T) {
	cfg := DefaultRetrievalConfig()

	// New defaults: 40% FTS, 60% Vector
	assert.Equal(t, 0.40, cfg.FTSWeight, "FTS weight should be 0.40 (new default)")
	assert.Equal(t, 0.60, cfg.VectorWeight, "Vector weight should be 0.60 (new default)")

	// New threshold defaults
	assert.Equal(t, 0.35, cfg.VectorScoreThreshold, "Vector threshold should be 0.35 (new default)")
	assert.Equal(t, 0.12, cfg.MinResultScoreThreshold, "Min result threshold should be 0.12 (new default)")
}

func TestLoadRetrievalConfig_ViperOverrides(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	// Set custom weights via Viper
	viper.Set("retrieval.weights.fts", 0.70)
	viper.Set("retrieval.weights.vector", 0.30)

	cfg := LoadRetrievalConfig()

	// Custom values should be used
	assert.Equal(t, 0.70, cfg.FTSWeight, "Should use Viper override for FTS weight")
	assert.Equal(t, 0.30, cfg.VectorWeight, "Should use Viper override for Vector weight")

	// Clean up
	viper.Reset()
}

func TestRetrievalConfig_TEISettings(t *testing.T) {
	cfg := DefaultRetrievalConfig()

	// TEI defaults
	assert.Equal(t, "http://localhost:8080", cfg.TEIBaseURL)
	assert.Equal(t, "Qwen/Qwen3-Embedding-8B", cfg.TEIModelName)

	// Reranking defaults
	assert.False(t, cfg.RerankingEnabled, "Reranking should be off by default")
	assert.Equal(t, "Qwen/Qwen3-Reranker-8B", cfg.RerankModelName)
	assert.Equal(t, 20, cfg.RerankTopK)
}
