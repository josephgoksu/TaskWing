package config

import (
	"github.com/spf13/viper"
)

// RetrievalConfig holds configuration for the hybrid search and TEI integration.
type RetrievalConfig struct {
	// Hybrid search weights (must sum to 1.0)
	FTSWeight    float64 `mapstructure:"fts_weight"`
	VectorWeight float64 `mapstructure:"vector_weight"`

	// Score thresholds
	VectorScoreThreshold    float64 `mapstructure:"vector_score_threshold"`
	MinResultScoreThreshold float64 `mapstructure:"min_result_score_threshold"`

	// Semantic linking thresholds (ingest time)
	SemanticSimilarityThreshold float64 `mapstructure:"semantic_similarity_threshold"`
	DeduplicationThreshold      float64 `mapstructure:"deduplication_threshold"`

	// Graph expansion settings
	GraphExpansionEnabled           bool    `mapstructure:"graph_expansion_enabled"`
	GraphExpansionDiscount          float64 `mapstructure:"graph_expansion_discount"`
	GraphExpansionMaxDepth          int     `mapstructure:"graph_expansion_max_depth"`
	GraphExpansionMinEdgeConfidence float64 `mapstructure:"graph_expansion_min_edge_confidence"`
	GraphExpansionReservedSlots     int     `mapstructure:"graph_expansion_reserved_slots"`

	// Query rewriting settings
	QueryRewriteEnabled bool `mapstructure:"query_rewrite_enabled"`
}

// DefaultRetrievalConfig returns the default retrieval configuration.
// These values are tuned for balanced hybrid search performance.
func DefaultRetrievalConfig() RetrievalConfig {
	return RetrievalConfig{
		// Hybrid search weights - favor semantic search slightly
		FTSWeight:    0.40,
		VectorWeight: 0.60,

		// Score thresholds
		VectorScoreThreshold:    0.35,
		MinResultScoreThreshold: 0.12,

		// Semantic linking thresholds
		SemanticSimilarityThreshold: 0.55,
		DeduplicationThreshold:      0.92,

		// Graph expansion
		GraphExpansionEnabled:           true,
		GraphExpansionDiscount:          0.8,
		GraphExpansionMaxDepth:          1,
		GraphExpansionMinEdgeConfidence: 0.5,
		GraphExpansionReservedSlots:     2,

		// Query rewriting
		QueryRewriteEnabled: true, // Enabled by default - improves search quality
	}
}

// LoadRetrievalConfig loads retrieval configuration from Viper with defaults.
func LoadRetrievalConfig() RetrievalConfig {
	defaults := DefaultRetrievalConfig()

	return RetrievalConfig{
		// Hybrid search weights
		FTSWeight:    getFloat64WithDefault("retrieval.weights.fts", defaults.FTSWeight),
		VectorWeight: getFloat64WithDefault("retrieval.weights.vector", defaults.VectorWeight),

		// Score thresholds
		VectorScoreThreshold:    getFloat64WithDefault("retrieval.thresholds.vector_score", defaults.VectorScoreThreshold),
		MinResultScoreThreshold: getFloat64WithDefault("retrieval.thresholds.min_result_score", defaults.MinResultScoreThreshold),

		// Semantic linking thresholds
		SemanticSimilarityThreshold: getFloat64WithDefault("retrieval.thresholds.semantic_similarity", defaults.SemanticSimilarityThreshold),
		DeduplicationThreshold:      getFloat64WithDefault("retrieval.thresholds.deduplication", defaults.DeduplicationThreshold),

		// Graph expansion
		GraphExpansionEnabled:           getBoolWithDefault("retrieval.graph.enabled", defaults.GraphExpansionEnabled),
		GraphExpansionDiscount:          getFloat64WithDefault("retrieval.graph.discount", defaults.GraphExpansionDiscount),
		GraphExpansionMaxDepth:          getIntWithDefault("retrieval.graph.max_depth", defaults.GraphExpansionMaxDepth),
		GraphExpansionMinEdgeConfidence: getFloat64WithDefault("retrieval.graph.min_edge_confidence", defaults.GraphExpansionMinEdgeConfidence),
		GraphExpansionReservedSlots:     getIntWithDefault("retrieval.graph.reserved_slots", defaults.GraphExpansionReservedSlots),

		// Query rewriting
		QueryRewriteEnabled: getBoolWithDefault("retrieval.query_rewrite.enabled", defaults.QueryRewriteEnabled),
	}
}

// Helper functions for Viper with defaults

func getFloat64WithDefault(key string, defaultVal float64) float64 {
	if viper.IsSet(key) {
		return viper.GetFloat64(key)
	}
	return defaultVal
}

func getIntWithDefault(key string, defaultVal int) int {
	if viper.IsSet(key) {
		return viper.GetInt(key)
	}
	return defaultVal
}

func getBoolWithDefault(key string, defaultVal bool) bool {
	if viper.IsSet(key) {
		return viper.GetBool(key)
	}
	return defaultVal
}
