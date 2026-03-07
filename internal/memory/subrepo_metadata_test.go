package memory

import (
	"testing"
)

func TestSubrepoMetadataPresent(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	t.Run("nodes_stored_with_workspace_context", func(t *testing.T) {
		// Simulate multi-repo bootstrap: each sub-repo gets its own workspace tag
		subrepos := []struct {
			workspace string
			summary   string
		}{
			{"api", "API Service Architecture"},
			{"api", "API Authentication Pattern"},
			{"web", "Web Frontend React Setup"},
			{"cli", "CLI Go Module Structure"},
		}

		for _, sr := range subrepos {
			node := &Node{
				Summary:   sr.summary,
				Type:      "feature",
				Content:   "Description of " + sr.summary,
				Workspace: sr.workspace,
			}
			if err := store.CreateNode(node); err != nil {
				t.Fatalf("CreateNode %q: %v", sr.summary, err)
			}
		}

		// Query by workspace and verify isolation
		apiNodes, err := store.ListNodesFiltered(NodeFilter{Workspace: "api"})
		if err != nil {
			t.Fatalf("ListNodesFiltered(api): %v", err)
		}
		if len(apiNodes) != 2 {
			t.Errorf("api workspace has %d nodes, want 2", len(apiNodes))
		}

		webNodes, err := store.ListNodesFiltered(NodeFilter{Workspace: "web"})
		if err != nil {
			t.Fatalf("ListNodesFiltered(web): %v", err)
		}
		if len(webNodes) != 1 {
			t.Errorf("web workspace has %d nodes, want 1", len(webNodes))
		}

		cliNodes, err := store.ListNodesFiltered(NodeFilter{Workspace: "cli"})
		if err != nil {
			t.Fatalf("ListNodesFiltered(cli): %v", err)
		}
		if len(cliNodes) != 1 {
			t.Errorf("cli workspace has %d nodes, want 1", len(cliNodes))
		}
	})

	t.Run("root_workspace_used_as_default", func(t *testing.T) {
		node := &Node{
			Summary:   "Global Pattern",
			Type:      "pattern",
			Content:   "A global pattern",
			Workspace: "root",
		}
		if err := store.CreateNode(node); err != nil {
			t.Fatalf("CreateNode: %v", err)
		}

		rootNodes, err := store.ListNodesFiltered(NodeFilter{Workspace: "root"})
		if err != nil {
			t.Fatalf("ListNodesFiltered(root): %v", err)
		}

		found := false
		for _, n := range rootNodes {
			if n.Summary == "Global Pattern" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Root workspace node not found")
		}
	})

	t.Run("edges_between_workspaces_succeed", func(t *testing.T) {
		nodeA := &Node{Summary: "Cross WS A", Type: "feature", Content: "a", Workspace: "api"}
		nodeB := &Node{Summary: "Cross WS B", Type: "feature", Content: "b", Workspace: "web"}
		store.CreateNode(nodeA)
		store.CreateNode(nodeB)

		// Cross-workspace linking should work
		err := store.LinkNodes(nodeA.ID, nodeB.ID, NodeRelationRelatesTo, 0.8, nil)
		if err != nil {
			t.Errorf("Cross-workspace LinkNodes should succeed, got: %v", err)
		}
	})

	t.Run("metadata_fields_populated", func(t *testing.T) {
		node := &Node{
			Summary:     "Service Metadata",
			Type:        "metadata",
			Content:     "git stats and docs metadata",
			Workspace:   "backend",
			SourceAgent: "git-stats",
		}
		if err := store.CreateNode(node); err != nil {
			t.Fatalf("CreateNode: %v", err)
		}

		// Retrieve and verify fields
		retrieved, err := store.GetNode(node.ID)
		if err != nil {
			t.Fatalf("GetNode: %v", err)
		}
		if retrieved.Workspace != "backend" {
			t.Errorf("Workspace = %q, want backend", retrieved.Workspace)
		}
		if retrieved.SourceAgent != "git-stats" {
			t.Errorf("SourceAgent = %q, want git-stats", retrieved.SourceAgent)
		}
		if retrieved.Type != "metadata" {
			t.Errorf("Type = %q, want metadata", retrieved.Type)
		}
	})
}
