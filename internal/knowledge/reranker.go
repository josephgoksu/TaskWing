package knowledge

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/josephgoksu/TaskWing/internal/llm"
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
// Returns nil if reranking is disabled.
type RerankerFactory func(ctx context.Context, cfg RetrievalConfig) (Reranker, error)

// DefaultRerankerFactory creates a TEI reranker if enabled in config.
var DefaultRerankerFactory RerankerFactory = func(ctx context.Context, cfg RetrievalConfig) (Reranker, error) {
	if !cfg.RerankingEnabled {
		return nil, nil
	}

	reranker, err := llm.NewTeiReranker(ctx, &llm.TeiRerankerConfig{
		BaseURL: cfg.RerankBaseURL,
		Model:   cfg.RerankModelName,
		TopK:    cfg.RerankTopK,
		Timeout: 5 * time.Second, // Default timeout for reranking
	})
	if err != nil {
		return nil, err
	}

	return newCircuitBreakerReranker(&teiRerankerAdapter{reranker: reranker}), nil
}

// teiRerankerAdapter adapts llm.TeiReranker to knowledge.Reranker interface.
type teiRerankerAdapter struct {
	reranker *llm.TeiReranker
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

type circuitBreakerReranker struct {
	inner     Reranker
	threshold int
	mu        sync.Mutex
	failures  int
	disabled  bool
}

func newCircuitBreakerReranker(inner Reranker) Reranker {
	return &circuitBreakerReranker{
		inner:     inner,
		threshold: 1,
	}
}

func (c *circuitBreakerReranker) Rerank(ctx context.Context, query string, documents []string) ([]RerankResult, error) {
	c.mu.Lock()
	if c.disabled {
		c.mu.Unlock()
		return nil, ErrRerankerDisabled
	}
	c.mu.Unlock()

	results, err := c.inner.Rerank(ctx, query, documents)
	if err != nil {
		c.mu.Lock()
		c.failures++
		if c.failures >= c.threshold {
			c.disabled = true
		}
		c.mu.Unlock()
		if c.disabled {
			slog.Warn("reranker disabled after failure", "error", err)
			return nil, ErrRerankerDisabled
		}
		return nil, err
	}

	c.mu.Lock()
	c.failures = 0
	c.mu.Unlock()
	return results, nil
}

func (c *circuitBreakerReranker) Close() error {
	return c.inner.Close()
}

// rerankResults applies reranking to scored nodes with timeout and fallback.
// If reranking fails or times out, returns the original results unchanged.
// Preserves reranker ordering and normalizes scores to meaningful display range.
func rerankResults(ctx context.Context, reranker Reranker, query string, scored []ScoredNode, timeout time.Duration) []ScoredNode {
	if reranker == nil || len(scored) == 0 {
		return scored
	}

	// Create timeout context
	rerankCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Extract document contents for reranking
	documents := make([]string, len(scored))
	originalScores := make([]float32, len(scored))
	for i, sn := range scored {
		// Use content + summary for better reranking
		documents[i] = sn.Node.Summary + "\n" + sn.Node.Text()
		originalScores[i] = sn.Score
	}

	// Attempt reranking
	results, err := reranker.Rerank(rerankCtx, query, documents)
	if err != nil {
		if errors.Is(err, ErrRerankerDisabled) {
			return scored
		}
		// Log warning and return original results
		slog.Warn("reranking failed, using original scores",
			"error", err,
			"timeout", timeout,
			"candidates", len(scored))
		return scored
	}

	if len(results) == 0 {
		return scored
	}

	// Find min/max reranker scores for normalization
	minRerankScore := results[0].Score
	maxRerankScore := results[0].Score
	for _, r := range results {
		if r.Score < minRerankScore {
			minRerankScore = r.Score
		}
		if r.Score > maxRerankScore {
			maxRerankScore = r.Score
		}
	}
	rerankRange := maxRerankScore - minRerankScore

	// Find max original score as the ceiling for normalized scores
	var maxOrigScore float32
	for _, s := range originalScores {
		if s > maxOrigScore {
			maxOrigScore = s
		}
	}
	if maxOrigScore < 0.3 {
		maxOrigScore = 0.3 // Minimum ceiling
	}

	// Create reranked results with normalized scores
	// Normalize reranker scores to [0.15, maxOrigScore] range
	// This preserves relative differences while ensuring meaningful display
	reranked := make([]ScoredNode, 0, len(results))
	for _, r := range results {
		if r.Index < len(scored) {
			node := scored[r.Index]

			// Normalize reranker score to 0-1 range, then scale to display range
			var normalizedScore float32
			if rerankRange > 0.0001 { // Avoid division by near-zero
				normalizedScore = float32((r.Score - minRerankScore) / rerankRange)
			} else {
				normalizedScore = 1.0 // All scores equal, treat as max
			}

			// Scale to [0.15, maxOrigScore] range
			// This preserves relative differences from the reranker
			displayScore := 0.15 + normalizedScore*(maxOrigScore-0.15)

			node.Score = displayScore
			reranked = append(reranked, node)
		}
	}

	slog.Debug("reranking complete",
		"input_count", len(scored),
		"output_count", len(reranked),
		"rerank_range", rerankRange,
		"max_orig_score", maxOrigScore)

	return reranked
}
