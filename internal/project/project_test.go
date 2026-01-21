package project

import (
	"testing"

	"github.com/spf13/afero"
)

func TestMarkerTypeString(t *testing.T) {
	tests := []struct {
		marker   MarkerType
		expected string
	}{
		{MarkerNone, "none"},
		{MarkerTaskWing, ".taskwing"},
		{MarkerGoMod, "go.mod"},
		{MarkerPackageJSON, "package.json"},
		{MarkerCargoToml, "Cargo.toml"},
		{MarkerPomXML, "pom.xml"},
		{MarkerPyProjectToml, "pyproject.toml"},
		{MarkerGit, ".git"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.marker.String(); got != tt.expected {
				t.Errorf("MarkerType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMarkerTypePriority(t *testing.T) {
	// TaskWing should have highest priority
	if MarkerTaskWing.Priority() <= MarkerGoMod.Priority() {
		t.Error("MarkerTaskWing should have higher priority than MarkerGoMod")
	}

	// Language manifests should have higher priority than Git
	if MarkerGoMod.Priority() <= MarkerGit.Priority() {
		t.Error("MarkerGoMod should have higher priority than MarkerGit")
	}

	// Git should have higher priority than None
	if MarkerGit.Priority() <= MarkerNone.Priority() {
		t.Error("MarkerGit should have higher priority than MarkerNone")
	}
}

func TestMarkerTypeIsLanguageManifest(t *testing.T) {
	languageManifests := []MarkerType{
		MarkerGoMod,
		MarkerPackageJSON,
		MarkerCargoToml,
		MarkerPomXML,
		MarkerPyProjectToml,
	}

	for _, m := range languageManifests {
		if !m.IsLanguageManifest() {
			t.Errorf("%s should be a language manifest", m.String())
		}
	}

	nonManifests := []MarkerType{
		MarkerNone,
		MarkerTaskWing,
		MarkerGit,
	}

	for _, m := range nonManifests {
		if m.IsLanguageManifest() {
			t.Errorf("%s should not be a language manifest", m.String())
		}
	}
}

func TestDetectWithGoMod(t *testing.T) {
	// Create in-memory filesystem
	fs := afero.NewMemMapFs()

	// Create a directory structure with go.mod
	_ = fs.MkdirAll("/project/subdir", 0755)
	_ = afero.WriteFile(fs, "/project/go.mod", []byte("module test"), 0644)

	detector := NewDetector(fs)

	// Detect from subdir should find go.mod in parent
	ctx, err := detector.Detect("/project/subdir")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if ctx.RootPath != "/project" {
		t.Errorf("RootPath = %v, want /project", ctx.RootPath)
	}

	if ctx.MarkerType != MarkerGoMod {
		t.Errorf("MarkerType = %v, want MarkerGoMod", ctx.MarkerType)
	}
}

func TestDetectWithTaskWing(t *testing.T) {
	// Create in-memory filesystem
	fs := afero.NewMemMapFs()

	// Create a directory structure with both .taskwing and go.mod
	// .taskwing should take precedence
	_ = fs.MkdirAll("/project/.taskwing", 0755)
	_ = afero.WriteFile(fs, "/project/go.mod", []byte("module test"), 0644)

	detector := NewDetector(fs)

	ctx, err := detector.Detect("/project")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if ctx.MarkerType != MarkerTaskWing {
		t.Errorf("MarkerType = %v, want MarkerTaskWing (should have highest priority)", ctx.MarkerType)
	}
}

func TestDetectWithGit(t *testing.T) {
	// Create in-memory filesystem
	fs := afero.NewMemMapFs()

	// Create a directory structure with only .git
	_ = fs.MkdirAll("/project/.git", 0755)

	detector := NewDetector(fs)

	ctx, err := detector.Detect("/project")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if ctx.MarkerType != MarkerGit {
		t.Errorf("MarkerType = %v, want MarkerGit", ctx.MarkerType)
	}

	if ctx.GitRoot != "/project" {
		t.Errorf("GitRoot = %v, want /project", ctx.GitRoot)
	}
}

func TestContextRelativeGitPath(t *testing.T) {
	tests := []struct {
		name     string
		ctx      Context
		expected string
	}{
		{
			name:     "same path",
			ctx:      Context{RootPath: "/project", GitRoot: "/project"},
			expected: ".",
		},
		{
			name:     "subdir of git root",
			ctx:      Context{RootPath: "/project/packages/api", GitRoot: "/project"},
			expected: "packages/api",
		},
		{
			name:     "empty git root",
			ctx:      Context{RootPath: "/project", GitRoot: ""},
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.RelativeGitPath(); got != tt.expected {
				t.Errorf("RelativeGitPath() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWorkspaceTypeString(t *testing.T) {
	tests := []struct {
		wsType   WorkspaceType
		expected string
	}{
		{WorkspaceTypeSingle, "single"},
		{WorkspaceTypeMonorepo, "monorepo"},
		{WorkspaceTypeMultiRepo, "multi-repo"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.wsType.String(); got != tt.expected {
				t.Errorf("WorkspaceType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWorkspaceInfoMethods(t *testing.T) {
	info := &WorkspaceInfo{
		Type:     WorkspaceTypeMonorepo,
		RootPath: "/project",
		Services: []string{"api", "web", "common"},
		Name:     "myproject",
	}

	if info.IsMultiRepo() {
		t.Error("Monorepo should not be reported as multi-repo")
	}

	if info.ServiceCount() != 3 {
		t.Errorf("ServiceCount() = %d, want 3", info.ServiceCount())
	}

	expectedPath := "/project/api"
	if got := info.GetServicePath("api"); got != expectedPath {
		t.Errorf("GetServicePath() = %v, want %v", got, expectedPath)
	}
}

// === Workspace Auto-Detection Tests ===

func TestExtractWorkspaceName(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		expected string
	}{
		{"simple name", "osprey", "osprey"},
		{"nested path", "services/osprey", "osprey"},
		{"deeply nested", "apps/frontend/web", "web"},
		{"empty path", "", ""},
		{"dot path", ".", ""},
		{"root slash", "/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractWorkspaceName(tt.relPath)
			if result != tt.expected {
				t.Errorf("extractWorkspaceName(%q) = %q, want %q", tt.relPath, result, tt.expected)
			}
		})
	}
}

func TestDetectWorkspaceFromPath_SingleRepo(t *testing.T) {
	// For a single repo (non-monorepo), should always return "root"
	// Use a path that we know won't be detected as a monorepo
	workspace, err := DetectWorkspaceFromPath("/nonexistent/path")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if workspace != "root" {
		t.Errorf("DetectWorkspaceFromPath for nonexistent path = %q, want 'root'", workspace)
	}
}

func TestAutoDetectWorkspace_FallbackToRoot(t *testing.T) {
	// AutoDetectWorkspace should never return empty string
	workspace, _ := DetectWorkspaceFromCwd()
	// We can't predict the cwd, but it should either return "root" or a valid workspace name
	if workspace == "" {
		t.Error("DetectWorkspaceFromCwd returned empty string, expected 'root' or workspace name")
	}
}

func TestDetectWorkspaceFromPath_WithMonorepoContext(t *testing.T) {
	// This test documents the expected behavior for monorepo detection
	// The actual detection depends on the file system structure
	// We test the edge cases here

	tests := []struct {
		name            string
		relPath         string
		isMonorepo      bool
		expectedDefault string
	}{
		{"at root", ".", false, "root"},
		{"empty", "", false, "root"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For non-monorepo or at root, should return "root"
			ctx := &Context{
				RootPath:   "/project",
				GitRoot:    "/project",
				IsMonorepo: tt.isMonorepo,
			}

			relPath := ctx.RelativeGitPath()
			workspace := extractWorkspaceName(relPath)

			// At root, RelativeGitPath returns ".", extractWorkspaceName returns ""
			// which means we should fallback to "root"
			if relPath == "." && workspace == "" {
				workspace = "root"
			}

			if workspace != tt.expectedDefault {
				t.Errorf("workspace = %q, want %q", workspace, tt.expectedDefault)
			}
		})
	}
}
