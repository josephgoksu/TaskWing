package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleStats_IncludesPatternCount(t *testing.T) {
	// Setup temp directory with test database
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, ".taskwing", "memory")
	require.NoError(t, os.MkdirAll(memDir, 0755))

	store, err := memory.NewSQLiteStore(memDir)
	require.NoError(t, err)
	defer store.Close()

	// Create test nodes of each type
	testNodes := []memory.Node{
		{ID: "f1", Type: "feature", Summary: "Feature 1"},
		{ID: "f2", Type: "feature", Summary: "Feature 2"},
		{ID: "d1", Type: "decision", Summary: "Decision 1"},
		{ID: "d2", Type: "decision", Summary: "Decision 2"},
		{ID: "d3", Type: "decision", Summary: "Decision 3"},
		{ID: "p1", Type: "pattern", Summary: "Pattern 1"},
		{ID: "p2", Type: "pattern", Summary: "Pattern 2"},
	}

	for _, n := range testNodes {
		require.NoError(t, store.CreateNode(n))
	}

	// Create server with test store
	srv := &Server{store: store}

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()

	// Call handler
	srv.handleStats(rec, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rec.Code)

	var stats map[string]int
	err = json.Unmarshal(rec.Body.Bytes(), &stats)
	require.NoError(t, err)

	// Verify all node types are counted
	assert.Equal(t, 7, stats["total"], "total should be 7")
	assert.Equal(t, 2, stats["feature"], "feature should be 2")
	assert.Equal(t, 3, stats["decision"], "decision should be 3")
	assert.Equal(t, 2, stats["pattern"], "pattern should be 2")
}

func TestHandleStats_EmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, ".taskwing", "memory")
	require.NoError(t, os.MkdirAll(memDir, 0755))

	store, err := memory.NewSQLiteStore(memDir)
	require.NoError(t, err)
	defer store.Close()

	srv := &Server{store: store}

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()

	srv.handleStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stats map[string]int
	err = json.Unmarshal(rec.Body.Bytes(), &stats)
	require.NoError(t, err)

	assert.Equal(t, 0, stats["total"])
	assert.Equal(t, 0, stats["feature"])
	assert.Equal(t, 0, stats["decision"])
	assert.Equal(t, 0, stats["pattern"])
}

func TestHandleAgents_CountsBySourceAgent(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, ".taskwing", "memory")
	require.NoError(t, os.MkdirAll(memDir, 0755))

	store, err := memory.NewSQLiteStore(memDir)
	require.NoError(t, err)
	defer store.Close()

	// Create nodes with different source agents
	testNodes := []memory.Node{
		{ID: "1", Type: "feature", Summary: "F1", SourceAgent: "doc"},
		{ID: "2", Type: "feature", Summary: "F2", SourceAgent: "doc"},
		{ID: "3", Type: "decision", Summary: "D1", SourceAgent: "git"},
		{ID: "4", Type: "decision", Summary: "D2", SourceAgent: "deps"},
		{ID: "5", Type: "pattern", Summary: "P1", SourceAgent: "react_code"},
	}

	for _, n := range testNodes {
		require.NoError(t, store.CreateNode(n))
	}

	srv := &Server{store: store}

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	rec := httptest.NewRecorder()

	srv.handleAgents(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var agents []struct {
		ID        string `json:"id"`
		NodeCount int    `json:"nodeCount"`
	}
	err = json.Unmarshal(rec.Body.Bytes(), &agents)
	require.NoError(t, err)

	// Find counts by agent ID
	counts := make(map[string]int)
	for _, a := range agents {
		counts[a.ID] = a.NodeCount
	}

	assert.Equal(t, 2, counts["doc"], "doc agent should have 2 nodes")
	assert.Equal(t, 1, counts["git"], "git agent should have 1 node")
	assert.Equal(t, 1, counts["deps"], "deps agent should have 1 node")
	assert.Equal(t, 1, counts["react_code"], "react_code agent should have 1 node")
}
