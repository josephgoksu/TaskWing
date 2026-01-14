package knowledge

import (
	"context"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// MockRepository implements Repository for testing
type MockRepository struct {
	nodes     []memory.Node
	nodeByID  map[string]*memory.Node
	ftsResult []memory.FTSResult
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		nodeByID: make(map[string]*memory.Node),
	}
}

func (m *MockRepository) AddNode(n memory.Node) {
	m.nodes = append(m.nodes, n)
	nodeCopy := n
	m.nodeByID[n.ID] = &nodeCopy
}

func (m *MockRepository) SetFTSResults(results []memory.FTSResult) {
	m.ftsResult = results
}

// Repository interface implementations
func (m *MockRepository) ListNodes(_ string) ([]memory.Node, error) {
	return m.nodes, nil
}

func (m *MockRepository) GetNode(id string) (*memory.Node, error) {
	if n, ok := m.nodeByID[id]; ok {
		return n, nil
	}
	return nil, nil
}

func (m *MockRepository) CreateNode(_ *memory.Node) error                    { return nil }
func (m *MockRepository) UpsertNodeBySummary(_ memory.Node) error            { return nil }
func (m *MockRepository) DeleteNodesByAgent(_ string) error                  { return nil }
func (m *MockRepository) DeleteNodesByFiles(_ string, _ []string) error      { return nil }
func (m *MockRepository) GetNodesByFiles(_ string, _ []string) ([]memory.Node, error) {
	return nil, nil
}
func (m *MockRepository) CreateFeature(_ memory.Feature) error                  { return nil }
func (m *MockRepository) CreatePattern(_ memory.Pattern) error                  { return nil }
func (m *MockRepository) AddDecision(_ string, _ memory.Decision) error         { return nil }
func (m *MockRepository) ListFeatures() ([]memory.Feature, error)               { return nil, nil }
func (m *MockRepository) GetDecisions(_ string) ([]memory.Decision, error)      { return nil, nil }
func (m *MockRepository) LinkNodes(_, _, _ string, _ float64, _ map[string]any) error {
	return nil
}
func (m *MockRepository) GetNodeEdges(_ string) ([]memory.NodeEdge, error) { return nil, nil }
func (m *MockRepository) ListNodesWithEmbeddings() ([]memory.Node, error) {
	return m.nodes, nil
}
func (m *MockRepository) SearchFTS(_ string, _ int) ([]memory.FTSResult, error) {
	return m.ftsResult, nil
}
func (m *MockRepository) GetEmbeddingStats() (*memory.EmbeddingStats, error) { return nil, nil }
func (m *MockRepository) GetProjectOverview() (*memory.ProjectOverview, error) {
	return nil, nil
}

func TestDebugRetrieval_ExactIDMatch(t *testing.T) {
	repo := NewMockRepository()

	// Add a task node
	taskNode := memory.Node{
		ID:      "task-abc123",
		Type:    "task",
		Summary: "Test Task",
		Content: "This is a test task",
	}
	repo.AddNode(taskNode)

	svc := NewService(repo, llm.Config{})

	// Test exact ID match
	result, err := svc.SearchDebug(context.Background(), "task-abc123", 10)
	if err != nil {
		t.Fatalf("SearchDebug failed: %v", err)
	}

	if len(result.Results) == 0 {
		t.Fatal("Expected at least one result for exact ID match")
	}

	// First result should be the exact match
	first := result.Results[0]
	if first.ID != "task-abc123" {
		t.Errorf("Expected ID task-abc123, got %s", first.ID)
	}
	if !first.IsExactMatch {
		t.Error("Expected IsExactMatch to be true")
	}
	if first.CombinedScore != 1.0 {
		t.Errorf("Expected CombinedScore 1.0 for exact match, got %f", first.CombinedScore)
	}

	// Check pipeline includes ExactMatch
	hasExactMatch := false
	for _, stage := range result.Pipeline {
		if stage == "ExactMatch" {
			hasExactMatch = true
			break
		}
	}
	if !hasExactMatch {
		t.Error("Expected Pipeline to include ExactMatch")
	}
}

