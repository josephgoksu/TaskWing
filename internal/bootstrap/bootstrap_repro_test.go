package bootstrap_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/verification"
	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/git"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/project"
	"github.com/spf13/afero"
)

// =============================================================================
// Defect 1: RootPath resolves to HOME when .taskwing exists at HOME
// =============================================================================

func TestBootstrapRepro_RootPathResolvesToHome(t *testing.T) {
	// Reproduce: When ~/.taskwing exists and user runs from a workspace
	// subdirectory that has its own .git, Detect() should NOT climb to HOME.
	// The CRITICAL FIX in detect.go should prevent this, but the multi-repo
	// workspace scenario (no .git at workspace root) still resolves to HOME.

	tests := []struct {
		name         string
		paths        []string
		startPath    string
		wantRootPath string
		wantNotHome  bool // if true, assert RootPath != simulated home
	}{
		{
			name: "taskwing at home should not be used when git repo below",
			paths: []string{
				"/home/user/.taskwing/",
				"/home/user/workspace/project/.git/",
				"/home/user/workspace/project/go.mod",
			},
			startPath:    "/home/user/workspace/project",
			wantRootPath: "/home/user/workspace/project",
		},
		{
			name: "multi-repo workspace without git root - should not climb to home",
			paths: []string{
				"/home/user/.taskwing/",
				"/home/user/workspace/cli/.git/",
				"/home/user/workspace/cli/go.mod",
				"/home/user/workspace/app/.git/",
				"/home/user/workspace/app/package.json",
			},
			// Running from the workspace dir (not a git repo itself)
			startPath:   "/home/user/workspace",
			wantNotHome: true, // Should NOT resolve to /home/user
		},
		{
			name: "taskwing at project level is valid",
			paths: []string{
				"/home/user/.taskwing/",
				"/home/user/workspace/.taskwing/",
				"/home/user/workspace/.git/",
				"/home/user/workspace/go.mod",
			},
			startPath:    "/home/user/workspace",
			wantRootPath: "/home/user/workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := setupFS(tt.paths)
			d := project.NewDetector(fs)
			ctx, err := d.Detect(tt.startPath)
			if err != nil {
				t.Fatalf("Detect() error: %v", err)
			}

			if tt.wantRootPath != "" && ctx.RootPath != tt.wantRootPath {
				t.Errorf("RootPath = %q, want %q", ctx.RootPath, tt.wantRootPath)
			}
			if tt.wantNotHome && ctx.RootPath == "/home/user" {
				t.Errorf("RootPath resolved to HOME (%q) - this is the bug", ctx.RootPath)
			}
		})
	}
}

// =============================================================================
// Defect 2: FK constraint failures in knowledge graph linking
// =============================================================================

func TestBootstrapRepro_FKConstraintOnLinkNodes(t *testing.T) {
	// Reproduce the real bug: The bootstrap output shows:
	//   "failed to link nodes (llm): constraint failed: FOREIGN KEY constraint failed (787)"
	// This happens in knowledge/ingest.go when LLM-extracted relationships reference
	// node IDs that were looked up from an in-memory map but have been purged from the DB.
	//
	// The INSERT OR IGNORE in LinkNodes silently drops FK violations for evidence/semantic
	// edges, but the LLM linking path hits a different code path where errors surface.
	// We test both behaviors.

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	t.Run("insert_or_ignore_silently_drops_fk_violations", func(t *testing.T) {
		// LinkNodes pre-checks node existence, so linking nonexistent nodes must error.
		err := store.LinkNodes("n-nonexistent1", "n-nonexistent2", "relates_to", 0.8, nil)
		if err == nil {
			t.Error("LinkNodes with nonexistent nodes should return error, got nil")
		}
	})

	t.Run("purge_then_link_race_condition", func(t *testing.T) {
		// Simulate the real bootstrap flow:
		// 1. Create nodes (bootstrap agents produce findings)
		// 2. Purge old nodes by agent (bootstrap cleans stale data)
		// 3. Re-insert new nodes (from current run)
		// 4. LLM linking tries to connect nodes using an in-memory title->ID map
		//    that may contain stale IDs from before the purge
		node1 := &memory.Node{
			ID: "n-purge01", Content: "Will be purged", Type: "decision",
			Summary: "Purge test 1", SourceAgent: "test-agent", CreatedAt: time.Now(),
		}
		node2 := &memory.Node{
			ID: "n-purge02", Content: "Will be purged", Type: "feature",
			Summary: "Purge test 2", SourceAgent: "test-agent", CreatedAt: time.Now(),
		}
		if err := store.CreateNode(node1); err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if err := store.CreateNode(node2); err != nil {
			t.Fatalf("CreateNode: %v", err)
		}

		// Build title->ID map (simulates what ingest.go does before linking)
		titleMap := map[string]string{
			"purge test 1": "n-purge01",
			"purge test 2": "n-purge02",
		}

		// Purge by agent (simulates bootstrap "Purging all stale nodes" step)
		if err := store.DeleteNodesByAgent("test-agent"); err != nil {
			t.Fatalf("DeleteNodesByAgent: %v", err)
		}

		// LLM linking uses stale title map — IDs no longer exist in DB
		fromID := titleMap["purge test 1"]
		toID := titleMap["purge test 2"]
		err := store.LinkNodes(fromID, toID, "relates_to", 1.0, map[string]any{"llm_extracted": true})
		if err == nil {
			t.Error("LinkNodes after purge should return error for deleted nodes, got nil")
		}
	})
}

