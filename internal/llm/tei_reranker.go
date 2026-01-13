// TEI (Text Embeddings Inference) reranker client.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

// TeiRerankerConfig holds configuration for the TEI reranker.
type TeiRerankerConfig struct {
	// BaseURL is the TEI server URL (e.g., "http://localhost:8081")
	BaseURL string

	// Model is the reranker model name (optional)
	Model string

	// Timeout for HTTP requests (default: 30s)
	Timeout time.Duration

	// TopK is the number of top results to return after reranking
	TopK int
}

// TeiReranker provides reranking functionality using TEI's /rerank endpoint.
type TeiReranker struct {
	baseURL string
	model   string
	topK    int
	client  *http.Client
}

// TeiRerankRequest is the request payload for TEI /rerank endpoint.
type TeiRerankRequest struct {
	Query     string   `json:"query"`
	Texts     []string `json:"texts"`
	RawScores bool     `json:"raw_scores,omitempty"`
	Truncate  bool     `json:"truncate,omitempty"`
}

// TeiRerankResponse is a single rerank result from TEI.
type TeiRerankResponse struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
	Text  string  `json:"text,omitempty"` // Only if return_text=true
}

// TeiRerankResult represents a document with its reranked score.
type TeiRerankResult struct {
	Index        int     // Original index in the input slice
	Score        float64 // Relevance score from reranker
	OriginalText string  // The original text (for reference)
}

// NewTeiReranker creates a new TEI reranker client.
func NewTeiReranker(ctx context.Context, cfg *TeiRerankerConfig) (*TeiReranker, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("TEI base URL is required")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	topK := cfg.TopK
	if topK <= 0 {
		topK = 5 // Default to top 5
	}

	return &TeiReranker{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		topK:    topK,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// Rerank reorders documents by relevance to the query.
// Returns results sorted by score (highest first), limited to TopK.
func (r *TeiReranker) Rerank(ctx context.Context, query string, documents []string) ([]TeiRerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	reqBody := TeiRerankRequest{
		Query:     query,
		Texts:     documents,
		RawScores: false,
		Truncate:  true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := r.baseURL + "/rerank"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TEI rerank returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var rerankResp []TeiRerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&rerankResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Convert to results with original text reference
	results := make([]TeiRerankResult, len(rerankResp))
	for i, rr := range rerankResp {
		originalText := ""
		if rr.Index < len(documents) {
			originalText = documents[rr.Index]
		}
		results[i] = TeiRerankResult{
			Index:        rr.Index,
			Score:        rr.Score,
			OriginalText: originalText,
		}
	}

	// Sort by score (highest first) - TEI may already return sorted, but ensure consistency
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit to TopK
	if len(results) > r.topK {
		results = results[:r.topK]
	}

	return results, nil
}

// RerankWithScores returns just the scores aligned with input document indices.
// Useful when you need to merge scores with existing data structures.
func (r *TeiReranker) RerankWithScores(ctx context.Context, query string, documents []string) ([]float64, error) {
	results, err := r.Rerank(ctx, query, documents)
	if err != nil {
		return nil, err
	}

	// Create score array aligned with original indices
	scores := make([]float64, len(documents))
	for _, res := range results {
		if res.Index < len(scores) {
			scores[res.Index] = res.Score
		}
	}

	return scores, nil
}

// Close releases any resources held by the reranker.
func (r *TeiReranker) Close() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}
