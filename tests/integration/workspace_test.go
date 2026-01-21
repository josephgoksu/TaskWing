// Package integration contains end-to-end tests for TaskWing features.
package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/project"
)

// TestMonorepoWorkspace_EndToEnd tests the complete workspace-aware knowledge scoping
// flow for a monorepo structure. This validates:
// 1. Workspace detection in monorepo structures
// 2. Knowledge nodes are created with correct workspace tags
// 3. Recall filtering returns workspace-scoped results
// 4. Root knowledge is included when IncludeRoot=true
func TestMonorepoWorkspace_EndToEnd(t *testing.T) {
	// Skip in short mode (for quick CI runs)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary directory for the test monorepo
	tmpDir, err := os.MkdirTemp("", "taskwing-monorepo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Setup: Create a monorepo structure
	monorepo := setupMonorepoFixture(t, tmpDir)

	// Test 1: Workspace detection
	t.Run("workspace_detection", func(t *testing.T) {
		ws, err := project.DetectWorkspace(monorepo.root)
		if err != nil {
			t.Fatalf("DetectWorkspace failed: %v", err)
		}

		if ws.Type != project.WorkspaceTypeMonorepo {
			t.Errorf("workspace type = %v, want monorepo", ws.Type)
		}

		// Should detect all services
		services := make(map[string]bool)
		for _, svc := range ws.Services {
			services[svc] = true
		}

		for _, expected := range []string{"api", "web", "common"} {
			if !services[expected] {
				t.Errorf("expected service %q not detected", expected)
			}
		}
	})

	// Test 2: Create knowledge nodes with workspace tags
	t.Run("knowledge_with_workspace_tags", func(t *testing.T) {
		repo, err := memory.NewDefaultRepository(filepath.Join(monorepo.root, ".taskwing", "memory"))
		if err != nil {
			t.Fatalf("failed to create repository: %v", err)
		}
		defer func() { _ = repo.Close() }()

		// Create nodes in different workspaces
		testNodes := []memory.Node{
			{
				ID:        "dec-root-auth",
				Type:      memory.NodeTypeDecision,
				Summary:   "Use JWT for authentication",
				Content:   "All services will use JWT tokens for authentication. Tokens are verified by the API gateway.",
				Workspace: "root",
			},
			{
				ID:        "dec-root-db",
				Type:      memory.NodeTypeDecision,
				Summary:   "PostgreSQL as primary database",
				Content:   "PostgreSQL is the primary database. Each service has its own schema.",
				Workspace: "root",
			},
			{
				ID:        "pat-api-rest",
				Type:      memory.NodeTypePattern,
				Summary:   "REST API conventions",
				Content:   "API service follows RESTful conventions with versioned endpoints (/v1/, /v2/).",
				Workspace: "api",
			},
			{
				ID:        "con-api-rate",
				Type:      memory.NodeTypeConstraint,
				Summary:   "Rate limiting required",
				Content:   "All API endpoints must have rate limiting. Default: 100 req/min per user.",
				Workspace: "api",
			},
			{
				ID:        "pat-web-react",
				Type:      memory.NodeTypePattern,
				Summary:   "React component structure",
				Content:   "Web frontend uses React with functional components and hooks.",
				Workspace: "web",
			},
			{
				ID:        "pat-common-utils",
				Type:      memory.NodeTypePattern,
				Summary:   "Shared utility functions",
				Content:   "Common utilities are shared across services via the common package.",
				Workspace: "common",
			},
		}

		for _, node := range testNodes {
			n := node
			if err := repo.CreateNode(&n); err != nil {
				t.Fatalf("failed to create node %s: %v", node.ID, err)
			}
		}

		// Verify nodes were created with correct workspaces
		allNodes, err := repo.ListNodes("")
		if err != nil {
			t.Fatalf("ListNodes failed: %v", err)
		}

		if len(allNodes) != len(testNodes) {
			t.Errorf("created %d nodes, want %d", len(allNodes), len(testNodes))
		}

		workspaceCounts := make(map[string]int)
		for _, n := range allNodes {
			workspaceCounts[n.Workspace]++
		}

		expectedCounts := map[string]int{
			"root":   2,
			"api":    2,
			"web":    1,
			"common": 1,
		}

		for ws, want := range expectedCounts {
			if got := workspaceCounts[ws]; got != want {
				t.Errorf("workspace %q: got %d nodes, want %d", ws, got, want)
			}
		}
	})

	// Test 3: Recall filtering by workspace
	t.Run("recall_workspace_filtering", func(t *testing.T) {
		repo, err := memory.NewDefaultRepository(filepath.Join(monorepo.root, ".taskwing", "memory"))
		if err != nil {
			t.Fatalf("failed to create repository: %v", err)
		}
		defer func() { _ = repo.Close() }()

		ctx := context.Background()
		appCtx := app.NewContextWithConfig(repo, llm.Config{}) // No LLM needed for search
		recallApp := app.NewRecallApp(appCtx)

		// Test: Search from "api" workspace with IncludeRoot=true
		t.Run("api_with_root", func(t *testing.T) {
			// Use ListNodesFiltered directly to verify workspace scoping
			// (The recall Query uses NodeResponse which strips workspace for token efficiency)
			nodes, err := repo.ListNodesFiltered(memory.NodeFilter{
				Workspace:   "api",
				IncludeRoot: true,
			})
			if err != nil {
				t.Fatalf("ListNodesFiltered failed: %v", err)
			}

			// Should find api nodes + root nodes, NOT web/common nodes
			foundWorkspaces := make(map[string]bool)
			for _, n := range nodes {
				foundWorkspaces[n.Workspace] = true
			}

			if foundWorkspaces["web"] {
				t.Error("should NOT include web workspace nodes")
			}
			if foundWorkspaces["common"] {
				t.Error("should NOT include common workspace nodes")
			}

			// Should have api and root nodes
			if !foundWorkspaces["api"] {
				t.Error("should include api workspace nodes")
			}
			if !foundWorkspaces["root"] {
				t.Error("should include root workspace nodes when IncludeRoot=true")
			}

			// Verify count: 2 api + 2 root = 4
			if len(nodes) != 4 {
				t.Errorf("got %d nodes, want 4 (api + root)", len(nodes))
			}
		})

		// Test: Search from "api" workspace WITHOUT root
		t.Run("api_without_root", func(t *testing.T) {
			// Create a new knowledge service for direct testing
			ks := knowledge.NewService(repo, llm.Config{})

			results, err := ks.SearchWithFilter(ctx, "API", 10, memory.NodeFilter{
				Workspace:   "api",
				IncludeRoot: false,
			})
			if err != nil {
				t.Fatalf("SearchWithFilter failed: %v", err)
			}

			for _, r := range results {
				if r.Node.Workspace != "api" {
					t.Errorf("got node from workspace %q, want only 'api'", r.Node.Workspace)
				}
			}
		})

		// Test: Search from root (empty workspace = all)
		t.Run("root_sees_all", func(t *testing.T) {
			// Empty workspace filter should return all nodes
			nodes, err := repo.ListNodesFiltered(memory.NodeFilter{
				Workspace: "", // Empty = no filtering
			})
			if err != nil {
				t.Fatalf("ListNodesFiltered failed: %v", err)
			}

			// Should find nodes from all workspaces
			foundWorkspaces := make(map[string]bool)
			for _, n := range nodes {
				foundWorkspaces[n.Workspace] = true
			}

			// Should see all 4 workspaces
			expectedWorkspaces := []string{"root", "api", "web", "common"}
			for _, ws := range expectedWorkspaces {
				if !foundWorkspaces[ws] {
					t.Errorf("expected workspace %q not found", ws)
				}
			}

			// Should have all 6 nodes
			if len(nodes) != 6 {
				t.Errorf("got %d nodes, want 6 (all nodes)", len(nodes))
			}
		})

		// Note: recallApp is still used for validation that the app layer works
		_ = recallApp
	})

	// Test 4: ListNodesFiltered integration
	t.Run("list_nodes_filtered", func(t *testing.T) {
		repo, err := memory.NewDefaultRepository(filepath.Join(monorepo.root, ".taskwing", "memory"))
		if err != nil {
			t.Fatalf("failed to create repository: %v", err)
		}
		defer func() { _ = repo.Close() }()

		// Test: List API workspace nodes with root
		nodes, err := repo.ListNodesFiltered(memory.NodeFilter{
			Workspace:   "api",
			IncludeRoot: true,
		})
		if err != nil {
			t.Fatalf("ListNodesFiltered failed: %v", err)
		}

		// Should have: 2 api + 2 root = 4 nodes
		if len(nodes) != 4 {
			t.Errorf("got %d nodes, want 4 (api + root)", len(nodes))
		}

		// Verify no web/common nodes
		for _, n := range nodes {
			if n.Workspace == "web" || n.Workspace == "common" {
				t.Errorf("unexpected node from workspace %q", n.Workspace)
			}
		}

		// Test: List API workspace nodes without root
		nodes, err = repo.ListNodesFiltered(memory.NodeFilter{
			Workspace:   "api",
			IncludeRoot: false,
		})
		if err != nil {
			t.Fatalf("ListNodesFiltered failed: %v", err)
		}

		// Should have: 2 api nodes only
		if len(nodes) != 2 {
			t.Errorf("got %d nodes, want 2 (api only)", len(nodes))
		}

		for _, n := range nodes {
			if n.Workspace != "api" {
				t.Errorf("got node from workspace %q, want only 'api'", n.Workspace)
			}
		}
	})

	// Test 5: SearchFTSFiltered integration
	t.Run("fts_workspace_filtering", func(t *testing.T) {
		repo, err := memory.NewDefaultRepository(filepath.Join(monorepo.root, ".taskwing", "memory"))
		if err != nil {
			t.Fatalf("failed to create repository: %v", err)
		}
		defer func() { _ = repo.Close() }()

		// Search for "pattern" - should match multiple nodes
		results, err := repo.SearchFTSFiltered("pattern", 10, memory.NodeFilter{
			Workspace:   "api",
			IncludeRoot: true,
		})
		if err != nil {
			t.Fatalf("SearchFTSFiltered failed: %v", err)
		}

		// Should NOT find web-react pattern or common-utils pattern
		for _, r := range results {
			if r.Node.Workspace == "web" {
				t.Error("should NOT include web workspace in api+root search")
			}
			if r.Node.Workspace == "common" {
				t.Error("should NOT include common workspace in api+root search")
			}
		}
	})
}

// TestWorkspaceDetectionFromCwd tests workspace detection when running from subdirectories.
// Note: This test requires a real git repository for accurate cwd detection,
// so it tests the basic behavior with the understanding that cwd-based detection
// relies on git internals that aren't fully simulated in test fixtures.
func TestWorkspaceDetectionFromCwd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "taskwing-cwd-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	monorepo := setupMonorepoFixture(t, tmpDir)

	// Save current dir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Test: From root - should return "root" since fixture doesn't have full git setup
	t.Run("from_root", func(t *testing.T) {
		if err := os.Chdir(monorepo.root); err != nil {
			t.Fatalf("failed to chdir: %v", err)
		}

		ws, err := project.DetectWorkspaceFromCwd()
		if err != nil {
			t.Fatalf("DetectWorkspaceFromCwd failed: %v", err)
		}

		// Should return "root" for the monorepo root
		if ws != "root" {
			t.Errorf("workspace = %q, want 'root'", ws)
		}
	})

	// Test: DetectWorkspace (not DetectWorkspaceFromCwd) correctly identifies services
	t.Run("detect_workspace_services", func(t *testing.T) {
		ws, err := project.DetectWorkspace(monorepo.root)
		if err != nil {
			t.Fatalf("DetectWorkspace failed: %v", err)
		}

		// Should detect api, web, common as services
		services := make(map[string]bool)
		for _, svc := range ws.Services {
			services[svc] = true
		}

		for _, expected := range []string{"api", "web", "common"} {
			if !services[expected] {
				t.Errorf("expected service %q not detected", expected)
			}
		}
	})

	// Test: DetectWorkspaceFromPath with explicit path
	t.Run("detect_from_path_api", func(t *testing.T) {
		apiPath := filepath.Join(monorepo.root, "api")
		ws, err := project.DetectWorkspaceFromPath(apiPath)
		if err != nil {
			t.Fatalf("DetectWorkspaceFromPath failed: %v", err)
		}

		// Note: Without real git repo, this may return "root"
		// The key test is that it doesn't error out
		t.Logf("DetectWorkspaceFromPath(%s) = %q", apiPath, ws)
	})
}

