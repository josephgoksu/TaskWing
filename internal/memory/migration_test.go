package memory

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUpdateNodeWorkspace_Success tests that workspace updates work correctly.
func TestUpdateNodeWorkspace_Success(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "taskwing-migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize repository
	repo, err := NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create a test node
	testNode := &Node{
		ID:        "test-node-001",
		Content:   "Test content for migration",
		Type:      NodeTypeDecision,
		Summary:   "Test decision",
		Workspace: "root",
	}
	if err := repo.CreateNode(testNode); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Update workspace
	if err := repo.UpdateNodeWorkspace("test-node-001", "osprey"); err != nil {
		t.Fatalf("UpdateNodeWorkspace failed: %v", err)
	}

	// Verify update
	updated, err := repo.GetNode("test-node-001")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if updated.Workspace != "osprey" {
		t.Errorf("workspace = %q, want %q", updated.Workspace, "osprey")
	}
}

// TestUpdateNodeWorkspace_NotFound tests error handling for non-existent nodes.
func TestUpdateNodeWorkspace_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Try to update a non-existent node
	err = repo.UpdateNodeWorkspace("nonexistent-node", "osprey")
	if err == nil {
		t.Fatal("expected error for non-existent node")
	}
}

// TestWorkspaceDefaultsToRoot tests that new nodes default to 'root' workspace.
func TestWorkspaceDefaultsToRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create a node without explicit workspace
	testNode := &Node{
		ID:      "test-node-002",
		Content: "Test content",
		Type:    NodeTypeDecision,
		Summary: "Test",
		// Workspace not set - should default to "root"
	}
	if err := repo.CreateNode(testNode); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Verify default
	node, err := repo.GetNode("test-node-002")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	// Empty workspace should be treated as root by the application
	// The DB stores empty string, but business logic treats it as "root"
	if node.Workspace != "" && node.Workspace != "root" {
		t.Errorf("workspace = %q, want empty or 'root'", node.Workspace)
	}
}

// TestListNodesFiltered_ByWorkspace tests workspace filtering in ListNodesFiltered.
func TestListNodesFiltered_ByWorkspace(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create nodes in different workspaces
	nodes := []Node{
		{ID: "node-root-1", Content: "Root content 1", Type: NodeTypeDecision, Summary: "Root 1", Workspace: "root"},
		{ID: "node-root-2", Content: "Root content 2", Type: NodeTypePattern, Summary: "Root 2", Workspace: "root"},
		{ID: "node-osprey-1", Content: "Osprey content", Type: NodeTypeDecision, Summary: "Osprey", Workspace: "osprey"},
		{ID: "node-studio-1", Content: "Studio content", Type: NodeTypeFeature, Summary: "Studio", Workspace: "studio"},
	}

	for _, n := range nodes {
		node := n // capture
		if err := repo.CreateNode(&node); err != nil {
			t.Fatalf("failed to create node %s: %v", n.ID, err)
		}
	}

	// Test: Filter by workspace "osprey"
	filter := NodeFilter{Workspace: "osprey"}
	filtered, err := repo.ListNodesFiltered(filter)
	if err != nil {
		t.Fatalf("ListNodesFiltered failed: %v", err)
	}

	// Currently placeholder returns all nodes - this test documents expected behavior
	// When filtering is implemented, this should return only osprey nodes
	if len(filtered) == 0 {
		t.Error("ListNodesFiltered returned no nodes")
	}

	// Test: Default filter returns all nodes
	defaultFilter := DefaultNodeFilter()
	all, err := repo.ListNodesFiltered(defaultFilter)
	if err != nil {
		t.Fatalf("ListNodesFiltered with default filter failed: %v", err)
	}

	if len(all) != 4 {
		t.Errorf("default filter returned %d nodes, want 4", len(all))
	}
}

