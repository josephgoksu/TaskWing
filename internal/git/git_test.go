package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mockCommander implements Commander for deterministic testing.
type mockCommander struct {
	responses map[string]struct {
		output string
		err    error
	}
}

func (m *mockCommander) Run(name string, args ...string) (string, error) {
	return m.RunInDir("", name, args...)
}

func (m *mockCommander) RunInDir(dir, name string, args ...string) (string, error) {
	key := fmt.Sprintf("%s:%s %s", dir, name, strings.Join(args, " "))
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}
	// Also try without dir prefix for convenience
	keyNoDir := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	if resp, ok := m.responses[keyNoDir]; ok {
		return resp.output, resp.err
	}
	return "", fmt.Errorf("exit status 128: fatal: not a git repository")
}

// =============================================================================
// TestGitWrapper_Exit128: Verify Client handles exit 128 gracefully
// =============================================================================

func TestGitWrapper_Exit128(t *testing.T) {
	tests := []struct {
		name       string
		workDir    string
		responses  map[string]struct{ output string; err error }
		wantIsRepo bool
	}{
		{
			name:    "non-git directory returns false",
			workDir: "/workspace/not-a-repo",
			responses: map[string]struct{ output string; err error }{
				"/workspace/not-a-repo:git rev-parse --is-inside-work-tree": {
					err: fmt.Errorf("exit status 128: fatal: not a git repository"),
				},
			},
			wantIsRepo: false,
		},
		{
			name:    "valid git repo returns true",
			workDir: "/workspace/valid-repo",
			responses: map[string]struct{ output string; err error }{
				"/workspace/valid-repo:git rev-parse --is-inside-work-tree": {
					output: "true",
				},
			},
			wantIsRepo: true,
		},
		{
			name:    "empty directory with no response returns false",
			workDir: "/tmp/empty",
			responses: map[string]struct{ output string; err error }{},
			wantIsRepo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &mockCommander{responses: tt.responses}
			client := NewClientWithCommander(tt.workDir, cmd)

			got := client.IsRepository()
			if got != tt.wantIsRepo {
				t.Errorf("IsRepository() = %v, want %v", got, tt.wantIsRepo)
			}
		})
	}
}

// =============================================================================
// TestIsGitRepository: Standalone function with real temp dirs
// =============================================================================

func TestIsGitRepository(t *testing.T) {
	// Skip if git is not installed
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	t.Run("non-git directory returns false", func(t *testing.T) {
		dir := t.TempDir()
		if IsGitRepository(dir) {
			t.Error("IsGitRepository() = true for non-git dir, want false")
		}
	})

	t.Run("git-inited directory returns true", func(t *testing.T) {
		dir := t.TempDir()
		cmd := exec.Command("git", "init", dir)
		if err := cmd.Run(); err != nil {
			t.Fatalf("git init: %v", err)
		}
		if !IsGitRepository(dir) {
			t.Error("IsGitRepository() = false for git-inited dir, want true")
		}
	})

	t.Run("nonexistent directory returns false", func(t *testing.T) {
		if IsGitRepository("/nonexistent/path/xyz123") {
			t.Error("IsGitRepository() = true for nonexistent path, want false")
		}
	})

	t.Run("subdirectory of git repo returns true", func(t *testing.T) {
		dir := t.TempDir()
		cmd := exec.Command("git", "init", dir)
		if err := cmd.Run(); err != nil {
			t.Fatalf("git init: %v", err)
		}
		subDir := filepath.Join(dir, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if !IsGitRepository(subDir) {
			t.Error("IsGitRepository() = false for subdir of git repo, want true")
		}
	})
}

// =============================================================================
// TestBootstrapContinueOnGitError: Verify bootstrap-like flow continues
// =============================================================================

func TestBootstrapContinueOnGitError(t *testing.T) {
	// Simulate a multi-repo workspace where some repos have git and some don't.
	// Bootstrap should continue processing remaining repos when one fails.

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Create 3 "repos": 2 with git, 1 without
	repos := []struct {
		name   string
		hasGit bool
	}{
		{"cli", true},
		{"app", true},
		{"macos", false}, // This simulates the exit 128 scenario
	}

	for _, r := range repos {
		repoDir := filepath.Join(tmpDir, r.name)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		if r.hasGit {
			cmd := exec.Command("git", "init", repoDir)
			if err := cmd.Run(); err != nil {
				t.Fatalf("git init %s: %v", r.name, err)
			}
			// Create a dummy commit so git log works
			dummyFile := filepath.Join(repoDir, "README.md")
			if err := os.WriteFile(dummyFile, []byte("# "+r.name), 0644); err != nil {
				t.Fatal(err)
			}
			addCmd := exec.Command("git", "-C", repoDir, "add", ".")
			addCmd.Run()
			commitCmd := exec.Command("git", "-C", repoDir, "commit", "-m", "init", "--allow-empty")
			commitCmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test",
				"GIT_AUTHOR_EMAIL=test@test.com",
				"GIT_COMMITTER_NAME=Test",
				"GIT_COMMITTER_EMAIL=test@test.com",
			)
			commitCmd.Run()
		}
	}

	// Simulate bootstrap: iterate repos, skip non-git ones
	var processed []string
	var skipped []string

	for _, r := range repos {
		repoDir := filepath.Join(tmpDir, r.name)
		if !IsGitRepository(repoDir) {
			skipped = append(skipped, r.name)
			continue
		}
		processed = append(processed, r.name)
	}

	if len(processed) != 2 {
		t.Errorf("processed %d repos, want 2: %v", len(processed), processed)
	}
	if len(skipped) != 1 || skipped[0] != "macos" {
		t.Errorf("skipped = %v, want [macos]", skipped)
	}
}
