package project

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupFS creates an in-memory filesystem and the specified directory structure.
// dirs should be absolute paths. Files can be specified as paths ending with a filename.
func setupFS(t *testing.T, dirs []string, files []string) afero.Fs {
	t.Helper()
	fs := afero.NewMemMapFs()

	for _, dir := range dirs {
		err := fs.MkdirAll(dir, 0755)
		require.NoError(t, err, "failed to create dir: %s", dir)
	}

	for _, file := range files {
		// Create parent directory if needed
		err := afero.WriteFile(fs, file, []byte{}, 0644)
		require.NoError(t, err, "failed to create file: %s", file)
	}

	return fs
}

func TestDetect_StandardRepo(t *testing.T) {
	// Scenario 1: Standard Repo - Root equals GitRoot
	// Structure:
	//   /project/.git/
	//   /project/go.mod
	//   /project/src/main.go

	fs := setupFS(t,
		[]string{"/project/.git", "/project/src"},
		[]string{"/project/go.mod", "/project/src/main.go"},
	)

	detector := NewDetector(fs)

	// Test from project root
	ctx, err := detector.Detect("/project")
	require.NoError(t, err)

	assert.Equal(t, "/project", ctx.RootPath)
	assert.Equal(t, MarkerGoMod, ctx.MarkerType)
	assert.Equal(t, "/project", ctx.GitRoot)
	assert.False(t, ctx.IsMonorepo)

	// Test from subdirectory - should bubble up to project root
	ctx, err = detector.Detect("/project/src")
	require.NoError(t, err)

	assert.Equal(t, "/project", ctx.RootPath)
	assert.Equal(t, MarkerGoMod, ctx.MarkerType)
	assert.Equal(t, "/project", ctx.GitRoot)
	assert.False(t, ctx.IsMonorepo)
}

func TestDetect_MonorepoSubdir(t *testing.T) {
	// Scenario 2: Monorepo Subdir - Start in subdir, find root at parent via .taskwing
	// Structure:
	//   /monorepo/.git/
	//   /monorepo/.taskwing/     <- Parent has .taskwing
	//   /monorepo/packages/
	//   /monorepo/packages/backend/
	//   /monorepo/packages/backend/go.mod  <- Child has go.mod

	fs := setupFS(t,
		[]string{
			"/monorepo/.git",
			"/monorepo/.taskwing",
			"/monorepo/packages/backend",
		},
		[]string{
			"/monorepo/packages/backend/go.mod",
		},
	)

	detector := NewDetector(fs)

	// Start from backend subdirectory
	ctx, err := detector.Detect("/monorepo/packages/backend")
	require.NoError(t, err)

	// Should find .taskwing at parent, not go.mod in current dir
	assert.Equal(t, "/monorepo", ctx.RootPath)
	assert.Equal(t, MarkerTaskWing, ctx.MarkerType)
	assert.Equal(t, "/monorepo", ctx.GitRoot)
	assert.False(t, ctx.IsMonorepo) // GitRoot == RootPath
}

func TestDetect_NestedOverride(t *testing.T) {
	// Scenario 3: Nested Override - Start in subdir, find root in subdir via .taskwing
	// Structure:
	//   /parent/.git/
	//   /parent/.taskwing/       <- Parent has .taskwing
	//   /parent/child/
	//   /parent/child/.taskwing/ <- Child ALSO has .taskwing (should win)
	//   /parent/child/go.mod

	fs := setupFS(t,
		[]string{
			"/parent/.git",
			"/parent/.taskwing",
			"/parent/child/.taskwing",
		},
		[]string{
			"/parent/child/go.mod",
		},
	)

	detector := NewDetector(fs)

	// Start from child directory
	ctx, err := detector.Detect("/parent/child")
	require.NoError(t, err)

	// Child's .taskwing should win (found first during bubble-up)
	assert.Equal(t, "/parent/child", ctx.RootPath)
	assert.Equal(t, MarkerTaskWing, ctx.MarkerType)
	assert.Equal(t, "/parent", ctx.GitRoot)
	assert.True(t, ctx.IsMonorepo) // GitRoot != RootPath
}