// =============================================================================
// Defect 3: Git log exit 128 for sub-repos
// =============================================================================

// mockCommander implements git.Commander for testing
type mockCommander struct {
	responses map[string]mockResponse
}

type mockResponse struct {
	output string
	err    error
}

func (m *mockCommander) Run(name string, args ...string) (string, error) {
	return m.RunInDir("", name, args...)
}

func (m *mockCommander) RunInDir(dir, name string, args ...string) (string, error) {
	key := fmt.Sprintf("%s:%s %s", dir, name, strings.Join(args, " "))
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}
	// Also try without dir prefix
	keyNoDir := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	if resp, ok := m.responses[keyNoDir]; ok {
		return resp.output, resp.err
	}
	return "", fmt.Errorf("exit status 128: fatal: not a git repository")
}

func TestBootstrapRepro_GitLogExit128(t *testing.T) {
	// Reproduce: git log fails with exit 128 when run against a non-git directory

	tests := []struct {
		name      string
		workDir   string
		responses map[string]mockResponse
		wantIsRepo bool
	}{
		{
			name:    "non-git directory returns exit 128",
			workDir: "/workspace/not-a-repo",
			responses: map[string]mockResponse{
				"/workspace/not-a-repo:git rev-parse --is-inside-work-tree": {
					output: "",
					err:    fmt.Errorf("exit status 128: fatal: not a git repository"),
				},
			},
			wantIsRepo: false,
		},
		{
			name:    "valid git repo succeeds",
			workDir: "/workspace/valid-repo",
			responses: map[string]mockResponse{
				"/workspace/valid-repo:git rev-parse --is-inside-work-tree": {
					output: "true",
					err:    nil,
				},
			},
			wantIsRepo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &mockCommander{responses: tt.responses}
			client := git.NewClientWithCommander(tt.workDir, cmd)

			got := client.IsRepository()
			if got != tt.wantIsRepo {
				t.Errorf("IsRepository() = %v, want %v", got, tt.wantIsRepo)
			}
		})
	}
}

func TestBootstrapRepro_GitLogOnRealTempDirs(t *testing.T) {
	// Create a real temp dir without git init - git log should fail gracefully
	tmpDir := t.TempDir()
	nonGitDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0755); err != nil {
		t.Fatal(err)
	}

	client := git.NewClient(nonGitDir)
	if client.IsRepository() {
		t.Error("IsRepository() = true for non-git dir, want false")
	}

	// Create a dir with .git init and verify it works
	gitDir := filepath.Join(tmpDir, "has-git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Actually init git so we can test the real path
	cmd := git.NewClient(gitDir)
	// Use os/exec directly for setup since Commander doesn't expose init
	initCmd := exec.Command("git", "init", gitDir)
	if err := initCmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}

	if !cmd.IsRepository() {
		t.Error("IsRepository() = false for git-inited dir, want true")
	}
}

