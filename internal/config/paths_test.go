package config

import (
	"errors"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/project"
)

func TestSetProjectContext_NilReturnsError(t *testing.T) {
	// Clear any existing context
	ClearProjectContext()

	err := SetProjectContext(nil)
	if err == nil {
		t.Fatal("expected error for nil context, got nil")
	}

	// Verify error message is helpful
	if err.Error() != "SetProjectContext called with nil context" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestSetProjectContext_ValidContext(t *testing.T) {
	// Clear any existing context
	ClearProjectContext()
	defer ClearProjectContext()

	ctx := &project.Context{
		RootPath:   "/test/path",
		MarkerType: project.MarkerGit,
	}

	err := SetProjectContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify context was set
	got := GetProjectContext()
	if got == nil {
		t.Fatal("expected context to be set")
	}
	if got.RootPath != ctx.RootPath {
		t.Errorf("expected RootPath %q, got %q", ctx.RootPath, got.RootPath)
	}
}

func TestGetProjectContextOrError_NotSet(t *testing.T) {
	ClearProjectContext()

	ctx, err := GetProjectContextOrError()
	if err == nil {
		t.Fatal("expected error when context not set")
	}
	if !errors.Is(err, ErrProjectContextNotSet) {
		t.Errorf("expected ErrProjectContextNotSet, got: %v", err)
	}
	if ctx != nil {
		t.Error("expected nil context")
	}
}

func TestGetProjectContextOrError_Set(t *testing.T) {
	ClearProjectContext()
	defer ClearProjectContext()

	expected := &project.Context{RootPath: "/test"}
	_ = SetProjectContext(expected)

	ctx, err := GetProjectContextOrError()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != expected {
		t.Error("context does not match expected")
	}
}

func TestGetProjectRoot_NotSet(t *testing.T) {
	ClearProjectContext()

	root, err := GetProjectRoot()
	if err == nil {
		t.Fatal("expected error when context not set")
	}
	if !errors.Is(err, ErrProjectContextNotSet) {
		t.Errorf("expected ErrProjectContextNotSet, got: %v", err)
	}
	if root != "" {
		t.Errorf("expected empty root, got: %s", root)
	}
}

func TestGetProjectRoot_EmptyRootPath(t *testing.T) {
	ClearProjectContext()
	defer ClearProjectContext()

	ctx := &project.Context{RootPath: ""}
	_ = SetProjectContext(ctx)

	root, err := GetProjectRoot()
	if err == nil {
		t.Fatal("expected error for empty RootPath")
	}
	if root != "" {
		t.Errorf("expected empty root, got: %s", root)
	}
}

func TestGetProjectRoot_Valid(t *testing.T) {
	ClearProjectContext()
	defer ClearProjectContext()

	expected := "/my/project"
	ctx := &project.Context{RootPath: expected}
	_ = SetProjectContext(ctx)

	root, err := GetProjectRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != expected {
		t.Errorf("expected %q, got %q", expected, root)
	}
}

func TestGetMemoryBasePath_NotSet(t *testing.T) {
	ClearProjectContext()

	path, err := GetMemoryBasePath()
	if err == nil {
		t.Fatal("expected error when context not set")
	}
	if !errors.Is(err, ErrProjectContextNotSet) {
		t.Errorf("expected ErrProjectContextNotSet, got: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path, got: %s", path)
	}
}

func TestGetMemoryBasePathOrGlobal_FallsBackToGlobal(t *testing.T) {
	ClearProjectContext()

	// Should fall back to global without error
	path, err := GetMemoryBasePathOrGlobal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
	// Should contain "memory" in the path
	if len(path) < 6 || path[len(path)-6:] != "memory" {
		t.Errorf("expected path to end with 'memory', got: %s", path)
	}
}

func TestGetMemoryBasePathOrGlobal_GlobalDirError(t *testing.T) {
	ClearProjectContext()

	// Save original function
	original := GetGlobalConfigDir
	defer func() { GetGlobalConfigDir = original }()

	// Mock to return error
	GetGlobalConfigDir = func() (string, error) {
		return "", errors.New("test error: cannot get home dir")
	}

	path, err := GetMemoryBasePathOrGlobal()
	if err == nil {
		t.Fatal("expected error when global config dir fails")
	}
	if path != "" {
		t.Errorf("expected empty path on error, got: %s", path)
	}
}
