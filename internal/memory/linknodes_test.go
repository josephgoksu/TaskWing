package memory

import (
	"strings"
	"testing"
)

func TestKnowledgeLinking_NoFK(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	t.Run("link_existing_nodes_succeeds", func(t *testing.T) {
		node1 := &Node{Summary: "Node A", Type: "feature", Content: "First node"}
		node2 := &Node{Summary: "Node B", Type: "feature", Content: "Second node"}

		if err := store.CreateNode(node1); err != nil {
			t.Fatalf("CreateNode A: %v", err)
		}
		if err := store.CreateNode(node2); err != nil {
			t.Fatalf("CreateNode B: %v", err)
		}

		err := store.LinkNodes(node1.ID, node2.ID, NodeRelationRelatesTo, 0.9, map[string]any{"test": true})
		if err != nil {
			t.Errorf("LinkNodes with existing nodes should succeed, got: %v", err)
		}

		edges, err := store.GetNodeEdges(node1.ID)
		if err != nil {
			t.Fatalf("GetNodeEdges: %v", err)
		}
		if len(edges) == 0 {
			t.Error("Expected at least 1 edge after LinkNodes")
		}
	})

	t.Run("link_missing_from_node_returns_error_not_panic", func(t *testing.T) {
		node := &Node{Summary: "Existing Node", Type: "feature", Content: "exists"}
		if err := store.CreateNode(node); err != nil {
			t.Fatalf("CreateNode: %v", err)
		}

		err := store.LinkNodes("nonexistent-id", node.ID, NodeRelationDependsOn, 1.0, nil)
		if err == nil {
			t.Error("LinkNodes with missing from_node should return error")
		}
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			t.Error("Got raw FK constraint error — should be handled gracefully")
		}
		if !strings.Contains(err.Error(), "link skipped") {
			t.Errorf("Expected 'link skipped' error, got: %v", err)
		}
	})

	t.Run("link_missing_to_node_returns_error_not_panic", func(t *testing.T) {
		node := &Node{Summary: "Another Node", Type: "feature", Content: "exists"}
		if err := store.CreateNode(node); err != nil {
			t.Fatalf("CreateNode: %v", err)
		}

		err := store.LinkNodes(node.ID, "nonexistent-id", NodeRelationDependsOn, 1.0, nil)
		if err == nil {
			t.Error("LinkNodes with missing to_node should return error")
		}
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			t.Error("Got raw FK constraint error — should be handled gracefully")
		}
	})

	t.Run("link_both_missing_returns_error", func(t *testing.T) {
		err := store.LinkNodes("fake-a", "fake-b", NodeRelationRelatesTo, 1.0, nil)
		if err == nil {
			t.Error("LinkNodes with both nodes missing should return error")
		}
	})

	t.Run("duplicate_link_ignored", func(t *testing.T) {
		node1 := &Node{Summary: "Dup A", Type: "feature", Content: "a"}
		node2 := &Node{Summary: "Dup B", Type: "feature", Content: "b"}
		store.CreateNode(node1)
		store.CreateNode(node2)

		err := store.LinkNodes(node1.ID, node2.ID, NodeRelationRelatesTo, 0.8, nil)
		if err != nil {
			t.Fatalf("First LinkNodes: %v", err)
		}

		// Duplicate should be silently ignored (INSERT OR IGNORE on unique constraint)
		err = store.LinkNodes(node1.ID, node2.ID, NodeRelationRelatesTo, 0.9, nil)
		if err != nil {
			t.Errorf("Duplicate LinkNodes should be ignored, got: %v", err)
		}
	})

	t.Run("node_deletion_cascades_edges", func(t *testing.T) {
		node1 := &Node{Summary: "Cascade A", Type: "feature", Content: "a"}
		node2 := &Node{Summary: "Cascade B", Type: "feature", Content: "b"}
		store.CreateNode(node1)
		store.CreateNode(node2)

		store.LinkNodes(node1.ID, node2.ID, NodeRelationRelatesTo, 1.0, nil)

		store.DeleteNode(node1.ID)

		edges, _ := store.GetNodeEdges(node2.ID)
		for _, e := range edges {
			if e.FromNode == node1.ID || e.ToNode == node1.ID {
				t.Error("Edge referencing deleted node should be cascaded")
			}
		}
	})
}
