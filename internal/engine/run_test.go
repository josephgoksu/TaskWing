package engine

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josephgoksu/TaskWing/v2/internal/box"
)

func sh(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo makes a small repository with one commit, plus a bare origin.
func newRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	origin := filepath.Join(root, "origin.git")
	sh(t, root, "git", "init", "-q", "--bare", origin)
	sh(t, root, "git", "init", "-q", "-b", "main", repo)
	sh(t, repo, "git", "config", "user.name", "Test Person")
	sh(t, repo, "git", "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sh(t, repo, "git", "add", "-A")
	sh(t, repo, "git", "commit", "-q", "-m", "start")
	sh(t, repo, "git", "remote", "add", "origin", origin)
	sh(t, repo, "git", "push", "-q", "origin", "main")
	return repo
}

func testdata(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func quietSecrets(t *testing.T) {
	for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "AWS_SECRET_ACCESS_KEY"} {
		t.Setenv(k, "")
	}
}

func checkJoined(t *testing.T, repo string, res *Result) {
	t.Helper()
	if res.Passed != 3 || res.Total != 3 {
		t.Fatalf("exam: %d of %d passed", res.Passed, res.Total)
	}
	files := sh(t, repo, "git", "ls-tree", "-r", "--name-only", res.Branch)
	for _, want := range []string{"README.md", "greet/hello.txt", "farewell/bye.txt"} {
		if !strings.Contains(files, want) {
			t.Errorf("branch %s is missing %s:\n%s", res.Branch, want, files)
		}
	}
	if strings.Contains(files, "taskwing_exam") {
		t.Errorf("the hidden exam leaked into the branch:\n%s", files)
	}
	log := sh(t, repo, "git", "log", "--format=%an <%ae> %s", res.Branch)
	if strings.Contains(log, "Claude") || !strings.Contains(log, "Test Person <test@example.com>") {
		t.Errorf("unexpected authors:\n%s", log)
	}
}

func TestRunLocal(t *testing.T) {
	quietSecrets(t)
	repo := newRepo(t)
	home := t.TempDir()
	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Task:     "add a greeting and a farewell",
		Dir:      repo,
		Provider: &box.Local{Root: filepath.Join(home, "boxes"), AgentCmd: "bash " + testdata(t, "agent.sh")},
		Home:     home,
		LeadCmd:  "bash " + testdata(t, "lead.sh"),
		ExamCmd:  "bash " + testdata(t, "exam.sh"),
		Yes:      true,
		NoPR:     true,
		Out:      &out,
		In:       strings.NewReader(""),
	})
	t.Log("\n" + out.String())
	if err != nil {
		t.Fatal(err)
	}
	checkJoined(t, repo, res)
	if !strings.Contains(out.String(), "Plan refused") {
		t.Error("the overlapping first plan was not refused")
	}
	if left, _ := os.ReadDir(filepath.Join(home, "boxes")); len(left) != 0 {
		t.Errorf("%d boxes left behind", len(left))
	}
	if st := sh(t, repo, "git", "status", "--porcelain"); st != "" {
		t.Errorf("the user's working tree changed:\n%s", st)
	}
	if wts := sh(t, repo, "git", "worktree", "list"); strings.Count(wts, "\n") != 0 {
		t.Errorf("worktrees left behind:\n%s", wts)
	}
	if sh(t, repo, "git", "rev-parse", "--abbrev-ref", "HEAD") != "main" {
		t.Error("the user's branch changed")
	}
}

func TestRunBoatWithPR(t *testing.T) {
	quietSecrets(t)
	repo := newRepo(t)
	home := t.TempDir()
	state := t.TempDir()
	t.Setenv("PATH", testdata(t, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_STATE", state)
	t.Setenv("FAKE_AGENT", testdata(t, "agent.sh"))
	t.Setenv("FAKE_AGENT_STRAY", "1")
	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Task:     "add a greeting and a farewell",
		Dir:      repo,
		Provider: &box.Boat{Base: "base1", Poll: 200 * time.Millisecond},
		Home:     home,
		LeadCmd:  "bash " + testdata(t, "lead.sh"),
		ExamCmd:  "bash " + testdata(t, "exam.sh"),
		Out:      &out,
		In:       strings.NewReader("y\n"),
	})
	t.Log("\n" + out.String())
	if err != nil {
		t.Fatal(err)
	}
	checkJoined(t, repo, res)
	if res.PRURL != "https://github.com/example/repo/pull/7" {
		t.Errorf("PR url = %q", res.PRURL)
	}
	if got := sh(t, repo, "git", "ls-remote", "origin", res.Branch); got == "" {
		t.Error("the branch was not pushed")
	}
	body, err := os.ReadFile(filepath.Join(state, "pr-body.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"3 of 3 checks passed", "01 Greeting", "greet/hello.txt says hello", "outside its area: stray.txt", "Secret scan: nothing found"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("PR body is missing %q:\n%s", want, body)
		}
	}
	for _, banned := range []string{"Claude", "Anthropic", "AI "} {
		if strings.Contains(string(body), banned) {
			t.Errorf("PR body mentions %q", banned)
		}
	}
	if left, _ := os.ReadDir(filepath.Join(state, "boxes")); len(left) != 0 {
		t.Errorf("%d boat boxes left behind", len(left))
	}
}