func TestDebugRetrieval_FTSMatch(t *testing.T) {
	repo := NewMockRepository()

	// Add a node
	node := memory.Node{
		ID:      "n-test1",
		Type:    "decision",
		Summary: "Authentication Decision",
		Content: "We use JWT for authentication",
	}
	repo.AddNode(node)

	// Setup FTS results
	repo.SetFTSResults([]memory.FTSResult{
		{Node: node, Rank: -5.0}, // BM25 rank (negative, more negative = better)
	})

	svc := NewService(repo, llm.Config{})

	result, err := svc.SearchDebug(context.Background(), "authentication", 10)
	if err != nil {
		t.Fatalf("SearchDebug failed: %v", err)
	}

	if len(result.Results) == 0 {
		t.Fatal("Expected at least one result")
	}

	first := result.Results[0]
	if first.FTSScore == 0 {
		t.Error("Expected non-zero FTSScore")
	}

	// Check pipeline includes FTS
	hasFTS := false
	for _, stage := range result.Pipeline {
		if stage == "FTS" {
			hasFTS = true
			break
		}
	}
	if !hasFTS {
		t.Error("Expected Pipeline to include FTS")
	}
}

func TestDebugRetrieval_ResponseStructure(t *testing.T) {
	repo := NewMockRepository()
	svc := NewService(repo, llm.Config{})

	result, err := svc.SearchDebug(context.Background(), "test query", 10)
	if err != nil {
		t.Fatalf("SearchDebug failed: %v", err)
	}

	// Verify response structure
	if result.Query != "test query" {
		t.Errorf("Expected Query 'test query', got '%s'", result.Query)
	}

	if result.Timings == nil {
		t.Error("Expected Timings to be initialized")
	}

	// Timings should have entries for each stage
	expectedTimings := []string{"exact_match", "fts", "vector", "rerank", "graph"}
	for _, key := range expectedTimings {
		if _, ok := result.Timings[key]; !ok {
			t.Errorf("Expected Timings to have key '%s'", key)
		}
	}
}

func TestDebugRetrieval_PlanIDMatch(t *testing.T) {
	repo := NewMockRepository()

	// Add a plan node
	planNode := memory.Node{
		ID:      "plan-xyz789",
		Type:    "plan",
		Summary: "Implementation Plan",
		Content: "Plan details here",
	}
	repo.AddNode(planNode)

	svc := NewService(repo, llm.Config{})

	result, err := svc.SearchDebug(context.Background(), "plan-xyz789", 10)
	if err != nil {
		t.Fatalf("SearchDebug failed: %v", err)
	}

	if len(result.Results) == 0 {
		t.Fatal("Expected at least one result for plan ID match")
	}

	first := result.Results[0]
	if first.ID != "plan-xyz789" {
		t.Errorf("Expected ID plan-xyz789, got %s", first.ID)
	}
	if !first.IsExactMatch {
		t.Error("Expected IsExactMatch to be true for plan ID")
	}
}

func TestDebugRetrievalResult_Fields(t *testing.T) {
	// Test that DebugRetrievalResult has all expected fields
	result := DebugRetrievalResult{
		ID:                 "test-id",
		ChunkID:            "test-chunk",
		NodeType:           "decision",
		SourceFilePath:     "/path/to/file.go",
		SourceAgent:        "test-agent",
		Summary:            "Test Summary",
		Content:            "Test Content",
		FTSScore:           0.5,
		VectorScore:        0.7,
		CombinedScore:      0.6,
		RerankScore:        0.8,
		IsExactMatch:       true,
		IsGraphExpanded:    false,
		EmbeddingDimension: 1536,
	}

	if result.ID != "test-id" {
		t.Error("ID field not set correctly")
	}
	if result.VectorScore != 0.7 {
		t.Error("VectorScore field not set correctly")
	}
	if result.EmbeddingDimension != 1536 {
		t.Error("EmbeddingDimension field not set correctly")
	}
}
