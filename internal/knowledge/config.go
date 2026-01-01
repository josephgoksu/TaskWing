// Package knowledge provides search configuration constants.
// All search tuning parameters are centralized here for maintainability.
package knowledge

// Search algorithm weights and thresholds.
// These values control hybrid FTS5 + vector search behavior.
const (
	// FTSWeight is the weight given to FTS5 keyword match scores in hybrid search.
	// Higher values prioritize exact keyword matches over semantic similarity.
	// Range: 0.0 to 1.0, must sum with VectorWeight to 1.0
	FTSWeight = 0.65

	// VectorWeight is the weight given to vector similarity scores in hybrid search.
	// Higher values prioritize semantic similarity over exact keyword matches.
	// Range: 0.0 to 1.0, must sum with FTSWeight to 1.0
	VectorWeight = 0.35

	// VectorScoreThreshold filters out low-confidence vector similarity results.
	// Results with vector scores below this threshold are excluded entirely.
	// Range: 0.0 to 1.0, higher = stricter filtering, fewer but more relevant results.
	VectorScoreThreshold = 0.25

	// MinResultScoreThreshold filters the final merged results.
	// Results with combined scores below this threshold are excluded from output.
	// This prevents noise in top-N results. Lower = more results, higher = stricter.
	MinResultScoreThreshold = 0.08

	// SemanticSimilarityThreshold is used during ingest to create "semantically_similar" edges.
	// Only node pairs with cosine similarity >= this value are linked.
	// 0.55 provides a good balance between precision and recall for knowledge graph connections.
	SemanticSimilarityThreshold = 0.55

	// DeduplicationThreshold is used to detect near-duplicate nodes during ingest.
	// Nodes with content similarity >= this value are considered duplicates and merged.
	DeduplicationThreshold = 0.92

	// Edge Weights for Knowledge Graph (reserved for Phase 2: evidence-based linking)
	EdgeWeightDependsOn = 0.9 // Nodes sharing code evidence
	EdgeWeightRelatesTo = 0.7 // General relationship

	// Graph Expansion Settings (for graph-enhanced search)
	// When enabled, search results are expanded by traversing knowledge graph edges.
	GraphExpansionEnabled = true

	// GraphExpansionDiscount is the multiplier applied to connected nodes' scores.
	// Connected nodes get parent_score * edge_confidence * this discount.
	// Range: 0.0 to 1.0. Higher = more weight to connected nodes.
	GraphExpansionDiscount = 0.8

	// GraphExpansionMaxDepth controls how many hops to traverse.
	// 1 = only direct connections, 2 = friends-of-friends, etc.
	GraphExpansionMaxDepth = 1

	// GraphExpansionMinEdgeConfidence filters out weak edges during expansion.
	// Only edges with confidence >= this value are traversed.
	GraphExpansionMinEdgeConfidence = 0.5

	// GraphExpansionReservedSlots is the minimum number of slots to reserve for
	// graph-expanded nodes in the final output. This ensures visibility of graph connections.
	GraphExpansionReservedSlots = 2
)