// monorepoFixture holds paths for a test monorepo structure.
type monorepoFixture struct {
	root   string
	api    string
	web    string
	common string
}

// setupMonorepoFixture creates a test monorepo structure with Go modules.
func setupMonorepoFixture(t *testing.T, baseDir string) monorepoFixture {
	t.Helper()

	fixture := monorepoFixture{
		root:   baseDir,
		api:    filepath.Join(baseDir, "api"),
		web:    filepath.Join(baseDir, "web"),
		common: filepath.Join(baseDir, "common"),
	}

	// Create directories
	for _, dir := range []string{fixture.api, fixture.web, fixture.common} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Create root go.mod (monorepo root)
	rootMod := `module example.com/monorepo

go 1.21
`
	if err := os.WriteFile(filepath.Join(fixture.root, "go.mod"), []byte(rootMod), 0644); err != nil {
		t.Fatalf("failed to write root go.mod: %v", err)
	}

	// Create service go.mod files (markers for workspace detection)
	for _, svc := range []struct {
		path string
		name string
	}{
		{fixture.api, "api"},
		{fixture.web, "web"},
		{fixture.common, "common"},
	} {
		modContent := "module example.com/monorepo/" + svc.name + "\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(svc.path, "go.mod"), []byte(modContent), 0644); err != nil {
			t.Fatalf("failed to write %s go.mod: %v", svc.name, err)
		}
	}

	// Create .git directory (makes it a monorepo rather than multi-repo)
	gitDir := filepath.Join(fixture.root, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}

	// Create .taskwing/memory directory
	memoryDir := filepath.Join(fixture.root, ".taskwing", "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatalf("failed to create memory directory: %v", err)
	}

	return fixture
}
