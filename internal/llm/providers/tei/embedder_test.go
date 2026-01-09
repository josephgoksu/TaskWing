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

func TestNewEmbedder(t *testing.T) {
	t.Run("requires base URL", func(t *testing.T) {
		_, err := NewEmbedder(context.Background(), &Config{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "base URL is required")
	})

	t.Run("creates embedder with valid config", func(t *testing.T) {
		embedder, err := NewEmbedder(context.Background(), &Config{
			BaseURL: "http://localhost:8080",
			Model:   "test-model",
		})
		require.NoError(t, err)
		assert.NotNil(t, embedder)
		assert.Equal(t, "http://localhost:8080", embedder.baseURL)
		assert.Equal(t, "test-model", embedder.model)
	})
}

func TestEmbedder_EmbedStrings_OpenAICompatible(t *testing.T) {
	// Mock TEI server with OpenAI-compatible endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/embeddings", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req embeddingRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, []string{"hello world", "test text"}, req.Input)

		// Return mock embeddings
		resp := embeddingResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Object: "embedding", Embedding: []float64{0.1, 0.2, 0.3}, Index: 0},
				{Object: "embedding", Embedding: []float64{0.4, 0.5, 0.6}, Index: 1},
			},
			Model: "test-model",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, err := NewEmbedder(context.Background(), &Config{
		BaseURL: server.URL,
		Model:   "test-model",
	})
	require.NoError(t, err)

	embeddings, err := embedder.EmbedStrings(context.Background(), []string{"hello world", "test text"})
	require.NoError(t, err)
	assert.Len(t, embeddings, 2)
	assert.Equal(t, []float64{0.1, 0.2, 0.3}, embeddings[0])
	assert.Equal(t, []float64{0.4, 0.5, 0.6}, embeddings[1])
}

func TestEmbedder_EmbedStrings_NativeEndpoint(t *testing.T) {
	// Mock TEI server with native /embed endpoint (fallback)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path == "/v1/embeddings" {
			// Simulate OpenAI endpoint not available
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		assert.Equal(t, "/embed", r.URL.Path)

		var req teiEmbedRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, []string{"hello"}, req.Inputs)

		// Return native TEI format: [][]float64
		embeddings := [][]float64{{0.7, 0.8, 0.9}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(embeddings)
	}))
	defer server.Close()

	embedder, err := NewEmbedder(context.Background(), &Config{
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	embeddings, err := embedder.EmbedStrings(context.Background(), []string{"hello"})
	require.NoError(t, err)
	assert.Len(t, embeddings, 1)
	assert.Equal(t, []float64{0.7, 0.8, 0.9}, embeddings[0])
	assert.Equal(t, 2, requestCount) // Should have tried OpenAI first, then native
}

func TestEmbedder_EmbedStrings_Empty(t *testing.T) {
	embedder, err := NewEmbedder(context.Background(), &Config{
		BaseURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	embeddings, err := embedder.EmbedStrings(context.Background(), []string{})
	require.NoError(t, err)
	assert.Nil(t, embeddings)
}

func TestEmbedder_EmbedStrings_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	embedder, err := NewEmbedder(context.Background(), &Config{
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	_, err = embedder.EmbedStrings(context.Background(), []string{"test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestEmbedder_GetDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingResponse{
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: make([]float64, 1024), Index: 0}, // 1024-dim embedding
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, err := NewEmbedder(context.Background(), &Config{
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	dims, err := embedder.GetDimensions(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1024, dims)
}

func TestEmbedder_Close(t *testing.T) {
	embedder, err := NewEmbedder(context.Background(), &Config{
		BaseURL: "http://localhost:8080",
	})
	require.NoError(t, err)

	// Close should not error
	assert.NoError(t, embedder.Close())
}