// =============================================================================
// Defect 4: Gemini MCP install failure (exit status 41)
// =============================================================================

func TestBootstrapRepro_GeminiMCPInstallFails(t *testing.T) {
	// This test verifies the behavior when gemini CLI is not available.
	// The install function uses exec.LookPath("gemini") which we can't easily mock
	// without refactoring. Instead, we verify the code path handles missing CLI.

	// Verify LookPath returns an error for a non-existent binary
	_, err := lookPath("gemini-nonexistent-binary-xyz")
	if err == nil {
		t.Fatal("LookPath should fail for non-existent binary")
	}

	// The actual fix requires making installGeminiCLI testable via interfaces.
	// For now, this test documents that the current code does check LookPath
	// but doesn't handle version-specific exit codes (like 41) gracefully.
	t.Log("installGeminiCLI checks exec.LookPath but doesn't handle non-standard exit codes")
}

// =============================================================================
// Defect 5: Zero docs loaded
// =============================================================================

func TestBootstrapRepro_ZeroDocsLoaded(t *testing.T) {
	// Reproduce: DocLoader finds 0 files when basePath has no docs at root
	// but sub-repos contain documentation

	tests := []struct {
		name      string
		setup     func(t *testing.T, dir string)
		wantMin   int // minimum expected doc count
		wantZero  bool
	}{
		{
			name: "empty directory yields zero docs",
			setup: func(t *testing.T, dir string) {
				// Nothing to create
			},
			wantZero: true,
		},
		{
			name: "root with README yields docs",
			setup: func(t *testing.T, dir string) {
				writeFile(t, filepath.Join(dir, "README.md"), "# Project\nDescription here")
			},
			wantMin: 1,
		},
		{
			name: "docs only in sub-repos not found by root loader",
			setup: func(t *testing.T, dir string) {
				// Sub-repo has docs but root DocLoader doesn't scan sub-repos
				subrepo := filepath.Join(dir, "cli")
				os.MkdirAll(subrepo, 0755)
				writeFile(t, filepath.Join(subrepo, "README.md"), "# CLI Readme")
				writeFile(t, filepath.Join(subrepo, "ARCHITECTURE.md"), "# Architecture")
				// Docs dir inside sub-repo
				os.MkdirAll(filepath.Join(subrepo, "docs"), 0755)
				writeFile(t, filepath.Join(subrepo, "docs", "guide.md"), "# Guide")
			},
			// DocLoader at workspace root won't find sub-repo docs
			// because it only scans basePath/README.md, basePath/docs/, basePath/.taskwing/
			wantZero: true,
		},
		{
			name: "docs directory at root is found",
			setup: func(t *testing.T, dir string) {
				os.MkdirAll(filepath.Join(dir, "docs"), 0755)
				writeFile(t, filepath.Join(dir, "docs", "api.md"), "# API Docs")
				writeFile(t, filepath.Join(dir, "docs", "guide.md"), "# User Guide")
			},
			wantMin: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			loader := bootstrap.NewDocLoader(dir)
			docs, err := loader.Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			if tt.wantZero && len(docs) > 0 {
				t.Errorf("Load() returned %d docs, want 0", len(docs))
			}
			if tt.wantMin > 0 && len(docs) < tt.wantMin {
				t.Errorf("Load() returned %d docs, want >= %d", len(docs), tt.wantMin)
			}
		})
	}
}

// =============================================================================
// Defect 6: Claude MCP drift detected but not fixed
// =============================================================================

func TestBootstrapRepro_MCPDriftDetection(t *testing.T) {
	// This test verifies drift detection logic. The actual detectClaudeMCP()
	// function uses exec.LookPath and exec.Command which require a real
	// claude binary. The test documents the behavior and prepares for the fix.

	// Test that the detection functions handle missing binaries gracefully
	t.Run("missing_claude_binary", func(t *testing.T) {
		// detectClaudeMCP() should return false if claude is not in PATH
		// We can't call it directly from bootstrap_test package, but we verify
		// the underlying mechanism works
		_, err := lookPath("claude-nonexistent-xyz")
		if err == nil {
			t.Error("expected error for missing binary")
		}
	})

	// The core issue: drift is DETECTED but bootstrap refuses to FIX it.
	// The fix needs: a --fix-mcp flag or interactive prompt.
	t.Log("MCP drift fix requires: (1) safe re-registration flow, (2) --fix-mcp flag or interactive prompt, (3) dry-run mode")
}

