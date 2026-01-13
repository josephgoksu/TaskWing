// TEI (Text Embeddings Inference) embedder client.
// TEI is a high-performance embedding server that supports OpenAI-compatible APIs.
// See: https://github.com/huggingface/text-embeddings-inference
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

// TeiConfig holds configuration for the TEI embedder.
type TeiConfig struct {
	// BaseURL is the TEI server URL (e.g., "http://localhost:8080")
	BaseURL string

	// Model is the model name (optional, TEI typically uses single model)
	Model string

	// Timeout for HTTP requests (default: 30s)
	Timeout time.Duration
}

// TeiEmbedder implements the eino embedding.Embedder interface for TEI servers.
// It uses the OpenAI-compatible /v1/embeddings endpoint.
type TeiEmbedder struct {
	baseURL string
	model   string
	client  *http.Client
}

// teiEmbeddingRequest is the request payload for /v1/embeddings
type teiEmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model,omitempty"`
}

// teiEmbeddingResponse is the response from /v1/embeddings
type teiEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// teiNativeEmbedRequest is the native TEI /embed request format
type teiNativeEmbedRequest struct {
	Inputs   []string `json:"inputs"`
	Truncate bool     `json:"truncate,omitempty"`
}

// NewTeiEmbedder creates a new TEI embedder.
func NewTeiEmbedder(ctx context.Context, cfg *TeiConfig) (*TeiEmbedder, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("TEI base URL is required")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &TeiEmbedder{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// EmbedStrings implements the embedding.Embedder interface.
// It sends texts to TEI and returns embeddings as [][]float64.
func (e *TeiEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Try OpenAI-compatible endpoint first
	embeddings, err := e.embedViaOpenAI(ctx, texts)
	if err != nil {
		// Fallback to native TEI endpoint
		embeddings, err = e.embedViaNative(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("TEI embedding failed: %w", err)
		}
	}

	return embeddings, nil
}

// embedViaOpenAI uses the OpenAI-compatible /v1/embeddings endpoint.
func (e *TeiEmbedder) embedViaOpenAI(ctx context.Context, texts []string) ([][]float64, error) {
	reqBody := teiEmbeddingRequest{
		Input: texts,
		Model: e.model,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := e.baseURL + "/v1/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TEI returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp teiEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract embeddings in order
	embeddings := make([][]float64, len(texts))
	for _, d := range embResp.Data {
		if d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}

	return embeddings, nil
}

// embedViaNative uses the native TEI /embed endpoint.
func (e *TeiEmbedder) embedViaNative(ctx context.Context, texts []string) ([][]float64, error) {
	reqBody := teiNativeEmbedRequest{
		Inputs:   texts,
		Truncate: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := e.baseURL + "/embed"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TEI returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Native TEI returns [][]float64 directly
	var embeddings [][]float64
	if err := json.NewDecoder(resp.Body).Decode(&embeddings); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return embeddings, nil
}

// GetDimensions returns the embedding dimension by making a test request.
// This is useful for validating compatibility with stored embeddings.
func (e *TeiEmbedder) GetDimensions(ctx context.Context) (int, error) {
	embeddings, err := e.EmbedStrings(ctx, []string{"test"})
	if err != nil {
		return 0, fmt.Errorf("test embedding: %w", err)
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return 0, fmt.Errorf("empty embedding returned")
	}
	return len(embeddings[0]), nil
}

// Close releases any resources held by the embedder.
func (e *TeiEmbedder) Close() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}

// Verify interface compliance at compile time
var _ embedding.Embedder = (*TeiEmbedder)(nil)
