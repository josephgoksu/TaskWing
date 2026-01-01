/*
Package knowledge - embedding generation for semantic search.
*/
package knowledge

import (
	"context"
	"fmt"
	"math"

	"github.com/josephgoksu/TaskWing/internal/llm"
)

// embeddingModelFactory allows injection for testing
var embeddingModelFactory = llm.NewCloseableEmbedder

// GenerateEmbedding creates a vector embedding for the given text.
// Uses the configuration to determine provider (OpenAI or Ollama).
func GenerateEmbedding(ctx context.Context, text string, cfg llm.Config) ([]float32, error) {
	embedder, err := embeddingModelFactory(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create embedding model: %w", err)
	}
	defer embedder.Close()

	// Eino returns [][]float64
	embeddings64, err := embedder.EmbedStrings(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	if len(embeddings64) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	// Convert []float64 to []float32
	embedding32 := make([]float32, len(embeddings64[0]))
	for i, v := range embeddings64[0] {
		embedding32[i] = float32(v)
	}

	return embedding32, nil
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns a value between -1 and 1, where 1 means identical.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}
