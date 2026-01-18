package task

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitVerifier detects actual file modifications using git.
// It compares self-reported file modifications against git diff results
// to catch cases where an agent lies about what files it modified.
type GitVerifier struct {
	repoRoot string
	timeout  time.Duration
}

// NewGitVerifier creates a verifier for the given repository root.
func NewGitVerifier(repoRoot string) *GitVerifier {
	return &GitVerifier{
		repoRoot: repoRoot,
		timeout:  5 * time.Second,
	}
}

// VerificationResult contains the comparison between reported and actual files.
type VerificationResult struct {
	ReportedFiles   []string `json:"reported_files"`            // What the agent claimed to modify
	ActualFiles     []string `json:"actual_files"`              // What git says was modified
	UnreportedFiles []string `json:"unreported_files,omitempty"` // In git but not reported (agent lied/forgot)
	OverReported    []string `json:"over_reported,omitempty"`    // Reported but not in git (agent hallucinated)
	IsVerified      bool     `json:"is_verified"`                // Whether git verification succeeded
	VerifyError     string   `json:"verify_error,omitempty"`     // Error message if verification failed
}

// GetActualModifications returns files modified according to git.
// Combines multiple sources to capture all modified files:
// 1. git status --porcelain (untracked, modified, staged in working directory)
// 2. git diff --name-only HEAD~1 HEAD (changes in last commit, if agent committed)
func (v *GitVerifier) GetActualModifications(ctx context.Context) ([]string, error) {
	// Strategy 1: All working directory changes (untracked, modified, staged)
	// This catches files the agent is currently working on
	files, err := v.runGitStatus(ctx)
	if err != nil {
		return nil, err
	}

	// Strategy 2: Also check last commit (agent may have committed changes)
	committedFiles, err := v.runGitDiff(ctx, "HEAD~1", "HEAD")
	if err == nil {
		// Merge committed files into the set
		fileSet := make(map[string]bool)
		for _, f := range files {
			fileSet[f] = true
		}
		for _, f := range committedFiles {
			if !fileSet[f] {
				files = append(files, f)
				fileSet[f] = true
			}
		}
	}
	// Ignore errors from HEAD~1 (might not exist for new repos)

	return files, nil
}

// runGitDiff executes git diff --name-only with given args.
func (v *GitVerifier) runGitDiff(ctx context.Context, args ...string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	cmdArgs := append([]string{"diff", "--name-only"}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = v.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return v.parseFileList(string(output)), nil
}

// runGitStatus gets modified files from git status (for repos without commits).
func (v *GitVerifier) runGitStatus(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = v.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if len(line) < 4 {
			continue
		}
		// Format: "XY filename" where XY is status (2 chars) + space
		file := strings.TrimSpace(line[3:])
		if file != "" {
			files = append(files, normalizePath(file))
		}
	}
	return files, nil
}

// parseFileList parses git output into normalized file paths.
func (v *GitVerifier) parseFileList(output string) []string {
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, normalizePath(line))
		}
	}
	return files
}

// Verify compares reported files against actual git modifications.
func (v *GitVerifier) Verify(ctx context.Context, reported []string) *VerificationResult {
	result := &VerificationResult{
		ReportedFiles: reported,
	}

	actual, err := v.GetActualModifications(ctx)
	if err != nil {
		result.IsVerified = false
		result.VerifyError = err.Error()
		return result
	}

	result.ActualFiles = actual
	result.IsVerified = true

	// Build sets for comparison
	reportedSet := make(map[string]bool)
	for _, f := range reported {
		reportedSet[normalizePath(f)] = true
	}

	actualSet := make(map[string]bool)
	for _, f := range actual {
		actualSet[normalizePath(f)] = true
	}

	// Find unreported: in git but not reported (agent lied or forgot)
	for f := range actualSet {
		if !reportedSet[f] {
			result.UnreportedFiles = append(result.UnreportedFiles, f)
		}
	}

	// Find over-reported: reported but not in git (agent hallucinated)
	for f := range reportedSet {
		if !actualSet[f] {
			result.OverReported = append(result.OverReported, f)
		}
	}

	return result
}

// VerifyWithBaseline compares reported files against git modifications,
// excluding files that were already modified before the task started.
// This prevents false positives from pre-existing uncommitted changes.
func (v *GitVerifier) VerifyWithBaseline(ctx context.Context, reported, baseline []string) *VerificationResult {
	result := &VerificationResult{
		ReportedFiles: reported,
	}

	actual, err := v.GetActualModifications(ctx)
	if err != nil {
		result.IsVerified = false
		result.VerifyError = err.Error()
		return result
	}

	result.ActualFiles = actual
	result.IsVerified = true

	// Build sets for comparison
	reportedSet := make(map[string]bool)
	for _, f := range reported {
		reportedSet[normalizePath(f)] = true
	}

	actualSet := make(map[string]bool)
	for _, f := range actual {
		actualSet[normalizePath(f)] = true
	}

	baselineSet := make(map[string]bool)
	for _, f := range baseline {
		baselineSet[normalizePath(f)] = true
	}

	// Find unreported: in git but not reported AND not in baseline
	// Files in baseline were already modified before task started - not the agent's fault
	for f := range actualSet {
		if !reportedSet[f] && !baselineSet[f] {
			result.UnreportedFiles = append(result.UnreportedFiles, f)
		}
	}

	// Find over-reported: reported but not in git (agent hallucinated)
	for f := range reportedSet {
		if !actualSet[f] {
			result.OverReported = append(result.OverReported, f)
		}
	}

	return result
}

// HasDiscrepancy returns true if there are unreported or over-reported files.
func (r *VerificationResult) HasDiscrepancy() bool {
	return len(r.UnreportedFiles) > 0 || len(r.OverReported) > 0
}

// HasUnreportedHighRisk returns true if any unreported file is high-risk.
func (r *VerificationResult) HasUnreportedHighRisk() bool {
	for _, f := range r.UnreportedFiles {
		if isHighRiskFile(f) {
			return true
		}
	}
	return false
}

// IsGitRepo checks if the given path is inside a git repository.
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	err := cmd.Run()
	return err == nil
}

// FindRepoRoot finds the root of the git repository containing the given path.
func FindRepoRoot(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = absPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}
