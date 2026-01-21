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

func (m *MockRepository) CreateNode(_ *memory.Node) error               { return nil }
func (m *MockRepository) UpsertNodeBySummary(_ memory.Node) error       { return nil }
func (m *MockRepository) DeleteNodesByAgent(_ string) error             { return nil }
func (m *MockRepository) DeleteNodesByFiles(_ string, _ []string) error { return nil }
func (m *MockRepository) GetNodesByFiles(_ string, _ []string) ([]memory.Node, error) {
	return nil, nil
}
func (m *MockRepository) CreateFeature(_ memory.Feature) error             { return nil }
func (m *MockRepository) CreatePattern(_ memory.Pattern) error             { return nil }
func (m *MockRepository) AddDecision(_ string, _ memory.Decision) error    { return nil }
func (m *MockRepository) ListFeatures() ([]memory.Feature, error)          { return nil, nil }
func (m *MockRepository) GetDecisions(_ string) ([]memory.Decision, error) { return nil, nil }
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

// Workspace-filtered methods for monorepo support
func (m *MockRepository) ListNodesFiltered(filter memory.NodeFilter) ([]memory.Node, error) {
	if filter.Workspace == "" {
		return m.nodes, nil
	}
	var filtered []memory.Node
	for _, n := range m.nodes {
		if n.Workspace == filter.Workspace {
			filtered = append(filtered, n)
		} else if filter.IncludeRoot && n.Workspace == "root" {
			filtered = append(filtered, n)
		}
	}
	return filtered, nil
}

func (m *MockRepository) ListNodesWithEmbeddingsFiltered(filter memory.NodeFilter) ([]memory.Node, error) {
	return m.ListNodesFiltered(filter)
}

func (m *MockRepository) SearchFTSFiltered(_ string, _ int, filter memory.NodeFilter) ([]memory.FTSResult, error) {
	if filter.Workspace == "" {
		return m.ftsResult, nil
	}
	var filtered []memory.FTSResult
	for _, r := range m.ftsResult {
		if r.Node.Workspace == filter.Workspace {
			filtered = append(filtered, r)
		} else if filter.IncludeRoot && r.Node.Workspace == "root" {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
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

// === Workspace Scoping Tests ===

func TestWorkspaceFiltering_ListNodesFiltered(t *testing.T) {
	repo := NewMockRepository()

	// Add nodes in different workspaces
	repo.AddNode(memory.Node{ID: "n-root-1", Summary: "Root Decision", Workspace: "root"})
	repo.AddNode(memory.Node{ID: "n-root-2", Summary: "Root Pattern", Workspace: "root"})
	repo.AddNode(memory.Node{ID: "n-osprey-1", Summary: "Osprey Decision", Workspace: "osprey"})
	repo.AddNode(memory.Node{ID: "n-studio-1", Summary: "Studio Decision", Workspace: "studio"})

	tests := []struct {
		name        string
		filter      memory.NodeFilter
		wantIDs     []string
		wantCount   int
		description string
	}{
		{
			name:        "empty workspace returns all",
			filter:      memory.NodeFilter{Workspace: ""},
			wantCount:   4,
			description: "Empty workspace should return all nodes",
		},
		{
			name:        "workspace only",
			filter:      memory.NodeFilter{Workspace: "osprey", IncludeRoot: false},
			wantIDs:     []string{"n-osprey-1"},
			wantCount:   1,
			description: "Should return only osprey nodes",
		},
		{
			name:        "workspace plus root",
			filter:      memory.NodeFilter{Workspace: "osprey", IncludeRoot: true},
			wantCount:   3, // osprey + 2 root nodes
			description: "Should return osprey + root nodes",
		},
		{
			name:        "root workspace only",
			filter:      memory.NodeFilter{Workspace: "root", IncludeRoot: false},
			wantIDs:     []string{"n-root-1", "n-root-2"},
			wantCount:   2,
			description: "Should return only root nodes",
		},
		{
			name:        "nonexistent workspace",
			filter:      memory.NodeFilter{Workspace: "nonexistent", IncludeRoot: false},
			wantCount:   0,
			description: "Should return no nodes for nonexistent workspace",
		},
		{
			name:        "nonexistent workspace with root",
			filter:      memory.NodeFilter{Workspace: "nonexistent", IncludeRoot: true},
			wantCount:   2, // Only root nodes
			description: "Should return root nodes even for nonexistent workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := repo.ListNodesFiltered(tt.filter)
			if err != nil {
				t.Fatalf("ListNodesFiltered failed: %v", err)
			}

			if len(nodes) != tt.wantCount {
				t.Errorf("%s: got %d nodes, want %d", tt.description, len(nodes), tt.wantCount)
			}

			if len(tt.wantIDs) > 0 {
				gotIDs := make(map[string]bool)
				for _, n := range nodes {
					gotIDs[n.ID] = true
				}
				for _, wantID := range tt.wantIDs {
					if !gotIDs[wantID] {
						t.Errorf("Expected node %s not found", wantID)
					}
				}
			}
		})
	}
}

func TestWorkspaceFiltering_SearchFTSFiltered(t *testing.T) {
	repo := NewMockRepository()

	// Add nodes for FTS
	rootNode := memory.Node{ID: "n-root", Summary: "Auth Decision", Workspace: "root"}
	ospreyNode := memory.Node{ID: "n-osprey", Summary: "Auth Pattern", Workspace: "osprey"}
	repo.AddNode(rootNode)
	repo.AddNode(ospreyNode)

	// Setup FTS results
	repo.SetFTSResults([]memory.FTSResult{
		{Node: rootNode, Rank: -5.0},
		{Node: ospreyNode, Rank: -4.0},
	})

	// Test: No filter returns all
	results, err := repo.SearchFTSFiltered("auth", 10, memory.NodeFilter{})
	if err != nil {
		t.Fatalf("SearchFTSFiltered failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("No filter: expected 2 results, got %d", len(results))
	}

	// Test: Workspace filter with IncludeRoot=true
	results, err = repo.SearchFTSFiltered("auth", 10, memory.NodeFilter{Workspace: "osprey", IncludeRoot: true})
	if err != nil {
		t.Fatalf("SearchFTSFiltered failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Osprey+root: expected 2 results, got %d", len(results))
	}

	// Test: Workspace filter with IncludeRoot=false
	results, err = repo.SearchFTSFiltered("auth", 10, memory.NodeFilter{Workspace: "osprey", IncludeRoot: false})
	if err != nil {
		t.Fatalf("SearchFTSFiltered failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Osprey only: expected 1 result, got %d", len(results))
	}
	if len(results) > 0 && results[0].Node.ID != "n-osprey" {
		t.Errorf("Expected osprey node, got %s", results[0].Node.ID)
	}
}

func TestNodeFilter_DefaultValues(t *testing.T) {
	filter := memory.DefaultNodeFilter()

	if filter.Type != "" {
		t.Errorf("Type = %q, want empty", filter.Type)
	}
	if filter.Workspace != "" {
		t.Errorf("Workspace = %q, want empty", filter.Workspace)
	}
	if !filter.IncludeRoot {
		t.Error("IncludeRoot = false, want true")
	}
}
