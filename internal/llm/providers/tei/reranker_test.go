package tei

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReranker(t *testing.T) {
	t.Run("requires base URL", func(t *testing.T) {
		_, err := NewReranker(context.Background(), &RerankerConfig{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "base URL is required")
	})

	t.Run("creates reranker with valid config", func(t *testing.T) {
		reranker, err := NewReranker(context.Background(), &RerankerConfig{
			BaseURL: "http://localhost:8081",
			Model:   "reranker-model",
			TopK:    10,
		})
		require.NoError(t, err)
		assert.NotNil(t, reranker)
		assert.Equal(t, "http://localhost:8081", reranker.baseURL)
		assert.Equal(t, 10, reranker.topK)
	})

	t.Run("uses default TopK", func(t *testing.T) {
		reranker, err := NewReranker(context.Background(), &RerankerConfig{
			BaseURL: "http://localhost:8081",
		})
		require.NoError(t, err)
		assert.Equal(t, 5, reranker.topK) // Default
	})
}

func TestReranker_Rerank(t *testing.T) {
	// Mock TEI rerank server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rerank", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req RerankRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "What is the capital of France?", req.Query)
		assert.Len(t, req.Texts, 3)

		// Return mock rerank results (TEI returns sorted by score)
		resp := []RerankResponse{
			{Index: 1, Score: 0.95}, // "Paris is the capital of France"
			{Index: 0, Score: 0.72}, // "France is a country in Europe"
			{Index: 2, Score: 0.15}, // "Berlin is in Germany"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	reranker, err := NewReranker(context.Background(), &RerankerConfig{
		BaseURL: server.URL,
		TopK:    5,
	})
	require.NoError(t, err)

	documents := []string{
		"France is a country in Europe",
		"Paris is the capital of France",
		"Berlin is in Germany",
	}

	results, err := reranker.Rerank(context.Background(), "What is the capital of France?", documents)
	require.NoError(t, err)

	// Should be sorted by score (highest first)
	require.Len(t, results, 3)
	assert.Equal(t, 1, results[0].Index)
	assert.Equal(t, 0.95, results[0].Score)
	assert.Equal(t, "Paris is the capital of France", results[0].OriginalText)

	assert.Equal(t, 0, results[1].Index)
	assert.Equal(t, 0.72, results[1].Score)

	assert.Equal(t, 2, results[2].Index)
	assert.Equal(t, 0.15, results[2].Score)
}

func TestReranker_Rerank_TopK(t *testing.T) {
	// Mock server that returns many results
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := []RerankResponse{
			{Index: 0, Score: 0.9},
			{Index: 1, Score: 0.8},
			{Index: 2, Score: 0.7},
			{Index: 3, Score: 0.6},
			{Index: 4, Score: 0.5},
			{Index: 5, Score: 0.4},
			{Index: 6, Score: 0.3},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	reranker, err := NewReranker(context.Background(), &RerankerConfig{
		BaseURL: server.URL,
		TopK:    3, // Only want top 3
	})
	require.NoError(t, err)

	documents := make([]string, 7)
	for i := range documents {
		documents[i] = "doc"
	}

	results, err := reranker.Rerank(context.Background(), "query", documents)
	require.NoError(t, err)

	// Should only return TopK (3) results
	assert.Len(t, results, 3)
	assert.Equal(t, 0.9, results[0].Score)
	assert.Equal(t, 0.8, results[1].Score)
	assert.Equal(t, 0.7, results[2].Score)
}

func TestReranker_Rerank_Empty(t *testing.T) {
	reranker, err := NewReranker(context.Background(), &RerankerConfig{
		BaseURL: "http://localhost:8081",
	})
	require.NoError(t, err)

	results, err := reranker.Rerank(context.Background(), "query", []string{})
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestReranker_Rerank_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	reranker, err := NewReranker(context.Background(), &RerankerConfig{
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	_, err = reranker.Rerank(context.Background(), "query", []string{"doc1", "doc2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 503")
}

func TestReranker_RerankWithScores(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := []RerankResponse{
			{Index: 0, Score: 0.9},
			{Index: 2, Score: 0.7},
			{Index: 1, Score: 0.5},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	reranker, err := NewReranker(context.Background(), &RerankerConfig{
		BaseURL: server.URL,
		TopK:    10,
	})
	require.NoError(t, err)

	scores, err := reranker.RerankWithScores(context.Background(), "query", []string{"a", "b", "c"})
	require.NoError(t, err)

	// Scores should be aligned with original indices
	assert.Len(t, scores, 3)
	assert.Equal(t, 0.9, scores[0]) // Index 0
	assert.Equal(t, 0.5, scores[1]) // Index 1
	assert.Equal(t, 0.7, scores[2]) // Index 2
}

func TestReranker_Close(t *testing.T) {
	reranker, err := NewReranker(context.Background(), &RerankerConfig{
		BaseURL: "http://localhost:8081",
	})
	require.NoError(t, err)

	// Close should not error
	assert.NoError(t, reranker.Close())
}
