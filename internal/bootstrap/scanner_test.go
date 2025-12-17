package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScannerDetectsDirectoryFeaturesAndConventionalCommits(t *testing.T) {
	tmpDir := t.TempDir()

	// Directory structure feature: src/auth/*
	authDir := filepath.Join(tmpDir, "src", "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "auth.go"), []byte("package auth\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Git history decisions
	runGit(t, tmpDir, nil, "init")
	runGit(t, tmpDir, nil, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, nil, "config", "user.name", "Test User")
	runGit(t, tmpDir, nil, "config", "commit.gpgsign", "false")

	runGit(t, tmpDir, nil, "add", ".")
	runGit(t, tmpDir, nil, "commit", "-m", "chore: init")

	if err := os.WriteFile(filepath.Join(authDir, "jwt.go"), []byte("package auth\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, tmpDir, nil, "add", ".")
	runGit(t, tmpDir, nil, "commit", "-m", "feat(auth): add jwt support")

	scanner := NewScanner(tmpDir)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	foundAuthFeature := false
	for _, f := range result.Features {
		if f.Name == "Auth" {
			foundAuthFeature = true
			if f.FileCount == 0 {
				t.Fatalf("expected auth feature file count > 0")
			}
			break
		}
	}
	if !foundAuthFeature {
		t.Fatalf("expected feature 'Auth' in scan results, got: %+v", result.Features)
	}

	foundAuthDecision := false
	for _, d := range result.Decisions {
		if d.Feature == "Auth" && strings.Contains(d.Title, "Add:") {
			foundAuthDecision = true
			break
		}
	}
	if !foundAuthDecision {
		t.Fatalf("expected at least one decision for feature 'Auth', got: %+v", result.Decisions)
	}
}
