package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockReranker implements Reranker for testing
type mockReranker struct {
	results []RerankResult
	err     error
	delay   time.Duration
}

func (m *mockReranker) Rerank(ctx context.Context, query string, documents []string) ([]RerankResult, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *mockReranker) Close() error {
	return nil
}

func TestRerankResults_Success(t *testing.T) {
	// Create test nodes
	nodes := []ScoredNode{
		{Node: &memory.Node{ID: "1", Summary: "First", Content: "First content"}, Score: 0.9},
		{Node: &memory.Node{ID: "2", Summary: "Second", Content: "Second content"}, Score: 0.8},
		{Node: &memory.Node{ID: "3", Summary: "Third", Content: "Third content"}, Score: 0.7},
	}

	// Reranker reverses the order
	reranker := &mockReranker{
		results: []RerankResult{
			{Index: 2, Score: 0.95}, // Third is now best
			{Index: 0, Score: 0.85}, // First is second
			{Index: 1, Score: 0.75}, // Second is third
		},
	}

	result := rerankResults(context.Background(), reranker, "test query", nodes, 5*time.Second)

	require.Len(t, result, 3)
	assert.Equal(t, "3", result[0].Node.ID) // Third is now first
	assert.Equal(t, float32(0.95), result[0].Score)
	assert.Equal(t, "1", result[1].Node.ID)
	assert.Equal(t, "2", result[2].Node.ID)
}

func TestRerankResults_NilReranker(t *testing.T) {
	nodes := []ScoredNode{
		{Node: &memory.Node{ID: "1"}, Score: 0.9},
	}

	// Should return original results when reranker is nil
	result := rerankResults(context.Background(), nil, "query", nodes, 5*time.Second)

	assert.Equal(t, nodes, result)
}

func TestRerankResults_EmptyInput(t *testing.T) {
	reranker := &mockReranker{}

	result := rerankResults(context.Background(), reranker, "query", []ScoredNode{}, 5*time.Second)

	assert.Empty(t, result)
}

func TestRerankResults_Error_Fallback(t *testing.T) {
	nodes := []ScoredNode{
		{Node: &memory.Node{ID: "1", Summary: "First"}, Score: 0.9},
		{Node: &memory.Node{ID: "2", Summary: "Second"}, Score: 0.8},
	}

	reranker := &mockReranker{
		err: errors.New("reranker unavailable"),
	}

	// Should fall back to original results on error
	result := rerankResults(context.Background(), reranker, "query", nodes, 5*time.Second)

	assert.Equal(t, nodes, result) // Original order preserved
}

func TestRerankResults_Timeout_Fallback(t *testing.T) {
	nodes := []ScoredNode{
		{Node: &memory.Node{ID: "1", Summary: "First"}, Score: 0.9},
		{Node: &memory.Node{ID: "2", Summary: "Second"}, Score: 0.8},
	}

	// Reranker takes longer than timeout
	reranker := &mockReranker{
		delay: 2 * time.Second,
		results: []RerankResult{
			{Index: 1, Score: 0.99},
			{Index: 0, Score: 0.50},
		},
	}

	// Short timeout (100ms) should trigger fallback
	result := rerankResults(context.Background(), reranker, "query", nodes, 100*time.Millisecond)

	// Should fall back to original results due to timeout
	assert.Equal(t, nodes, result)
}

func TestRerankResults_PartialResults(t *testing.T) {
	nodes := []ScoredNode{
		{Node: &memory.Node{ID: "1", Summary: "First"}, Score: 0.9},
		{Node: &memory.Node{ID: "2", Summary: "Second"}, Score: 0.8},
		{Node: &memory.Node{ID: "3", Summary: "Third"}, Score: 0.7},
	}

	// Reranker only returns top 2
	reranker := &mockReranker{
		results: []RerankResult{
			{Index: 2, Score: 0.95},
			{Index: 0, Score: 0.85},
		},
	}

	result := rerankResults(context.Background(), reranker, "query", nodes, 5*time.Second)

	// Should only have 2 results (what reranker returned)
	assert.Len(t, result, 2)
	assert.Equal(t, "3", result[0].Node.ID)
	assert.Equal(t, "1", result[1].Node.ID)
}

// TestRetrievalPipeline_WithReranking tests the full two-stage retrieval pipeline
func TestRetrievalPipeline_WithReranking(t *testing.T) {
	// This test verifies the pipeline flow without requiring a real database
	// Config values are used to verify the expected behavior
	cfg := DefaultRetrievalConfig()
	assert.True(t, !cfg.RerankingEnabled || cfg.RerankTopK > 0, "RerankTopK should be set when reranking is enabled")

	// Simulate Stage 1 results (hybrid search)
	stage1Results := []ScoredNode{
		{Node: &memory.Node{ID: "doc1", Summary: "France info", Content: "France is a country"}, Score: 0.85},
		{Node: &memory.Node{ID: "doc2", Summary: "Paris capital", Content: "Paris is the capital of France"}, Score: 0.80},
		{Node: &memory.Node{ID: "doc3", Summary: "Berlin info", Content: "Berlin is in Germany"}, Score: 0.75},
	}

	// Reranker should reorder based on query relevance
	reranker := &mockReranker{
		results: []RerankResult{
			{Index: 1, Score: 0.98}, // Paris capital - most relevant to "capital of France"
			{Index: 0, Score: 0.70}, // France info
			{Index: 2, Score: 0.20}, // Berlin info - not relevant
		},
	}

	result := rerankResults(context.Background(), reranker, "What is the capital of France?", stage1Results, 5*time.Second)

	require.Len(t, result, 3)
	// "Paris capital" should now be first
	assert.Equal(t, "doc2", result[0].Node.ID)
	assert.Equal(t, float32(0.98), result[0].Score)
}

// TestRetrievalPipeline_FallbackOnTimeout simulates the fallback scenario
func TestRetrievalPipeline_FallbackOnTimeout(t *testing.T) {
	stage1Results := []ScoredNode{
		{Node: &memory.Node{ID: "doc1"}, Score: 0.85},
		{Node: &memory.Node{ID: "doc2"}, Score: 0.80},
	}

	// Slow reranker (simulates network timeout)
	reranker := &mockReranker{
		delay: 10 * time.Second,
	}

	start := time.Now()
	result := rerankResults(context.Background(), reranker, "query", stage1Results, 100*time.Millisecond)
	elapsed := time.Since(start)

	// Should timeout quickly (not wait 10s)
	assert.Less(t, elapsed, 500*time.Millisecond)

	// Should return original results
	assert.Equal(t, stage1Results, result)
}