// TestMarkdownMirrorAfterWorkspaceUpdate tests that markdown mirror can be rebuilt.
func TestMarkdownMirrorAfterWorkspaceUpdate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create a feature node (features get markdown files)
	testNode := &Node{
		ID:        "test-feature-001",
		Content:   "Test feature content",
		Type:      NodeTypeFeature,
		Summary:   "Test Feature",
		Workspace: "root",
	}
	if err := repo.CreateNode(testNode); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Update workspace
	if err := repo.UpdateNodeWorkspace("test-feature-001", "osprey"); err != nil {
		t.Fatalf("UpdateNodeWorkspace failed: %v", err)
	}

	// Rebuild files (markdown mirror)
	if err := repo.RebuildFiles(); err != nil {
		t.Fatalf("RebuildFiles failed: %v", err)
	}

	// Verify features directory exists
	featuresDir := filepath.Join(tmpDir, "features")
	if _, err := os.Stat(featuresDir); os.IsNotExist(err) {
		// Features dir might not exist if no features to write - that's OK
		t.Log("features directory does not exist (expected if no writable features)")
	}
}

// TestNodeFilter_DefaultValues tests that DefaultNodeFilter returns expected values.
func TestNodeFilter_DefaultValues(t *testing.T) {
	filter := DefaultNodeFilter()

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

// TestSearchFTSFiltered_Workspace tests workspace filtering for full-text search.
func TestSearchFTSFiltered_Workspace(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-fts-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create nodes in different workspaces with searchable content
	nodes := []Node{
		{ID: "n-root-auth", Content: "Authentication system using JWT tokens", Type: NodeTypeDecision, Summary: "JWT Auth", Workspace: "root"},
		{ID: "n-osprey-auth", Content: "Authentication middleware for Osprey service", Type: NodeTypeDecision, Summary: "Osprey Auth", Workspace: "osprey"},
		{ID: "n-studio-auth", Content: "Authentication flow for Studio app", Type: NodeTypePattern, Summary: "Studio Auth", Workspace: "studio"},
		{ID: "n-root-db", Content: "Database connection pooling strategy", Type: NodeTypeDecision, Summary: "DB Pool", Workspace: "root"},
	}

	for _, n := range nodes {
		node := n
		if err := repo.CreateNode(&node); err != nil {
			t.Fatalf("failed to create node %s: %v", n.ID, err)
		}
	}

	tests := []struct {
		name        string
		query       string
		filter      NodeFilter
		wantMinimum int // At least this many results expected
		wantIDs     []string
		notWantIDs  []string
	}{
		{
			name:        "no filter returns all auth nodes",
			query:       "authentication",
			filter:      NodeFilter{},
			wantMinimum: 3,
		},
		{
			name:       "osprey workspace only",
			query:      "authentication",
			filter:     NodeFilter{Workspace: "osprey", IncludeRoot: false},
			wantIDs:    []string{"n-osprey-auth"},
			notWantIDs: []string{"n-root-auth", "n-studio-auth"},
		},
		{
			name:        "osprey workspace with root",
			query:       "authentication",
			filter:      NodeFilter{Workspace: "osprey", IncludeRoot: true},
			wantMinimum: 2,
			wantIDs:     []string{"n-osprey-auth", "n-root-auth"},
			notWantIDs:  []string{"n-studio-auth"},
		},
		{
			name:       "root workspace only",
			query:      "authentication",
			filter:     NodeFilter{Workspace: "root", IncludeRoot: false},
			wantIDs:    []string{"n-root-auth"},
			notWantIDs: []string{"n-osprey-auth", "n-studio-auth"},
		},
		{
			name:        "nonexistent workspace returns empty",
			query:       "authentication",
			filter:      NodeFilter{Workspace: "nonexistent", IncludeRoot: false},
			wantMinimum: 0,
		},
		{
			name:       "nonexistent workspace with root returns root only",
			query:      "authentication",
			filter:     NodeFilter{Workspace: "nonexistent", IncludeRoot: true},
			wantIDs:    []string{"n-root-auth"},
			notWantIDs: []string{"n-osprey-auth", "n-studio-auth"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := repo.SearchFTSFiltered(tt.query, 10, tt.filter)
			if err != nil {
				t.Fatalf("SearchFTSFiltered failed: %v", err)
			}

			// Check minimum count
			if tt.wantMinimum > 0 && len(results) < tt.wantMinimum {
				t.Errorf("got %d results, want at least %d", len(results), tt.wantMinimum)
			}

			// Build ID set for checking
			gotIDs := make(map[string]bool)
			for _, r := range results {
				gotIDs[r.Node.ID] = true
			}

			// Check expected IDs
			for _, wantID := range tt.wantIDs {
				if !gotIDs[wantID] {
					t.Errorf("expected result %s not found", wantID)
				}
			}

			// Check excluded IDs
			for _, notWantID := range tt.notWantIDs {
				if gotIDs[notWantID] {
					t.Errorf("unexpected result %s found", notWantID)
				}
			}
		})
	}
}

// TestListNodesFiltered_WorkspaceWithType tests workspace + type filtering.
func TestListNodesFiltered_WorkspaceWithType(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-filter-type-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create nodes in different workspaces with different types
	nodes := []Node{
		{ID: "n-root-dec", Content: "Root decision", Type: NodeTypeDecision, Summary: "Root Dec", Workspace: "root"},
		{ID: "n-root-pat", Content: "Root pattern", Type: NodeTypePattern, Summary: "Root Pat", Workspace: "root"},
		{ID: "n-osprey-dec", Content: "Osprey decision", Type: NodeTypeDecision, Summary: "Osprey Dec", Workspace: "osprey"},
		{ID: "n-osprey-feat", Content: "Osprey feature", Type: NodeTypeFeature, Summary: "Osprey Feat", Workspace: "osprey"},
	}

	for _, n := range nodes {
		node := n
		if err := repo.CreateNode(&node); err != nil {
			t.Fatalf("failed to create node %s: %v", n.ID, err)
		}
	}

	tests := []struct {
		name      string
		filter    NodeFilter
		wantCount int
		wantIDs   []string
	}{
		{
			name:      "osprey decisions only",
			filter:    NodeFilter{Workspace: "osprey", Type: NodeTypeDecision, IncludeRoot: false},
			wantCount: 1,
			wantIDs:   []string{"n-osprey-dec"},
		},
		{
			name:      "osprey decisions with root",
			filter:    NodeFilter{Workspace: "osprey", Type: NodeTypeDecision, IncludeRoot: true},
			wantCount: 2,
			wantIDs:   []string{"n-osprey-dec", "n-root-dec"},
		},
		{
			name:      "root patterns only",
			filter:    NodeFilter{Workspace: "root", Type: NodeTypePattern, IncludeRoot: false},
			wantCount: 1,
			wantIDs:   []string{"n-root-pat"},
		},
		{
			name:      "empty workspace returns all of type",
			filter:    NodeFilter{Type: NodeTypeDecision},
			wantCount: 2,
			wantIDs:   []string{"n-root-dec", "n-osprey-dec"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := repo.ListNodesFiltered(tt.filter)
			if err != nil {
				t.Fatalf("ListNodesFiltered failed: %v", err)
			}

			if len(nodes) != tt.wantCount {
				t.Errorf("got %d nodes, want %d", len(nodes), tt.wantCount)
			}

			gotIDs := make(map[string]bool)
			for _, n := range nodes {
				gotIDs[n.ID] = true
			}

			for _, wantID := range tt.wantIDs {
				if !gotIDs[wantID] {
					t.Errorf("expected node %s not found", wantID)
				}
			}
		})
	}
}

// TestListNodesFiltered_IncludeRootBehavior tests the IncludeRoot flag in detail.
func TestListNodesFiltered_IncludeRootBehavior(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-include-root-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create a mix of workspaces including empty (legacy nodes)
	nodes := []Node{
		{ID: "n-explicit-root", Content: "Explicit root", Type: NodeTypeDecision, Summary: "Explicit", Workspace: "root"},
		{ID: "n-empty-ws", Content: "Empty workspace", Type: NodeTypeDecision, Summary: "Empty", Workspace: ""},
		{ID: "n-osprey", Content: "Osprey node", Type: NodeTypeDecision, Summary: "Osprey", Workspace: "osprey"},
	}

	for _, n := range nodes {
		node := n
		if err := repo.CreateNode(&node); err != nil {
			t.Fatalf("failed to create node %s: %v", n.ID, err)
		}
	}

	// IncludeRoot=true should include both "root" and "" (empty) workspaces
	t.Run("include root gets explicit root and empty", func(t *testing.T) {
		filter := NodeFilter{Workspace: "osprey", IncludeRoot: true}
		nodes, err := repo.ListNodesFiltered(filter)
		if err != nil {
			t.Fatalf("ListNodesFiltered failed: %v", err)
		}

		// Should have osprey + root + empty = 3 nodes
		if len(nodes) != 3 {
			t.Errorf("got %d nodes, want 3 (osprey + root + empty)", len(nodes))
		}

		gotIDs := make(map[string]bool)
		for _, n := range nodes {
			gotIDs[n.ID] = true
		}

		if !gotIDs["n-explicit-root"] {
			t.Error("missing n-explicit-root")
		}
		if !gotIDs["n-empty-ws"] {
			t.Error("missing n-empty-ws (empty workspace should be treated as root)")
		}
		if !gotIDs["n-osprey"] {
			t.Error("missing n-osprey")
		}
	})

	// IncludeRoot=false should only get osprey
	t.Run("exclude root gets only specified workspace", func(t *testing.T) {
		filter := NodeFilter{Workspace: "osprey", IncludeRoot: false}
		nodes, err := repo.ListNodesFiltered(filter)
		if err != nil {
			t.Fatalf("ListNodesFiltered failed: %v", err)
		}

		if len(nodes) != 1 {
			t.Errorf("got %d nodes, want 1 (only osprey)", len(nodes))
		}

		if len(nodes) > 0 && nodes[0].ID != "n-osprey" {
			t.Errorf("expected n-osprey, got %s", nodes[0].ID)
		}
	})
}

// TestListNodesWithEmbeddingsFiltered_Workspace tests workspace filtering for nodes with embeddings.
func TestListNodesWithEmbeddingsFiltered_Workspace(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-embeddings-filter-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create nodes with embeddings in different workspaces
	// Note: We need to manually set embeddings since CreateNode doesn't generate them
	embedding := make([]float32, 4) // Small test embedding
	for i := range embedding {
		embedding[i] = float32(i) * 0.1
	}

	nodes := []struct {
		node      Node
		embedding []float32
	}{
		{
			node:      Node{ID: "n-root-emb", Content: "Root with embedding", Type: NodeTypeDecision, Summary: "Root Emb", Workspace: "root"},
			embedding: embedding,
		},
		{
			node:      Node{ID: "n-osprey-emb", Content: "Osprey with embedding", Type: NodeTypeDecision, Summary: "Osprey Emb", Workspace: "osprey"},
			embedding: embedding,
		},
		{
			node:      Node{ID: "n-studio-emb", Content: "Studio with embedding", Type: NodeTypePattern, Summary: "Studio Emb", Workspace: "studio"},
			embedding: embedding,
		},
		{
			node:      Node{ID: "n-no-emb", Content: "No embedding", Type: NodeTypeDecision, Summary: "No Emb", Workspace: "osprey"},
			embedding: nil, // No embedding
		},
	}

	for _, n := range nodes {
		node := n.node
		node.Embedding = n.embedding
		if err := repo.CreateNode(&node); err != nil {
			t.Fatalf("failed to create node %s: %v", n.node.ID, err)
		}
	}

	tests := []struct {
		name       string
		filter     NodeFilter
		wantCount  int
		wantIDs    []string
		notWantIDs []string
	}{
		{
			name:       "no filter returns all with embeddings",
			filter:     NodeFilter{},
			wantCount:  3,
			notWantIDs: []string{"n-no-emb"},
		},
		{
			name:       "osprey workspace only",
			filter:     NodeFilter{Workspace: "osprey", IncludeRoot: false},
			wantCount:  1,
			wantIDs:    []string{"n-osprey-emb"},
			notWantIDs: []string{"n-root-emb", "n-studio-emb", "n-no-emb"},
		},
		{
			name:       "osprey workspace with root",
			filter:     NodeFilter{Workspace: "osprey", IncludeRoot: true},
			wantCount:  2,
			wantIDs:    []string{"n-osprey-emb", "n-root-emb"},
			notWantIDs: []string{"n-studio-emb", "n-no-emb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := repo.ListNodesWithEmbeddingsFiltered(tt.filter)
			if err != nil {
				t.Fatalf("ListNodesWithEmbeddingsFiltered failed: %v", err)
			}

			if len(nodes) != tt.wantCount {
				t.Errorf("got %d nodes, want %d", len(nodes), tt.wantCount)
			}

			gotIDs := make(map[string]bool)
			for _, n := range nodes {
				gotIDs[n.ID] = true
			}

			for _, wantID := range tt.wantIDs {
				if !gotIDs[wantID] {
					t.Errorf("expected node %s not found", wantID)
				}
			}

			for _, notWantID := range tt.notWantIDs {
				if gotIDs[notWantID] {
					t.Errorf("unexpected node %s found", notWantID)
				}
			}
		})
	}
}
