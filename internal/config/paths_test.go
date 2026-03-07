package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/project"
)

func TestRootPathResolution(t *testing.T) {
	// Save and restore global state
	origGetGlobalConfigDir := GetGlobalConfigDir
	t.Cleanup(func() {
		ClearProjectContext()
		GetGlobalConfigDir = origGetGlobalConfigDir
	})

	t.Run("project_with_taskwing_marker_resolves_correctly", func(t *testing.T) {
		ClearProjectContext()
		projectDir := t.TempDir()
		taskwingDir := filepath.Join(projectDir, ".taskwing")
		memoryDir := filepath.Join(taskwingDir, "memory")
		if err := os.MkdirAll(memoryDir, 0o755); err != nil {
			t.Fatal(err)
		}

		ctx := &project.Context{
			RootPath:   projectDir,
			MarkerType: project.MarkerTaskWing,
		}
		if err := SetProjectContext(ctx); err != nil {
			t.Fatal(err)
		}

		got, err := GetMemoryBasePath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != memoryDir {
			t.Errorf("GetMemoryBasePath() = %q, want %q", got, memoryDir)
		}
	})

	t.Run("project_with_git_marker_resolves_correctly", func(t *testing.T) {
		ClearProjectContext()
		projectDir := t.TempDir()
		// Create .git and .taskwing dirs
		if err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		taskwingDir := filepath.Join(projectDir, ".taskwing")
		memoryDir := filepath.Join(taskwingDir, "memory")
		if err := os.MkdirAll(memoryDir, 0o755); err != nil {
			t.Fatal(err)
		}

		ctx := &project.Context{
			RootPath:   projectDir,
			MarkerType: project.MarkerGit,
			GitRoot:    projectDir,
		}
		if err := SetProjectContext(ctx); err != nil {
			t.Fatal(err)
		}

		got, err := GetMemoryBasePath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != memoryDir {
			t.Errorf("GetMemoryBasePath() = %q, want %q", got, memoryDir)
		}
	})

	t.Run("marker_none_rejected_even_with_taskwing_dir", func(t *testing.T) {
		// Simulates running from HOME where ~/.taskwing exists
		ClearProjectContext()
		fakeHome := t.TempDir()
		taskwingDir := filepath.Join(fakeHome, ".taskwing")
		if err := os.MkdirAll(filepath.Join(taskwingDir, "memory"), 0o755); err != nil {
			t.Fatal(err)
		}

		ctx := &project.Context{
			RootPath:   fakeHome,
			MarkerType: project.MarkerNone, // CWD fallback
		}
		if err := SetProjectContext(ctx); err != nil {
			t.Fatal(err)
		}

		_, err := GetMemoryBasePath()
		if err == nil {
			t.Error("GetMemoryBasePath() should fail for MarkerNone context (CWD fallback to HOME)")
		}
	})

	t.Run("nil_context_returns_error", func(t *testing.T) {
		ClearProjectContext()

		_, err := GetMemoryBasePath()
		if err == nil {
			t.Error("GetMemoryBasePath() should fail when project context is nil")
		}
	})

	t.Run("global_fallback_only_via_GetMemoryBasePathOrGlobal", func(t *testing.T) {
		ClearProjectContext()
		fakeGlobal := t.TempDir()
		GetGlobalConfigDir = func() (string, error) {
			return fakeGlobal, nil
		}

		// GetMemoryBasePath should fail
		_, err := GetMemoryBasePath()
		if err == nil {
			t.Error("GetMemoryBasePath() should fail without project context")
		}

		// GetMemoryBasePathOrGlobal should fall back to global
		got, err := GetMemoryBasePathOrGlobal()
		if err != nil {
			t.Fatalf("GetMemoryBasePathOrGlobal() unexpected error: %v", err)
		}
		want := filepath.Join(fakeGlobal, "memory")
		if got != want {
			t.Errorf("GetMemoryBasePathOrGlobal() = %q, want %q", got, want)
		}
	})

	t.Run("no_accidental_writes_to_home", func(t *testing.T) {
		// Verify that a MarkerNone context pointing to HOME is rejected
		ClearProjectContext()
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot determine HOME")
		}

		ctx := &project.Context{
			RootPath:   home,
			MarkerType: project.MarkerNone,
		}
		if err := SetProjectContext(ctx); err != nil {
			t.Fatal(err)
		}

		_, err = GetMemoryBasePath()
		if err == nil {
			t.Error("GetMemoryBasePath() must reject HOME as RootPath when MarkerType is None")
		}
	})
}
