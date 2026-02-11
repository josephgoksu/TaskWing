package knowledge

import (
	"context"
	"os"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// =============================================================================
// Ingestion Tests
// =============================================================================

// TestService_IngestFindings_BasicFinding tests basic finding ingestion.
func TestService_IngestFindings_BasicFinding(t *testing.T) {
	// Create temp directory for repository
	tmpDir, err := os.MkdirTemp("", "taskwing-ingest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize real repository using NewDefaultRepository
	repo, err := memory.NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create service with empty LLM config (embeddings disabled)
	svc := NewService(repo, llm.Config{})

	// Create a basic finding (simulating what bootstrap would produce)
	findings := []core.Finding{
		{
			Type:        memory.NodeTypeDecision,
			Title:       "Test Decision",
			Description: "This is a test decision for ingestion",
			SourceAgent: "test-agent",
			Metadata: map[string]any{
				"source": "test",
			},
		},
	}

	// Ingest the finding
	err = svc.IngestFindings(context.Background(), findings, nil, false)
	if err != nil {
		t.Fatalf("IngestFindings failed: %v", err)
	}

	// Verify the node was created
	nodes, err := repo.ListNodes("")
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(nodes) == 0 {
		t.Error("Expected at least one node after ingestion")
	}

	// Verify node content
	found := false
	for _, n := range nodes {
		if n.Summary == "Test Decision" && n.Type == memory.NodeTypeDecision {
			found = true
			if n.SourceAgent != "test-agent" {
				t.Errorf("SourceAgent = %q, want %q", n.SourceAgent, "test-agent")
			}
			break
		}
	}
	if !found {
		t.Error("Expected to find the ingested decision node")
	}
}

// TestService_IngestFindings_OpenCodeSkillMetadata tests ingestion of a finding
// that could come from an OpenCode skill analysis.
func TestService_IngestFindings_OpenCodeSkillMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-opencode-ingest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := memory.NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	svc := NewService(repo, llm.Config{})

	// Simulate a finding from OpenCode skill analysis
	// This tests that skill-related metadata can be properly ingested
	findings := []core.Finding{
		{
			Type:        memory.NodeTypePattern,
			Title:       "OpenCode Skills Pattern",
			Description: "OpenCode uses skills in .opencode/skills/<name>/SKILL.md format with YAML frontmatter",
			SourceAgent: "doc-agent",
			Metadata: map[string]any{
				"source":    "opencode",
				"skill_dir": ".opencode/skills/",
				"format":    "yaml-frontmatter",
			},
		},
		{
			Type:        memory.NodeTypeConstraint,
			Title:       "OpenCode Skill Name Validation",
			Description: "Skill names must match regex: ^[a-z0-9]+(-[a-z0-9]+)*$",
			SourceAgent: "doc-agent",
			Metadata: map[string]any{
				"source":  "opencode",
				"pattern": "^[a-z0-9]+(-[a-z0-9]+)*$",
			},
		},
	}

	err = svc.IngestFindings(context.Background(), findings, nil, false)
	if err != nil {
		t.Fatalf("IngestFindings failed: %v", err)
	}

	// Verify both nodes were created
	nodes, err := repo.ListNodes("")
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(nodes) < 2 {
		t.Errorf("Expected at least 2 nodes, got %d", len(nodes))
	}

	// Check for pattern node
	patternFound := false
	constraintFound := false
	for _, n := range nodes {
		if n.Type == memory.NodeTypePattern && n.Summary == "OpenCode Skills Pattern" {
			patternFound = true
		}
		if n.Type == memory.NodeTypeConstraint && n.Summary == "OpenCode Skill Name Validation" {
			constraintFound = true
		}
	}

	if !patternFound {
		t.Error("Pattern node not found")
	}
	if !constraintFound {
		t.Error("Constraint node not found")
	}
}

// TestService_IngestFindings_EmptyFindings tests that empty findings is a no-op.
func TestService_IngestFindings_EmptyFindings(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-empty-ingest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := memory.NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	svc := NewService(repo, llm.Config{})

	// Ingest empty findings - should be a no-op
	err = svc.IngestFindings(context.Background(), []core.Finding{}, nil, false)
	if err != nil {
		t.Errorf("IngestFindings with empty findings should not error: %v", err)
	}

	err = svc.IngestFindings(context.Background(), nil, nil, false)
	if err != nil {
		t.Errorf("IngestFindings with nil findings should not error: %v", err)
	}
}

// TestService_IngestFindings_MultipleTypes tests ingestion of multiple finding types.
func TestService_IngestFindings_MultipleTypes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-multi-type-ingest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := memory.NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	svc := NewService(repo, llm.Config{})

	// Create findings of different types
	findings := []core.Finding{
		{
			Type:        memory.NodeTypeDecision,
			Title:       "Architecture Decision",
			Description: "Use MVC pattern for web layer",
			SourceAgent: "code-agent",
		},
		{
			Type:        memory.NodeTypePattern,
			Title:       "Repository Pattern",
			Description: "Data access through repository interfaces",
			SourceAgent: "code-agent",
		},
		{
			Type:        memory.NodeTypeConstraint,
			Title:       "No External Dependencies",
			Description: "Must work offline without network access",
			SourceAgent: "code-agent",
		},
		{
			Type:        memory.NodeTypeFeature,
			Title:       "Semantic Search",
			Description: "Search using vector embeddings",
			SourceAgent: "doc-agent",
		},
		{
			Type:        memory.NodeTypeDocumentation,
			Title:       "API Documentation",
			Description: "OpenAPI spec for REST endpoints",
			SourceAgent: "doc-agent",
		},
	}

	err = svc.IngestFindings(context.Background(), findings, nil, false)
	if err != nil {
		t.Fatalf("IngestFindings failed: %v", err)
	}

	// Verify counts by type
	nodes, err := repo.ListNodes("")
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	typeCounts := make(map[string]int)
	for _, n := range nodes {
		typeCounts[n.Type]++
	}

	expectedTypes := []string{
		memory.NodeTypeDecision,
		memory.NodeTypePattern,
		memory.NodeTypeConstraint,
		memory.NodeTypeFeature,
		memory.NodeTypeDocumentation,
	}

	for _, expectedType := range expectedTypes {
		if typeCounts[expectedType] == 0 {
			t.Errorf("Expected at least one node of type %s", expectedType)
		}
	}
}

