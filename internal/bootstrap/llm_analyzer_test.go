package bootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, repoDir string, env []string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

// newTestAnalyzer creates an LLMAnalyzer for testing internal methods only.
// ChatModel (BaseChatModel) is nil since we're only testing git history parsing, not LLM calls.
func newTestAnalyzer(basePath string) *LLMAnalyzer {
	return &LLMAnalyzer{
		BasePath:  basePath,
		ChatModel: nil, // Not needed for git history tests
		Model:     "test-model",
	}
}

func TestSummarizeGitHistory_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	analyzer := newTestAnalyzer(tmpDir)

	summary, err := analyzer.summarizeGitHistory()
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary != "" {
		t.Fatalf("expected empty summary for non-git directory, got:\n%s", summary)
	}
}

func TestSummarizeGitHistory_ConventionalCountsAndScopes(t *testing.T) {
	tmpDir := t.TempDir()

	runGit(t, tmpDir, nil, "init")
	runGit(t, tmpDir, nil, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, nil, "config", "user.name", "Test User")
	runGit(t, tmpDir, nil, "config", "commit.gpgsign", "false")

	writeAndCommit := func(filename, content, message string) {
		t.Helper()
		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		runGit(t, tmpDir, nil, "add", filename)
		runGit(t, tmpDir, nil, "commit", "-m", message)
	}

	writeAndCommit("a.txt", "1", "chore: init")
	writeAndCommit("a.txt", "2", "feat(api): add endpoint")
	writeAndCommit("a.txt", "3", "fix(api): patch bug")
	writeAndCommit("a.txt", "4", "feat: add general")

	analyzer := newTestAnalyzer(tmpDir)
	summary, err := analyzer.summarizeGitHistory()
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}

	// High-signal invariants
	for _, want := range []string{
		"- Total commits: 4",
		"- Conventional commits (feat/fix/refactor/perf): 3",
		"feat=2 fix=1 refactor=0 perf=0",
		"- First commit: chore: init",
		"- Latest commit: feat: add general",
		"  - api: total=2",
		"  - General: total=1",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got:\n%s", want, summary)
		}
	}
}
