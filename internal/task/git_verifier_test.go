package task

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGitVerifier_Verify_NoDiscrepancy tests when reported matches actual.
func TestGitVerifier_Verify_NoDiscrepancy(t *testing.T) {
	// Create a temp git repo
	dir := setupTestGitRepo(t)

	// Create and modify a file
	testFile := filepath.Join(dir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	verifier := NewGitVerifier(dir)
	result := verifier.Verify(context.Background(), []string{"test.go"})

	if !result.IsVerified {
		t.Errorf("expected verification to succeed, got error: %s", result.VerifyError)
	}

	if result.HasDiscrepancy() {
		t.Errorf("expected no discrepancy, got unreported=%v, over_reported=%v",
			result.UnreportedFiles, result.OverReported)
	}
}

// TestGitVerifier_Verify_UnreportedFiles tests detection of files agent didn't report.
func TestGitVerifier_Verify_UnreportedFiles(t *testing.T) {
	dir := setupTestGitRepo(t)

	// Create two files
	for _, name := range []string{"reported.go", "unreported.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package main"), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	verifier := NewGitVerifier(dir)
	// Agent only reports one file
	result := verifier.Verify(context.Background(), []string{"reported.go"})

	if !result.IsVerified {
		t.Fatalf("expected verification to succeed, got error: %s", result.VerifyError)
	}

	if len(result.UnreportedFiles) != 1 || result.UnreportedFiles[0] != "unreported.go" {
		t.Errorf("expected unreported.go in unreported files, got %v", result.UnreportedFiles)
	}

	if !result.HasDiscrepancy() {
		t.Error("expected HasDiscrepancy() to return true")
	}
}

// TestGitVerifier_Verify_OverReported tests detection of files agent claimed but didn't change.
func TestGitVerifier_Verify_OverReported(t *testing.T) {
	dir := setupTestGitRepo(t)

	// Create only one file
	if err := os.WriteFile(filepath.Join(dir, "actual.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create actual.go: %v", err)
	}

	verifier := NewGitVerifier(dir)
	// Agent claims to have modified more files than actually changed
	result := verifier.Verify(context.Background(), []string{"actual.go", "hallucinated.go"})

	if !result.IsVerified {
		t.Fatalf("expected verification to succeed, got error: %s", result.VerifyError)
	}

	if len(result.OverReported) != 1 || result.OverReported[0] != "hallucinated.go" {
		t.Errorf("expected hallucinated.go in over-reported files, got %v", result.OverReported)
	}

	if !result.HasDiscrepancy() {
		t.Error("expected HasDiscrepancy() to return true")
	}
}

// TestGitVerifier_Verify_NotGitRepo tests graceful handling of non-git directories.
func TestGitVerifier_Verify_NotGitRepo(t *testing.T) {
	// Create temp dir without git init
	dir, err := os.MkdirTemp("", "test-no-git-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	verifier := NewGitVerifier(dir)
	result := verifier.Verify(context.Background(), []string{"test.go"})

	if result.IsVerified {
		t.Error("expected verification to fail for non-git directory")
	}

	if result.VerifyError == "" {
		t.Error("expected error message for non-git directory")
	}
}

// TestGitVerifier_HasUnreportedHighRisk tests high-risk file detection.
func TestGitVerifier_HasUnreportedHighRisk(t *testing.T) {
	tests := []struct {
		name            string
		unreportedFiles []string
		expectHighRisk  bool
	}{
		{
			name:            "no unreported files",
			unreportedFiles: nil,
			expectHighRisk:  false,
		},
		{
			name:            "normal file",
			unreportedFiles: []string{"handler.go"},
			expectHighRisk:  false,
		},
		{
			name:            "config file",
			unreportedFiles: []string{"config/database.yaml"},
			expectHighRisk:  true,
		},
		{
			name:            "secrets file",
			unreportedFiles: []string{"internal/auth/secrets.go"},
			expectHighRisk:  true,
		},
		{
			name:            "env file",
			unreportedFiles: []string{".env.production"},
			expectHighRisk:  true,
		},
		{
			name:            "migration file",
			unreportedFiles: []string{"db/migrations/001_create_users.sql"},
			expectHighRisk:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &VerificationResult{
				UnreportedFiles: tt.unreportedFiles,
				IsVerified:      true,
			}

			got := result.HasUnreportedHighRisk()
			if got != tt.expectHighRisk {
				t.Errorf("HasUnreportedHighRisk() = %v, want %v", got, tt.expectHighRisk)
			}
		})
	}
}

// TestGitVerifier_VerifyWithBaseline_ExcludesBaselineFiles tests that pre-existing
// modified files (baseline) are not flagged as unreported.
func TestGitVerifier_VerifyWithBaseline_ExcludesBaselineFiles(t *testing.T) {
	dir := setupTestGitRepo(t)

	// Create two files: one in baseline, one new
	for _, name := range []string{"baseline.go", "new.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package main"), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	verifier := NewGitVerifier(dir)

	// Simulate: baseline.go was modified before task started
	// Agent only reports new.go (correctly)
	baseline := []string{"baseline.go"}
	reported := []string{"new.go"}

	result := verifier.VerifyWithBaseline(context.Background(), reported, baseline)

	if !result.IsVerified {
		t.Fatalf("expected verification to succeed, got error: %s", result.VerifyError)
	}

	// baseline.go should NOT be in unreported (it was in baseline)
	for _, f := range result.UnreportedFiles {
		if f == "baseline.go" {
			t.Error("baseline.go should not be flagged as unreported - it was in baseline")
		}
	}

	// Result should show no discrepancy (agent correctly reported new.go, baseline.go excluded)
	if result.HasDiscrepancy() {
		t.Errorf("expected no discrepancy when baseline is properly excluded, got unreported=%v, over_reported=%v",
			result.UnreportedFiles, result.OverReported)
	}
}

// TestGitVerifier_VerifyWithBaseline_StillCatchesUnreported tests that files not in
// baseline AND not reported are still flagged.
func TestGitVerifier_VerifyWithBaseline_StillCatchesUnreported(t *testing.T) {
	dir := setupTestGitRepo(t)

	// Create three files
	for _, name := range []string{"baseline.go", "reported.go", "sneaky.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package main"), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	verifier := NewGitVerifier(dir)

	// baseline.go was pre-existing, agent reports reported.go, but sneaky.go is hidden
	baseline := []string{"baseline.go"}
	reported := []string{"reported.go"}

	result := verifier.VerifyWithBaseline(context.Background(), reported, baseline)

	if !result.IsVerified {
		t.Fatalf("expected verification to succeed, got error: %s", result.VerifyError)
	}

	// sneaky.go should be flagged as unreported
	foundSneaky := false
	for _, f := range result.UnreportedFiles {
		if f == "sneaky.go" {
			foundSneaky = true
		}
	}

	if !foundSneaky {
		t.Errorf("sneaky.go should be flagged as unreported, got unreported=%v", result.UnreportedFiles)
	}
}

// TestIsGitRepo tests git repository detection.
func TestIsGitRepo(t *testing.T) {
	// Test with a real git repo
	gitDir := setupTestGitRepo(t)
	if !IsGitRepo(gitDir) {
		t.Error("expected IsGitRepo to return true for git repository")
	}

	// Test with a non-git directory
	nonGitDir, err := os.MkdirTemp("", "test-no-git-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(nonGitDir)

	if IsGitRepo(nonGitDir) {
		t.Error("expected IsGitRepo to return false for non-git directory")
	}
}

// setupTestGitRepo creates a temporary git repository for testing.
func setupTestGitRepo(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "test-git-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	cmd.Run()

	// Create initial commit so HEAD exists
	readmeFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = dir
	cmd.Run()

	return dir
}