func TestRunStopsWhenUserSaysNo(t *testing.T) {
	quietSecrets(t)
	repo := newRepo(t)
	home := t.TempDir()
	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Task:     "add a greeting and a farewell",
		Dir:      repo,
		Provider: &box.Local{Root: filepath.Join(home, "boxes"), AgentCmd: "false"},
		Home:     home,
		LeadCmd:  "bash " + testdata(t, "lead.sh"),
		ExamCmd:  "bash " + testdata(t, "exam.sh"),
		NoPR:     true,
		Out:      &out,
		In:       strings.NewReader("n\n"),
	})
	if err != nil || !res.Stopped {
		t.Fatalf("expected a clean stop, got %v %+v\n%s", err, res, out.String())
	}
	if _, err := os.Stat(filepath.Join(res.RunDir, "exam", "exam.patch")); err != nil {
		t.Error("the exam was not saved")
	}
}

func TestRunRefusesDirtyTree(t *testing.T) {
	repo := newRepo(t)
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("changed\n"), 0o644)
	_, err := Run(context.Background(), Options{Task: "x", Dir: repo, Provider: &box.Local{}, Home: t.TempDir(), NoPR: true, Out: &bytes.Buffer{}, In: strings.NewReader("")})
	if err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("expected an uncommitted-changes error, got %v", err)
	}
}

func TestRunRefusesBoxWithGitHubLogin(t *testing.T) {
	quietSecrets(t)
	repo := newRepo(t)
	home := t.TempDir()
	state := t.TempDir()
	t.Setenv("PATH", testdata(t, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_STATE", state)
	t.Setenv("FAKE_AGENT", testdata(t, "agent.sh"))
	t.Setenv("GH_TOKEN", "ghp_abcdefghijklmnopqrstuvwxyz0123")
	var out bytes.Buffer
	_, err := Run(context.Background(), Options{
		Task:     "add a greeting and a farewell",
		Dir:      repo,
		Provider: &box.Boat{Base: "base1", Poll: 200 * time.Millisecond},
		Home:     home,
		LeadCmd:  "bash " + testdata(t, "lead.sh"),
		ExamCmd:  "bash " + testdata(t, "exam.sh"),
		Yes:      true,
		NoPR:     true,
		Out:      &out,
		In:       strings.NewReader(""),
	})
	if err == nil || !strings.Contains(out.String(), "holds GitHub token") {
		t.Fatalf("expected the run to refuse a box with a GitHub token, got %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "ghp_") {
		t.Error("a token value was printed")
	}
	if left, _ := os.ReadDir(filepath.Join(state, "boxes")); len(left) != 0 {
		t.Errorf("%d boxes left behind", len(left))
	}
}

func TestRunFailsFastWhenBoxHasNoClaudeLogin(t *testing.T) {
	quietSecrets(t)
	repo := newRepo(t)
	state := t.TempDir()
	t.Setenv("PATH", testdata(t, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_STATE", state)
	t.Setenv("FAKE_AGENT", testdata(t, "agent.sh"))
	t.Setenv("FAKE_BOAT_NO_LOGIN", "1")
	var out bytes.Buffer
	start := time.Now()
	_, err := Run(context.Background(), Options{
		Task:     "add a greeting and a farewell",
		Dir:      repo,
		Provider: &box.Boat{Base: "base1", Poll: 200 * time.Millisecond},
		Home:     t.TempDir(),
		LeadCmd:  "bash " + testdata(t, "lead.sh"),
		ExamCmd:  "bash " + testdata(t, "exam.sh"),
		Yes:      true,
		NoPR:     true,
		Out:      &out,
		In:       strings.NewReader(""),
	})
	if err == nil || !strings.Contains(out.String(), "Claude login required") {
		t.Fatalf("expected the pieces to fail with the login error, got %v\n%s", err, out.String())
	}
	if time.Since(start) > 30*time.Second {
		t.Errorf("took %s to notice", time.Since(start))
	}
}

func TestHuntLocal(t *testing.T) {
	quietSecrets(t)
	repo := newRepo(t)
	for f, body := range map[string]string{"calc/add.sh": "echo $(($1 - $2))\n", "text/upper.sh": "echo \"$1\"\n"} {
		_ = os.MkdirAll(filepath.Join(repo, filepath.Dir(f)), 0o755)
		_ = os.WriteFile(filepath.Join(repo, f), []byte(body), 0o644)
	}
	sh(t, repo, "git", "add", "-A")
	sh(t, repo, "git", "commit", "-q", "-m", "buggy scripts")
	home := t.TempDir()
	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Hunt:     true,
		Dir:      repo,
		Provider: &box.Local{Root: filepath.Join(home, "boxes"), AgentCmd: "bash " + testdata(t, "fixer.sh")},
		Home:     home,
		HuntCmd:  "bash " + testdata(t, "hunter.sh"),
		Yes:      true,
		NoPR:     true,
		Out:      &out,
		In:       strings.NewReader(""),
	})
	t.Log("\n" + out.String())
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 || res.Passed != 3 {
		t.Fatalf("exam: %d of %d passed (want 3 of 3: two bugs + regression)", res.Passed, res.Total)
	}
	for _, want := range []string{"add subtracts instead of adding", "upper does not uppercase", "a false alarm (its test passes on today's code)", "add also bad (touches the same files as b1)", "readme typo (the hunter dropped it: not a bug)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output is missing %q", want)
		}
	}
	if got := sh(t, repo, "bash", "-c", "git show "+res.Branch+":calc/add.sh"); !strings.Contains(got, "+") {
		t.Errorf("add.sh not fixed on the branch: %s", got)
	}
	if strings.Contains(sh(t, repo, "git", "ls-tree", "-r", "--name-only", res.Branch), "taskwing_exam") {
		t.Error("the hidden tests leaked into the branch")
	}
	logs, _ := filepath.Glob(filepath.Join(res.RunDir, "logs", "agent-*.log"))
	for _, l := range logs {
		if b, _ := os.ReadFile(l); strings.Contains(string(b), "LEAK") {
			t.Errorf("%s: the fixer saw the hidden test", l)
		}
	}
}