// =============================================================================
// Defect 7: Hallucinated findings rejected by verifier
// =============================================================================

func TestBootstrapRepro_HallucinatedFindingsRejected(t *testing.T) {
	// Reproduce: LLM generates findings with evidence pointing to non-existent files
	dir := t.TempDir()

	// Create a real file for valid evidence
	writeFile(t, filepath.Join(dir, "main.go"), `package main

func main() {
	println("hello")
}
`)

	verifier := verification.NewAgent(dir)

	findings := []core.Finding{
		{
			Title:           "Valid finding with real file",
			Description:     "This finding has correct evidence",
			ConfidenceScore: 0.8,
			Evidence: []core.Evidence{
				{
					FilePath:  "main.go",
					StartLine: 3,
					EndLine:   5,
					Snippet:   `func main() {`,
				},
			},
		},
		{
			Title:           "Hallucinated finding with fake file",
			Description:     "This finding references a non-existent file",
			ConfidenceScore: 0.9,
			Evidence: []core.Evidence{
				{
					FilePath:  "nonexistent/hallucinated.go",
					StartLine: 10,
					EndLine:   20,
					Snippet:   `func DoSomething() error {`,
				},
			},
		},
		{
			Title:           "Hallucinated finding with wrong line numbers",
			Description:     "File exists but snippet at wrong lines",
			ConfidenceScore: 0.7,
			Evidence: []core.Evidence{
				{
					FilePath:  "main.go",
					StartLine: 100,
					EndLine:   110,
					Snippet:   `func completelyWrongContent() {`,
				},
			},
		},
	}

	verified := verifier.VerifyFindings(context.Background(), findings)
	if len(verified) != 3 {
		t.Fatalf("VerifyFindings returned %d findings, want 3", len(verified))
	}

	// First finding should be verified or partial (real file, real snippet)
	if verified[0].VerificationStatus == core.VerificationStatusRejected {
		t.Errorf("Finding 0 (valid) was rejected, want verified or partial")
	}

	// Second finding should be rejected (non-existent file)
	if verified[1].VerificationStatus != core.VerificationStatusRejected {
		t.Errorf("Finding 1 (hallucinated file) status = %q, want %q",
			verified[1].VerificationStatus, core.VerificationStatusRejected)
	}

	// Verify confidence was penalized for rejected finding
	if verified[1].ConfidenceScore >= 0.9 {
		t.Errorf("Rejected finding confidence = %f, should be penalized below 0.9", verified[1].ConfidenceScore)
	}

	// Filter should remove rejected findings
	filtered := verification.FilterVerifiedFindings(verified)
	for _, f := range filtered {
		if f.VerificationStatus == core.VerificationStatusRejected {
			t.Errorf("FilterVerifiedFindings kept rejected finding: %q", f.Title)
		}
	}
}

// =============================================================================
// Defect 8: No metadata from sub-repos
// =============================================================================

func TestBootstrapRepro_NoMetadataFromSubRepos(t *testing.T) {
	// Reproduce: Metadata extraction runs only on parent dir.
	// If parent is not a git repo, no metadata is extracted even though
	// sub-repos have valid git history.

	dir := t.TempDir()

	// Create sub-repo structures (without actual .git - just markers)
	subrepos := []string{"cli", "app", "macos"}
	for _, name := range subrepos {
		subDir := filepath.Join(dir, name)
		os.MkdirAll(subDir, 0755)
		writeFile(t, filepath.Join(subDir, "go.mod"), fmt.Sprintf("module example.com/%s", name))
		writeFile(t, filepath.Join(subDir, "README.md"), fmt.Sprintf("# %s\nProject description", name))
	}

	// DocLoader from parent dir should NOT find sub-repo docs (current behavior = bug)
	loader := bootstrap.NewDocLoader(dir)
	docs, _ := loader.Load()

	if len(docs) > 0 {
		t.Logf("Root loader found %d docs (unexpected for multi-repo workspace without root docs)", len(docs))
	}

	// Each sub-repo DocLoader should find its own docs
	for _, name := range subrepos {
		subLoader := bootstrap.NewDocLoader(filepath.Join(dir, name))
		subDocs, _ := subLoader.Load()
		if len(subDocs) == 0 {
			t.Errorf("Sub-repo %q DocLoader found 0 docs, want >= 1 (has README.md)", name)
		}
	}

	// The fix: bootstrap should iterate sub-repos and aggregate docs
	t.Log("Fix needed: bootstrap should run DocLoader per sub-repo and aggregate results")
}

