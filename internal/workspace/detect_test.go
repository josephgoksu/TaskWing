package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_SingleRepo(t *testing.T) {
	// Create temp dir with .git
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	info, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if info.Type != TypeSingle {
		t.Errorf("Type = %v, want %v", info.Type, TypeSingle)
	}
	if len(info.Services) != 1 || info.Services[0] != "." {
		t.Errorf("Services = %v, want [.]", info.Services)
	}
}

func TestDetect_MultiRepo(t *testing.T) {
	// Create temp dir with nested repos (no root .git)
	dir := t.TempDir()

	// Create two "services" with their own .git dirs
	for _, name := range []string{"service-a", "service-b"} {
		svcDir := filepath.Join(dir, name)
		if err := os.Mkdir(svcDir, 0755); err != nil {
			t.Fatal(err)
		}
		gitDir := filepath.Join(svcDir, ".git")
		if err := os.Mkdir(gitDir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	info, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if info.Type != TypeMultiRepo {
		t.Errorf("Type = %v, want %v", info.Type, TypeMultiRepo)
	}
	if len(info.Services) != 2 {
		t.Errorf("Services count = %d, want 2", len(info.Services))
	}
}

func TestDetect_NoGit(t *testing.T) {
	// Create temp dir with no .git anywhere
	dir := t.TempDir()

	// Create a subdir (not a git repo)
	if err := os.Mkdir(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatal(err)
	}

	info, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	// Should fall back to single
	if info.Type != TypeSingle {
		t.Errorf("Type = %v, want %v", info.Type, TypeSingle)
	}
}

func TestInfo_GetServicePath(t *testing.T) {
	info := &Info{
		RootPath: "/workspace/tazama",
		Services: []string{"admin-service", "event-director"},
	}

	got := info.GetServicePath("admin-service")
	want := "/workspace/tazama/admin-service"
	if got != want {
		t.Errorf("GetServicePath() = %v, want %v", got, want)
	}
}