func TestDetect_PolyglotGoMod(t *testing.T) {
	// Scenario 4a: Polyglot - Identify root via go.mod in absence of .taskwing
	// Structure:
	//   /project/.git/
	//   /project/go.mod
	//   /project/internal/handler/

	fs := setupFS(t,
		[]string{
			"/project/.git",
			"/project/internal/handler",
		},
		[]string{
			"/project/go.mod",
		},
	)

	detector := NewDetector(fs)

	ctx, err := detector.Detect("/project/internal/handler")
	require.NoError(t, err)

	assert.Equal(t, "/project", ctx.RootPath)
	assert.Equal(t, MarkerGoMod, ctx.MarkerType)
	assert.Equal(t, "/project", ctx.GitRoot)
	assert.False(t, ctx.IsMonorepo)
}

func TestDetect_PolyglotPackageJSON(t *testing.T) {
	// Scenario 4b: Polyglot - Identify root via package.json in absence of .taskwing
	// Structure:
	//   /webapp/.git/
	//   /webapp/package.json
	//   /webapp/src/components/

	fs := setupFS(t,
		[]string{
			"/webapp/.git",
			"/webapp/src/components",
		},
		[]string{
			"/webapp/package.json",
		},
	)

	detector := NewDetector(fs)

	ctx, err := detector.Detect("/webapp/src/components")
	require.NoError(t, err)

	assert.Equal(t, "/webapp", ctx.RootPath)
	assert.Equal(t, MarkerPackageJSON, ctx.MarkerType)
	assert.Equal(t, "/webapp", ctx.GitRoot)
	assert.False(t, ctx.IsMonorepo)
}

func TestDetect_PolyglotCargoToml(t *testing.T) {
	// Scenario 4c: Polyglot - Identify root via Cargo.toml
	// Structure:
	//   /rustproject/.git/
	//   /rustproject/Cargo.toml
	//   /rustproject/src/

	fs := setupFS(t,
		[]string{
			"/rustproject/.git",
			"/rustproject/src",
		},
		[]string{
			"/rustproject/Cargo.toml",
		},
	)

	detector := NewDetector(fs)

	ctx, err := detector.Detect("/rustproject/src")
	require.NoError(t, err)

	assert.Equal(t, "/rustproject", ctx.RootPath)
	assert.Equal(t, MarkerCargoToml, ctx.MarkerType)
	assert.Equal(t, "/rustproject", ctx.GitRoot)
	assert.False(t, ctx.IsMonorepo)
}

func TestDetect_GitOnlyFallback(t *testing.T) {
	// No language manifest, only .git - should use git root as fallback
	// Structure:
	//   /repo/.git/
	//   /repo/docs/readme.md

	fs := setupFS(t,
		[]string{
			"/repo/.git",
			"/repo/docs",
		},
		[]string{
			"/repo/docs/readme.md",
		},
	)

	detector := NewDetector(fs)

	ctx, err := detector.Detect("/repo/docs")
	require.NoError(t, err)

	assert.Equal(t, "/repo", ctx.RootPath)
	assert.Equal(t, MarkerGit, ctx.MarkerType)
	assert.Equal(t, "/repo", ctx.GitRoot)
	assert.False(t, ctx.IsMonorepo)
}

func TestDetect_NoMarkersFound(t *testing.T) {
	// No markers at all - should return startPath with MarkerNone
	// Structure:
	//   /random/folder/

	fs := setupFS(t,
		[]string{"/random/folder"},
		nil,
	)

	detector := NewDetector(fs)

	ctx, err := detector.Detect("/random/folder")
	require.NoError(t, err)

	assert.Equal(t, "/random/folder", ctx.RootPath)
	assert.Equal(t, MarkerNone, ctx.MarkerType)
	assert.Empty(t, ctx.GitRoot)
	assert.False(t, ctx.IsMonorepo)
}

