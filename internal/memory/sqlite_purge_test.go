package memory

import (
	"testing"
	"time"
)

func TestDeleteNodesByFiles(t *testing.T) {
	db, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Seed data
	nodes := []Node{
		{
			ID:          "n1",
			SourceAgent: "test-agent",
			Content:     "Node 1",
			Evidence:    `[{"file_path": "foo.go"}]`,
			CreatedAt:   time.Now(),
		},
		{
			ID:          "n2",
			SourceAgent: "test-agent",
			Content:     "Node 2",
			Evidence:    `[{"file_path": "bar.go"}]`,
			CreatedAt:   time.Now(),
		},
		{
			ID:          "n3",
			SourceAgent: "other-agent",
			Content:     "Node 3",
			Evidence:    `[{"file_path": "foo.go"}]`,
			CreatedAt:   time.Now(),
		},
	}

	for _, n := range nodes {
		if err := db.CreateNode(n); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	// Test: Purge foo.go for test-agent
	// Only n1 should be deleted
	if err := db.DeleteNodesByFiles("test-agent", []string{"foo.go"}); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify n1 is gone (deleted)
	_, err = db.GetNode("n1")
	if err == nil {
		t.Errorf("n1 should be deleted")
	}

	// Verify n2 remains (same agent, different file)
	_, err = db.GetNode("n2")
	if err != nil {
		t.Errorf("n2 should remain: %v", err)
	}

	// Verify n3 remains (different agent, same file)
	_, err = db.GetNode("n3")
	if err != nil {
		t.Errorf("n3 should remain: %v", err)
	}
}
