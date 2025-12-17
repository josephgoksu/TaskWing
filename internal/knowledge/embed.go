/*
Package knowledge - embedding generation for semantic search.
*/
package knowledge

import (
	"context"
	"fmt"
	"math"

	"github.com/josephgoksu/TaskWing/internal/llm"
	openai "github.com/sashabaranov/go-openai"
)

// GenerateEmbedding creates a vector embedding for the given text.
// Uses OpenAI's text-embedding-3-small model by default.
func GenerateEmbedding(ctx context.Context, text string, cfg llm.Config) ([]float32, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key required for embeddings")
	}

	client := openai.NewClient(cfg.APIKey)

	// Use text-embedding-3-small for cost efficiency
	resp, err := client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return nil, fmt.Errorf("create embedding: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return resp.Data[0].Embedding, nil
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