func TestDetect_TaskWingStopsTraversal(t *testing.T) {
	// .taskwing should stop traversal immediately, ignoring markers above
	// Structure:
	//   /workspace/.git/
	//   /workspace/.taskwing/           <- Higher up, should be ignored
	//   /workspace/project/.taskwing/   <- Found first, stops traversal
	//   /workspace/project/go.mod

	fs := setupFS(t,
		[]string{
			"/workspace/.git",
			"/workspace/.taskwing",
			"/workspace/project/.taskwing",
		},
		[]string{
			"/workspace/project/go.mod",
		},
	)

	detector := NewDetector(fs)

	ctx, err := detector.Detect("/workspace/project")
	require.NoError(t, err)

	// Should stop at /workspace/project/.taskwing
	assert.Equal(t, "/workspace/project", ctx.RootPath)
	assert.Equal(t, MarkerTaskWing, ctx.MarkerType)
}

func TestDetect_MonorepoDetection(t *testing.T) {
	// IsMonorepo should be true when GitRoot differs from RootPath
	// Structure:
	//   /monorepo/.git/
	//   /monorepo/services/api/go.mod  <- Project root
	//   /monorepo/services/api/cmd/

	fs := setupFS(t,
		[]string{
			"/monorepo/.git",
			"/monorepo/services/api/cmd",
		},
		[]string{
			"/monorepo/services/api/go.mod",
		},
	)

	detector := NewDetector(fs)

	ctx, err := detector.Detect("/monorepo/services/api/cmd")
	require.NoError(t, err)

	assert.Equal(t, "/monorepo/services/api", ctx.RootPath)
	assert.Equal(t, MarkerGoMod, ctx.MarkerType)
	assert.Equal(t, "/monorepo", ctx.GitRoot)
	assert.True(t, ctx.IsMonorepo)
}

func TestContext_RelativeGitPath(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *Context
		expected string
	}{
		{
			name:     "same root",
			ctx:      &Context{RootPath: "/project", GitRoot: "/project"},
			expected: ".",
		},
		{
			name:     "nested path",
			ctx:      &Context{RootPath: "/monorepo/packages/api", GitRoot: "/monorepo"},
			expected: "packages/api",
		},
		{
			name:     "empty git root",
			ctx:      &Context{RootPath: "/project", GitRoot: ""},
			expected: ".",
		},
		{
			name:     "empty root path",
			ctx:      &Context{RootPath: "", GitRoot: "/project"},
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.RelativeGitPath()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestContext_HasTaskWingDir(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *Context
		expected bool
	}{
		{
			name:     "has taskwing",
			ctx:      &Context{MarkerType: MarkerTaskWing},
			expected: true,
		},
		{
			name:     "has go.mod",
			ctx:      &Context{MarkerType: MarkerGoMod},
			expected: false,
		},
		{
			name:     "has git",
			ctx:      &Context{MarkerType: MarkerGit},
			expected: false,
		},
		{
			name:     "no marker",
			ctx:      &Context{MarkerType: MarkerNone},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.HasTaskWingDir()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestMarkerType_String(t *testing.T) {
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
		{MarkerType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.marker.String()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestMarkerType_Priority(t *testing.T) {
	// TaskWing should have highest priority
	assert.Equal(t, 100, MarkerTaskWing.Priority())

	// Language manifests should have medium priority (all equal)
	assert.Equal(t, 50, MarkerGoMod.Priority())
	assert.Equal(t, 50, MarkerPackageJSON.Priority())
	assert.Equal(t, 50, MarkerCargoToml.Priority())
	assert.Equal(t, 50, MarkerPomXML.Priority())
	assert.Equal(t, 50, MarkerPyProjectToml.Priority())

	// Git should have low priority
	assert.Equal(t, 10, MarkerGit.Priority())

	// None should have zero priority
	assert.Equal(t, 0, MarkerNone.Priority())
}

func TestMarkerType_IsLanguageManifest(t *testing.T) {
	languageManifests := []MarkerType{
		MarkerGoMod,
		MarkerPackageJSON,
		MarkerCargoToml,
		MarkerPomXML,
		MarkerPyProjectToml,
	}

	for _, m := range languageManifests {
		assert.True(t, m.IsLanguageManifest(), "%s should be a language manifest", m)
	}

	nonManifests := []MarkerType{
		MarkerNone,
		MarkerTaskWing,
		MarkerGit,
	}

	for _, m := range nonManifests {
		assert.False(t, m.IsLanguageManifest(), "%s should not be a language manifest", m)
	}
}