// TestService_IngestFindings_WithWorkspace tests ingestion with workspace tagging.
func TestService_IngestFindings_WithWorkspace(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-workspace-ingest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := memory.NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer func() { _ = repo.Close() }()

	svc := NewService(repo, llm.Config{})

	// Create findings with workspace metadata (simulating monorepo bootstrap)
	// NOTE: Titles must be sufficiently distinct to avoid Jaccard similarity deduplication
	// (threshold is 0.35). Using completely different titles avoids false positives.
	findings := []core.Finding{
		{
			Type:        memory.NodeTypeDecision,
			Title:       "REST Endpoint Authentication Strategy",
			Description: "JWT-based auth for API gateway",
			SourceAgent: "code-agent",
			Metadata: map[string]any{
				"service":   "api",
				"workspace": "api",
			},
		},
		{
			Type:        memory.NodeTypePattern,
			Title:       "React Component Composition Pattern",
			Description: "Higher-order components for shared UI logic",
			SourceAgent: "code-agent",
			Metadata: map[string]any{
				"service":   "web",
				"workspace": "web",
			},
		},
	}

	err = svc.IngestFindings(context.Background(), findings, nil, false)
	if err != nil {
		t.Fatalf("IngestFindings failed: %v", err)
	}

	// Verify nodes exist
	nodes, err := repo.ListNodes("")
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(nodes) < 2 {
		t.Errorf("Expected at least 2 nodes, got %d", len(nodes))
	}

	// Verify both workspaces are represented
	workspaces := make(map[string]bool)
	for _, n := range nodes {
		workspaces[n.Workspace] = true
	}
	if !workspaces["api"] {
		t.Error("Expected a node with workspace 'api'")
	}
	if !workspaces["web"] {
		t.Error("Expected a node with workspace 'web'")
	}
}

// =============================================================================
// Repository Integration Tests (using NewDefaultRepository)
// =============================================================================

// TestNewDefaultRepository_CreateAndRetrieve tests basic repository operations.
func TestNewDefaultRepository_CreateAndRetrieve(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-repo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Use NewDefaultRepository as mandated by constraints
	repo, err := memory.NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewDefaultRepository failed: %v", err)
	}
	defer func() { _ = repo.Close() }()

	// Create a node
	testNode := &memory.Node{
		ID:        "test-node-create-retrieve",
		Content:   "Test content for create/retrieve test",
		Type:      memory.NodeTypeDecision,
		Summary:   "Test Summary",
		Workspace: "root",
	}

	err = repo.CreateNode(testNode)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	// Retrieve the node
	retrieved, err := repo.GetNode("test-node-create-retrieve")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetNode returned nil")
	}

	// Verify content
	if retrieved.Summary != "Test Summary" {
		t.Errorf("Summary = %q, want %q", retrieved.Summary, "Test Summary")
	}
	if retrieved.Type != memory.NodeTypeDecision {
		t.Errorf("Type = %q, want %q", retrieved.Type, memory.NodeTypeDecision)
	}
}

// TestNewDefaultRepository_SQLiteIsCanonical verifies SQLite is used as the source of truth.
func TestNewDefaultRepository_SQLiteIsCanonical(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskwing-sqlite-canonical-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repo, err := memory.NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewDefaultRepository failed: %v", err)
	}

	// Create multiple nodes
	for i := 0; i < 3; i++ {
		node := &memory.Node{
			ID:      "node-sqlite-" + string(rune('a'+i)),
			Content: "Content " + string(rune('a'+i)),
			Type:    memory.NodeTypeDecision,
			Summary: "Summary " + string(rune('a'+i)),
		}
		if err := repo.CreateNode(node); err != nil {
			t.Fatalf("CreateNode failed for %d: %v", i, err)
		}
	}

	// Close and reopen to verify persistence
	if err := repo.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	repo2, err := memory.NewDefaultRepository(tmpDir)
	if err != nil {
		t.Fatalf("NewDefaultRepository (reopen) failed: %v", err)
	}
	defer func() { _ = repo2.Close() }()

	// Verify data persisted (SQLite is the source of truth)
	nodes, err := repo2.ListNodes("")
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes after reopen, got %d", len(nodes))
	}
}