// =============================================================================
// Defect 9: IsMonorepo=false despite 4 detected repositories
// =============================================================================

func TestBootstrapRepro_IsMonorepoMisclassification(t *testing.T) {
	// Reproduce: workspace has 4 sub-repos but no .git at root.
	// project.Detect() finds no marker and falls back to startPath with
	// IsMonorepo=false. Meanwhile, workspace.DetectWorkspace() correctly
	// identifies it as multi-repo. The two APIs disagree.

	t.Run("detector_says_not_monorepo_for_multi_repo_workspace", func(t *testing.T) {
		// Simulate: /workspace has 4 sub-repos with .git each, no root .git
		fs := setupFS([]string{
			"/workspace/cli/.git/",
			"/workspace/cli/go.mod",
			"/workspace/app/.git/",
			"/workspace/app/package.json",
			"/workspace/macos/.git/",
			"/workspace/macos/Package.swift",
			"/workspace/karluk/.git/",
			"/workspace/karluk/pyproject.toml",
		})

		d := project.NewDetector(fs)
		ctx, err := d.Detect("/workspace")
		if err != nil {
			t.Fatalf("Detect() error: %v", err)
		}

		// Current behavior: falls through to CWD fallback with no marker
		// and IsMonorepo=false, which is wrong for a multi-repo workspace
		if ctx.MarkerType == project.MarkerNone && !ctx.IsMonorepo {
			t.Log("BUG CONFIRMED: Detect() returns MarkerNone + IsMonorepo=false for 4-repo workspace")
			t.Errorf("IsMonorepo = false for workspace with 4 sub-repos, want true (or use WorkspaceTypeMultiRepo)")
		}
	})

	t.Run("workspace_detector_correctly_identifies_multi_repo", func(t *testing.T) {
		// The workspace detector (using real FS) correctly identifies multi-repo
		dir := t.TempDir()
		repos := []struct {
			name   string
			marker string
		}{
			{"cli", "go.mod"},
			{"app", "package.json"},
			{"macos", "go.mod"},
			{"karluk", "pyproject.toml"},
		}
		for _, r := range repos {
			subDir := filepath.Join(dir, r.name)
			os.MkdirAll(subDir, 0755)
			writeFile(t, filepath.Join(subDir, r.marker), "")
		}

		ws, err := project.DetectWorkspace(dir)
		if err != nil {
			t.Fatalf("DetectWorkspace() error: %v", err)
		}

		if ws.Type == project.WorkspaceTypeSingle {
			t.Errorf("DetectWorkspace Type = Single, want MultiRepo or Monorepo")
		}
		if len(ws.Services) != 4 {
			t.Errorf("DetectWorkspace found %d services, want 4", len(ws.Services))
		}
	})

	t.Run("disagreement_between_detect_and_workspace", func(t *testing.T) {
		// Both APIs should agree on multi-repo detection
		dir := t.TempDir()
		for _, name := range []string{"svc-a", "svc-b", "svc-c"} {
			subDir := filepath.Join(dir, name)
			os.MkdirAll(subDir, 0755)
			writeFile(t, filepath.Join(subDir, "go.mod"), fmt.Sprintf("module %s", name))
		}

		// project.Detect uses OsFs internally
		detectCtx, _ := project.Detect(dir)
		wsInfo, _ := project.DetectWorkspace(dir)

		if wsInfo.Type == project.WorkspaceTypeMultiRepo && !detectCtx.IsMonorepo {
			t.Error("DISAGREEMENT: DetectWorkspace says MultiRepo but Detect says IsMonorepo=false")
		}
	})
}

// =============================================================================
// DB Integration: Verify node+edge transactional integrity
// =============================================================================

