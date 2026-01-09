package knowledge

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/josephgoksu/TaskWing/internal/llm/providers/tei"
)

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
// Returns nil if reranking is disabled.
type RerankerFactory func(ctx context.Context, cfg RetrievalConfig) (Reranker, error)

// DefaultRerankerFactory creates a TEI reranker if enabled in config.
var DefaultRerankerFactory RerankerFactory = func(ctx context.Context, cfg RetrievalConfig) (Reranker, error) {
	if !cfg.RerankingEnabled {
		return nil, nil
	}

	reranker, err := tei.NewReranker(ctx, &tei.RerankerConfig{
		BaseURL: cfg.RerankBaseURL,
		Model:   cfg.RerankModelName,
		TopK:    cfg.RerankTopK,
		Timeout: 5 * time.Second, // Default timeout for reranking
	})
	if err != nil {
		return nil, err
	}

	return &teiRerankerAdapter{reranker: reranker}, nil
}

// teiRerankerAdapter adapts tei.Reranker to knowledge.Reranker interface.
type teiRerankerAdapter struct {
	reranker *tei.Reranker
}

func (a *teiRerankerAdapter) Rerank(ctx context.Context, query string, documents []string) ([]RerankResult, error) {
	results, err := a.reranker.Rerank(ctx, query, documents)
	if err != nil {
		return nil, err
	}

	// Convert tei.RerankResult to knowledge.RerankResult
	converted := make([]RerankResult, len(results))
	for i, r := range results {
		converted[i] = RerankResult{
			Index: r.Index,
			Score: r.Score,
		}
	}
	return converted, nil
}

func (a *teiRerankerAdapter) Close() error {
	return a.reranker.Close()
}

// rerankResults applies reranking to scored nodes with timeout and fallback.
// If reranking fails or times out, returns the original results unchanged.
func rerankResults(ctx context.Context, reranker Reranker, query string, scored []ScoredNode, timeout time.Duration) []ScoredNode {
	if reranker == nil || len(scored) == 0 {
		return scored
	}

	// Create timeout context
	rerankCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Extract document contents for reranking
	documents := make([]string, len(scored))
	for i, sn := range scored {
		// Use content + summary for better reranking
		documents[i] = sn.Node.Summary + "\n" + sn.Node.Content
	}

	// Attempt reranking
	results, err := reranker.Rerank(rerankCtx, query, documents)
	if err != nil {
		// Log warning and return original results
		slog.Warn("reranking failed, using original scores",
			"error", err,
			"timeout", timeout,
			"candidates", len(scored))
		return scored
	}

	// Create reranked results
	reranked := make([]ScoredNode, 0, len(results))
	for _, r := range results {
		if r.Index < len(scored) {
			node := scored[r.Index]
			// Replace score with reranker score (normalized to 0-1 range if needed)
			node.Score = float32(r.Score)
			reranked = append(reranked, node)
		}
	}

	// Sort by reranker score (should already be sorted, but ensure consistency)
	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})

	slog.Debug("reranking complete",
		"input_count", len(scored),
		"output_count", len(reranked))

	return reranked
}
