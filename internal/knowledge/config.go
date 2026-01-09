// Package knowledge provides search configuration.
// Configuration is loaded dynamically from Viper via internal/config.
// This file provides backward-compatible constants and the RetrievalConfig type alias.
package knowledge

import (
	"github.com/josephgoksu/TaskWing/internal/config"
)

// RetrievalConfig is an alias to the config package's RetrievalConfig.
// This allows knowledge package consumers to use knowledge.RetrievalConfig
// without importing the config package directly.
type RetrievalConfig = config.RetrievalConfig

// LoadRetrievalConfig loads retrieval configuration from Viper with defaults.
// This is a convenience wrapper around config.LoadRetrievalConfig.
func LoadRetrievalConfig() RetrievalConfig {
	return config.LoadRetrievalConfig()
}

// DefaultRetrievalConfig returns the default retrieval configuration.
// This is a convenience wrapper around config.DefaultRetrievalConfig.
func DefaultRetrievalConfig() RetrievalConfig {
	return config.DefaultRetrievalConfig()
}

// Constants for backward compatibility with existing code.
// These match the default values from LoadRetrievalConfig().
// For dynamic configuration (e.g., via .taskwing.yaml), use LoadRetrievalConfig() instead.
const (
	// FTSWeight is the weight for FTS5 keyword matching (default: 0.40).
	FTSWeight = 0.40

	// VectorWeight is the weight for vector similarity (default: 0.60).
	VectorWeight = 0.60

	// VectorScoreThreshold filters low-confidence vector results (default: 0.35).
	VectorScoreThreshold = 0.35

	// MinResultScoreThreshold filters final merged results (default: 0.12).
	MinResultScoreThreshold = 0.12

	// SemanticSimilarityThreshold for creating "semantically_similar" edges (default: 0.55).
	SemanticSimilarityThreshold = 0.55

	// DeduplicationThreshold for detecting near-duplicate nodes (default: 0.92).
	DeduplicationThreshold = 0.92

	// Edge Weights for Knowledge Graph
	EdgeWeightDependsOn = 0.9 // Nodes sharing code evidence
	EdgeWeightRelatesTo = 0.7 // General relationship

	// GraphExpansionEnabled enables graph-enhanced search (default: true).
	GraphExpansionEnabled = true

	// GraphExpansionDiscount is the score multiplier for connected nodes (default: 0.8).
	GraphExpansionDiscount = 0.8

	// GraphExpansionMaxDepth controls traversal depth (default: 1).
	GraphExpansionMaxDepth = 1

	// GraphExpansionMinEdgeConfidence filters weak edges (default: 0.5).
	GraphExpansionMinEdgeConfidence = 0.5

	// GraphExpansionReservedSlots reserves slots for expanded nodes (default: 2).
	GraphExpansionReservedSlots = 2
)
