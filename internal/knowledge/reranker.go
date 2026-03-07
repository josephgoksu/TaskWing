package knowledge

import (
	"context"
	"errors"
)

// ErrRerankerDisabled is returned when reranking is disabled after repeated failures.
var ErrRerankerDisabled = errors.New("reranker disabled")

// Reranker defines the interface for reranking search results.
type Reranker interface {
	// Rerank reorders documents by relevance to the query.
	// Returns results sorted by score (highest first).
	Rerank(ctx context.Context, query string, documents []string) ([]RerankResult, error)
	Close() error
}

// RerankResult represents a reranked document with its score.
type RerankResult struct {
	Index int     // Original index in the input slice
	Score float64 // Relevance score from reranker
}

// RerankerFactory creates a Reranker from config.
// Returns nil if reranking is not configured.
type RerankerFactory func(ctx context.Context, cfg RetrievalConfig) (Reranker, error)

// DefaultRerankerFactory returns nil (reranking disabled — TEI layer removed).
var DefaultRerankerFactory RerankerFactory = func(_ context.Context, _ RetrievalConfig) (Reranker, error) {
	return nil, nil
}
