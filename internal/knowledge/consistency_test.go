package knowledge

import (
	"testing"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepoForConsistency implements Repository interface for consistency tests
type mockRepoForConsistency struct {
	stats *memory.EmbeddingStats
}

func (r *mockRepoForConsistency) ListNodes(typeFilter string) ([]memory.Node, error) { return nil, nil }
func (r *mockRepoForConsistency) GetNode(id string) (*memory.Node, error)            { return nil, nil }
func (r *mockRepoForConsistency) CreateNode(n memory.Node) error                     { return nil }
func (r *mockRepoForConsistency) UpsertNodeBySummary(n memory.Node) error            { return nil }
func (r *mockRepoForConsistency) DeleteNodesByAgent(agent string) error              { return nil }
func (r *mockRepoForConsistency) DeleteNodesByFiles(agent string, filePaths []string) error {
	return nil
}
func (r *mockRepoForConsistency) GetNodesByFiles(agent string, filePaths []string) ([]memory.Node, error) {
	return nil, nil
}
func (r *mockRepoForConsistency) CreateFeature(f memory.Feature) error                  { return nil }
func (r *mockRepoForConsistency) CreatePattern(p memory.Pattern) error                  { return nil }
func (r *mockRepoForConsistency) AddDecision(featureID string, d memory.Decision) error { return nil }
func (r *mockRepoForConsistency) ListFeatures() ([]memory.Feature, error)               { return nil, nil }
func (r *mockRepoForConsistency) GetDecisions(featureID string) ([]memory.Decision, error) {
	return nil, nil
}
func (r *mockRepoForConsistency) Link(fromID, toID string, relType string) error { return nil }
func (r *mockRepoForConsistency) LinkNodes(from, to, relation string, confidence float64, properties map[string]any) error {
	return nil
}
func (r *mockRepoForConsistency) GetNodeEdges(nodeID string) ([]memory.NodeEdge, error) {
	return nil, nil
}
func (r *mockRepoForConsistency) ListNodesWithEmbeddings() ([]memory.Node, error) { return nil, nil }
func (r *mockRepoForConsistency) SearchFTS(query string, limit int) ([]memory.FTSResult, error) {
	return nil, nil
}
func (r *mockRepoForConsistency) GetProjectOverview() (*memory.ProjectOverview, error) {
	return nil, nil
}
func (r *mockRepoForConsistency) GetEmbeddingStats() (*memory.EmbeddingStats, error) {
	return r.stats, nil
}

func TestCheckEmbeddingConsistency_NoIssues(t *testing.T) {
	repo := &mockRepoForConsistency{
		stats: &memory.EmbeddingStats{
			TotalNodes:             10,
			NodesWithEmbeddings:    10,
			NodesWithoutEmbeddings: 0,
			EmbeddingDimension:     768,
			MixedDimensions:        false,
		},
	}

	cfg := llm.Config{}
	svc := NewService(repo, cfg)

	check, err := svc.CheckEmbeddingConsistency()
	require.NoError(t, err)
	assert.Nil(t, check, "Should return nil when no issues")
}

func TestCheckEmbeddingConsistency_MissingEmbeddings(t *testing.T) {
	repo := &mockRepoForConsistency{
		stats: &memory.EmbeddingStats{
			TotalNodes:             10,
			NodesWithEmbeddings:    7,
			NodesWithoutEmbeddings: 3,
			EmbeddingDimension:     768,
			MixedDimensions:        false,
		},
	}

	cfg := llm.Config{}
	svc := NewService(repo, cfg)

	check, err := svc.CheckEmbeddingConsistency()
	require.NoError(t, err)
	require.NotNil(t, check)
	assert.True(t, check.NeedsAttention)
	assert.Contains(t, check.Message, "3 nodes missing embeddings")
}

func TestCheckEmbeddingConsistency_MixedDimensions(t *testing.T) {
	repo := &mockRepoForConsistency{
		stats: &memory.EmbeddingStats{
			TotalNodes:             10,
			NodesWithEmbeddings:    10,
			NodesWithoutEmbeddings: 0,
			EmbeddingDimension:     768,
			MixedDimensions:        true,
		},
	}

	cfg := llm.Config{}
	svc := NewService(repo, cfg)

	check, err := svc.CheckEmbeddingConsistency()
	require.NoError(t, err)
	require.NotNil(t, check)
	assert.True(t, check.NeedsAttention)
	assert.Contains(t, check.Message, "mixed embedding dimensions")
}

func TestCheckEmbeddingConsistency_EmptyDatabase(t *testing.T) {
	repo := &mockRepoForConsistency{
		stats: &memory.EmbeddingStats{
			TotalNodes: 0,
		},
	}

	cfg := llm.Config{}
	svc := NewService(repo, cfg)

	check, err := svc.CheckEmbeddingConsistency()
	require.NoError(t, err)
	assert.Nil(t, check, "Should return nil for empty database")
}

func TestCheckEmbeddingConsistency_BothIssues(t *testing.T) {
	repo := &mockRepoForConsistency{
		stats: &memory.EmbeddingStats{
			TotalNodes:             10,
			NodesWithEmbeddings:    8,
			NodesWithoutEmbeddings: 2,
			EmbeddingDimension:     768,
			MixedDimensions:        true,
		},
	}

	cfg := llm.Config{}
	svc := NewService(repo, cfg)

	check, err := svc.CheckEmbeddingConsistency()
	require.NoError(t, err)
	require.NotNil(t, check)
	assert.True(t, check.NeedsAttention)
	assert.Contains(t, check.Message, "mixed embedding dimensions")
	assert.Contains(t, check.Message, "2 nodes missing embeddings")
}