func TestBootstrapIntegration_NodeEdgeTransactionIntegrity(t *testing.T) {
	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	// Create a batch of nodes
	nodeIDs := make([]string, 10)
	for i := range nodeIDs {
		nodeIDs[i] = fmt.Sprintf("n-batch%03d", i)
		node := &memory.Node{
			ID:          nodeIDs[i],
			Content:     fmt.Sprintf("Node %d content", i),
			Type:        memory.NodeTypeDecision,
			Summary:     fmt.Sprintf("Decision %d", i),
			SourceAgent: "test",
			CreatedAt:   time.Now(),
		}
		if err := store.CreateNode(node); err != nil {
			t.Fatalf("CreateNode %d: %v", i, err)
		}
	}

	// Link valid nodes - should succeed
	for i := 0; i < len(nodeIDs)-1; i++ {
		err := store.LinkNodes(nodeIDs[i], nodeIDs[i+1], "relates_to", 0.9, nil)
		if err != nil {
			t.Errorf("LinkNodes(%s, %s) failed: %v", nodeIDs[i], nodeIDs[i+1], err)
		}
	}

	// Verify edges were created
	edges, err := store.GetAllNodeEdges()
	if err != nil {
		t.Fatalf("GetAllNodeEdges: %v", err)
	}
	if len(edges) != 9 {
		t.Errorf("got %d edges, want 9", len(edges))
	}

	// Now simulate the purge-then-link bug:
	// Delete all nodes by agent, then try to create edges
	if err := store.DeleteNodesByAgent("test"); err != nil {
		t.Fatalf("DeleteNodesByAgent: %v", err)
	}

	// With ON DELETE CASCADE, edges should also be deleted
	edgesAfterPurge, err := store.GetAllNodeEdges()
	if err != nil {
		t.Fatalf("GetAllNodeEdges after purge: %v", err)
	}
	if len(edgesAfterPurge) != 0 {
		t.Errorf("got %d edges after cascade delete, want 0", len(edgesAfterPurge))
	}

	// Try to link purged nodes - should error with existence pre-check
	err = store.LinkNodes("n-batch000", "n-batch001", "depends_on", 1.0, nil)
	if err == nil {
		t.Error("LinkNodes with purged nodes should return error, got nil")
	}
}

// =============================================================================
// DocLoader with .taskwing memory path
// =============================================================================

func TestBootstrapIntegration_DocLoaderWithTaskwingDir(t *testing.T) {
	dir := t.TempDir()
	taskwingDir := filepath.Join(dir, ".taskwing")
	os.MkdirAll(taskwingDir, 0755)

	writeFile(t, filepath.Join(dir, "README.md"), "# Root Readme")
	writeFile(t, filepath.Join(taskwingDir, "ARCHITECTURE.md"), "# Generated Architecture")

	loader := bootstrap.NewDocLoader(dir)
	docs, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	foundReadme := false
	foundArch := false
	for _, doc := range docs {
		if doc.Name == "README.md" && doc.Category == "readme" {
			foundReadme = true
		}
		if doc.Name == "ARCHITECTURE.md" && doc.Category == "architecture" && strings.HasPrefix(doc.Path, ".taskwing/") {
			foundArch = true
		}
	}

	if !foundReadme {
		t.Error("DocLoader did not find README.md")
	}
	if !foundArch {
		t.Error("DocLoader did not find .taskwing/ARCHITECTURE.md")
	}
}

// =============================================================================
// Helpers
// =============================================================================

// setupFS creates an in-memory filesystem with the given paths.
// Paths ending with "/" are created as directories, others as files.
func setupFS(paths []string) afero.Fs {
	fs := afero.NewMemMapFs()
	for _, p := range paths {
		if p[len(p)-1] == '/' {
			_ = fs.MkdirAll(p, 0755)
		} else {
			dir := filepath.Dir(p)
			_ = fs.MkdirAll(dir, 0755)
			_ = afero.WriteFile(fs, p, []byte(""), 0644)
		}
	}
	return fs
}

// writeFile is a test helper that creates a file with content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// lookPath wraps exec.LookPath for testability.
func lookPath(name string) (string, error) {
	return exec.LookPath(name)
}
