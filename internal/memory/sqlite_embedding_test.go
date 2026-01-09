package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEmbeddingStats_EmptyDatabase(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tw-embedding-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	db, err := NewSQLiteStore(filepath.Join(tmpDir, "memory.db"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	stats, err := db.GetEmbeddingStats()
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 0, stats.TotalNodes)
	assert.Equal(t, 0, stats.NodesWithEmbeddings)
	assert.Equal(t, 0, stats.NodesWithoutEmbeddings)
	assert.Equal(t, 0, stats.EmbeddingDimension)
	assert.False(t, stats.MixedDimensions)
}

func TestGetEmbeddingStats_WithEmbeddings(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tw-embedding-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	db, err := NewSQLiteStore(filepath.Join(tmpDir, "memory.db"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create nodes with embeddings (768 dimensions)
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}

	node1 := Node{
		ID:        "n-1",
		Content:   "Test content 1",
		Type:      "decision",
		Summary:   "Test node 1",
		Embedding: embedding,
	}
	node2 := Node{
		ID:        "n-2",
		Content:   "Test content 2",
		Type:      "pattern",
		Summary:   "Test node 2",
		Embedding: embedding,
	}

	require.NoError(t, db.CreateNode(node1))
	require.NoError(t, db.CreateNode(node2))

	stats, err := db.GetEmbeddingStats()
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 2, stats.TotalNodes)
	assert.Equal(t, 2, stats.NodesWithEmbeddings)
	assert.Equal(t, 0, stats.NodesWithoutEmbeddings)
	assert.Equal(t, 768, stats.EmbeddingDimension)
	assert.False(t, stats.MixedDimensions)
}

func TestGetEmbeddingStats_WithoutEmbeddings(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tw-embedding-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	db, err := NewSQLiteStore(filepath.Join(tmpDir, "memory.db"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create node without embedding
	node := Node{
		ID:      "n-1",
		Content: "Test content",
		Type:    "decision",
		Summary: "Test node",
		// No embedding
	}
	require.NoError(t, db.CreateNode(node))

	stats, err := db.GetEmbeddingStats()
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 1, stats.TotalNodes)
	assert.Equal(t, 0, stats.NodesWithEmbeddings)
	assert.Equal(t, 1, stats.NodesWithoutEmbeddings)
	assert.Equal(t, 0, stats.EmbeddingDimension) // No embeddings = no dimension
}

func TestGetEmbeddingStats_MixedEmbeddings(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tw-embedding-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	db, err := NewSQLiteStore(filepath.Join(tmpDir, "memory.db"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create node with 768-dim embedding
	embedding768 := make([]float32, 768)
	node1 := Node{
		ID:        "n-1",
		Content:   "Test content 1",
		Type:      "decision",
		Summary:   "Test node 1",
		Embedding: embedding768,
	}
	require.NoError(t, db.CreateNode(node1))

	// Create node with 1024-dim embedding (different dimension!)
	embedding1024 := make([]float32, 1024)
	node2 := Node{
		ID:        "n-2",
		Content:   "Test content 2",
		Type:      "pattern",
		Summary:   "Test node 2",
		Embedding: embedding1024,
	}
	require.NoError(t, db.CreateNode(node2))

	stats, err := db.GetEmbeddingStats()
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 2, stats.TotalNodes)
	assert.Equal(t, 2, stats.NodesWithEmbeddings)
	assert.Equal(t, 0, stats.NodesWithoutEmbeddings)
	assert.True(t, stats.MixedDimensions, "Should detect mixed dimensions")
}

func TestGetEmbeddingStats_MixedWithAndWithout(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tw-embedding-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	db, err := NewSQLiteStore(filepath.Join(tmpDir, "memory.db"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create node with embedding
	embedding := make([]float32, 1536)
	node1 := Node{
		ID:        "n-1",
		Content:   "Test content 1",
		Type:      "decision",
		Summary:   "Test node 1",
		Embedding: embedding,
	}
	require.NoError(t, db.CreateNode(node1))

	// Create node without embedding
	node2 := Node{
		ID:      "n-2",
		Content: "Test content 2",
		Type:    "pattern",
		Summary: "Test node 2",
	}
	require.NoError(t, db.CreateNode(node2))

	stats, err := db.GetEmbeddingStats()
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 2, stats.TotalNodes)
	assert.Equal(t, 1, stats.NodesWithEmbeddings)
	assert.Equal(t, 1, stats.NodesWithoutEmbeddings)
	assert.Equal(t, 1536, stats.EmbeddingDimension)
	assert.False(t, stats.MixedDimensions) // Only one dimension found
}
