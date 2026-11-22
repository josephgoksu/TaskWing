package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josephgoksu/TaskWing/v2/internal/box"
)

// client drives Serve the way a front end does: commands in, events out.
type client struct {
	t      *testing.T
	in     *io.PipeWriter
	events chan map[string]any
	seen   []map[string]any
	done   chan error
}

func startClient(t *testing.T, start StartFunc) *client {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	a := &client{t: t, in: inW, events: make(chan map[string]any, 1000), done: make(chan error, 1)}
	go func() {
		err := Serve(context.Background(), inR, outW, "test", start)
		outW.Close()
		a.done <- err
	}()
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			var ev map[string]any
			if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
				t.Errorf("engine wrote a line that is not JSON: %q", sc.Text())
				continue
			}
			a.events <- ev
		}
		close(a.events)
	}()
	t.Cleanup(func() {
		inW.Close()
		select {
		case <-a.done:
		case <-time.After(30 * time.Second):
			t.Error("the engine did not exit after stdin closed")
		}
	})
	return a
}

func (a *client) send(cmd map[string]any) {
	a.t.Helper()
	b, _ := json.Marshal(cmd)
	if _, err := a.in.Write(append(b, '\n')); err != nil {
		a.t.Fatal(err)
	}
}

// wait returns the next event that matches, failing after the timeout.
func (a *client) wait(timeout time.Duration, match func(map[string]any) bool) map[string]any {
	a.t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-a.events:
			if !ok {
				a.t.Fatalf("the engine stopped; events so far: %v", a.seen)
			}
			a.seen = append(a.seen, ev)
			if match(ev) {
				return ev
			}
		case <-deadline:
			a.t.Fatalf("timed out; events so far: %v", a.seen)
		}
	}
}

func typed(typ string) func(map[string]any) bool {
	return func(ev map[string]any) bool { return ev["type"] == typ }
}

func (a *client) count(match func(map[string]any) bool) int {
	n := 0
	for _, ev := range a.seen {
		if match(ev) {
			n++
		}
	}
	return n
}

func cardState(id, state string) func(map[string]any) bool {
	return func(ev map[string]any) bool {
		c, _ := ev["card"].(map[string]any)
		return ev["type"] == "card.update" && c["id"] == id && c["state"] == state
	}
}

func boatStart(t *testing.T, home string) StartFunc {
	return func(ctx context.Context, c Command) (Options, error) {
		dir, err := ResolveRepo(ctx, home, c.Repo)
		if err != nil {
			return Options{}, err
		}
		return Options{
			Task:     c.Task,
			Dir:      dir,
			Provider: &box.Boat{Base: "base1", Poll: 200 * time.Millisecond},
			Home:     home,
			LeadCmd:  "bash " + testdata(t, "lead.sh"),
			ExamCmd:  "bash " + testdata(t, "exam.sh"),
		}, nil
	}
}

func TestEngineStreamRunToPR(t *testing.T) {
	quietSecrets(t)
	repo := newRepo(t)
	home := t.TempDir()
	state := t.TempDir()
	t.Setenv("PATH", testdata(t, "fakebin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_STATE", state)
	t.Setenv("FAKE_AGENT", testdata(t, "agent.sh"))
	a := startClient(t, boatStart(t, home))

	hello := a.wait(5*time.Second, typed("engine.hello"))
	if hello["version"] != "test" || !strings.Contains(strings.Join(toStrings(hello["can"]), ","), "exam.now") {
		t.Errorf("hello = %v", hello)
	}
	a.send(map[string]any{"type": "run.start", "repo": repo, "task": "add a greeting and a farewell", "sandbox": "boat", "models": "Claude"})
	created := a.wait(5*time.Second, typed("run.created"))
	id := created["run"].(map[string]any)["id"].(string)

	plan := a.wait(30*time.Second, typed("plan.ready"))
	if plan["run_id"] != id {
		t.Fatalf("plan for run %v, want %s", plan["run_id"], id)
	}
	cards := plan["cards"].([]any)
	checks := plan["checks"].([]any)
	if len(cards) != 2 || len(checks) != 3 || checks[0].(map[string]any)["id"] != "k1" {
		t.Fatalf("plan = %v", plan)
	}
	a.wait(5*time.Second, func(ev map[string]any) bool { return ev["type"] == "run.stage" && ev["stage"] == "approve" })

	// Approve with one check of the user's own.
	approved := append(checks, map[string]any{"id": "u1", "text": "The greeting ends with a newline", "added_by": "user"})
	a.send(map[string]any{"type": "plan.approve", "run_id": id, "checks": approved})

	a.wait(60*time.Second, cardState("c1", "passed"))
	res := a.wait(60*time.Second, typed("exam.result"))
	got := map[string]string{}
	for _, c := range res["checks"].([]any) {
		m := c.(map[string]any)
		got[m["id"].(string)] = m["result"].(string)
	}
	for _, k := range []string{"k1", "k2", "k3", "u1"} {
		if got[k] != "pass" {
			t.Errorf("check %s = %q (all: %v)", k, got[k], got)
		}
	}
	pr := a.wait(30*time.Second, typed("pr.opened"))
	if pr["url"] != "https://github.com/example/repo/pull/7" || pr["number"] != float64(7) || pr["draft"] != true {
		t.Errorf("pr.opened = %v", pr)
	}
	fin := a.wait(10*time.Second, typed("run.finished"))
	if fin["ok"] != true || fin["secrets_found"] != float64(0) {
		t.Errorf("run.finished = %v", fin)
	}
	if a.count(func(ev map[string]any) bool { return ev["type"] == "run.stage" && ev["stage"] == "done" }) != 1 {
		t.Error("no done stage")
	}
	if a.count(typed("run.error")) != 0 {
		t.Error("a successful run sent run.error")
	}
	body, err := os.ReadFile(filepath.Join(state, "pr-body.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "4 of 4 checks passed") || !strings.Contains(string(body), "The greeting ends with a newline") {
		t.Errorf("PR body:\n%s", body)
	}
	if strings.Contains(sh(t, repo, "git", "ls-tree", "-r", "--name-only", "taskwing/"+id), "taskwing_exam") {
		t.Error("an exam leaked into the branch")
	}
}

func TestEngineStreamRedoAndExamNow(t *testing.T) {
	quietSecrets(t)
	repo := newRepo(t)
	home := t.TempDir()
	t.Setenv("FAKE_AGENT_SLEEP", "20")
	t.Setenv("FAKE_AGENT_WRITE_FIRST", "1")
	a := startClient(t, func(ctx context.Context, c Command) (Options, error) {
		return Options{
			Task:     c.Task,
			Dir:      c.Repo,
			Provider: &box.Local{Root: filepath.Join(home, "boxes"), AgentCmd: "bash " + testdata(t, "agent.sh")},
			Home:     home,
			LeadCmd:  "bash " + testdata(t, "lead.sh"),
			ExamCmd:  "bash " + testdata(t, "exam.sh"),
			NoPR:     true,
		}, nil
	})
	a.send(map[string]any{"type": "run.start", "repo": repo, "task": "add a greeting and a farewell"})
	id := a.wait(5*time.Second, typed("run.created"))["run"].(map[string]any)["id"].(string)
	a.wait(30*time.Second, typed("plan.ready"))
	a.send(map[string]any{"type": "exam.now", "run_id": id}) // ignored: nothing is building
	a.send(map[string]any{"type": "plan.redo", "run_id": id})
	a.wait(5*time.Second, func(ev map[string]any) bool { return ev["type"] == "run.stage" && ev["stage"] == "planning" })
	a.wait(30*time.Second, typed("plan.ready"))
	a.send(map[string]any{"type": "plan.approve", "run_id": id, "checks": []any{}})

	// Both agents work; the user asks for the exam before they finish.
	a.wait(30*time.Second, cardState("c1", "working"))
	a.wait(30*time.Second, cardState("c2", "working"))
	time.Sleep(1500 * time.Millisecond) // let the agents write their files
	start := time.Now()
	a.send(map[string]any{"type": "exam.now", "run_id": id})
	a.wait(30*time.Second, cardState("c1", "done"))
	res := a.wait(30*time.Second, typed("exam.result"))
	if time.Since(start) > 15*time.Second {
		t.Errorf("the exam waited for the agents (%s)", time.Since(start))
	}
	for _, c := range res["checks"].([]any) {
		if m := c.(map[string]any); m["result"] != "pass" {
			t.Errorf("check %v failed: the early work was lost", m["id"])
		}
	}
	fin := a.wait(10*time.Second, typed("run.finished"))
	if fin["ok"] != true {
		t.Errorf("run.finished = %v", fin)
	}
	if a.count(typed("plan.ready")) != 2 {
		t.Error("plan.redo did not make a new plan")
	}
}

func TestEngineStreamErrors(t *testing.T) {
	home := t.TempDir()
	a := startClient(t, boatStart(t, home))
	for _, repo := range []string{"/no/such/folder", "not a repo!"} {
		a.send(map[string]any{"type": "run.start", "repo": repo, "task": "x"})
		id := a.wait(5*time.Second, typed("run.created"))["run"].(map[string]any)["id"]
		e := a.wait(5*time.Second, typed("run.error"))
		if e["run_id"] != id || e["message"] == "" {
			t.Errorf("run.error = %v", e)
		}
		fin := a.wait(5*time.Second, typed("run.finished"))
		if fin["ok"] != false {
			t.Errorf("run.finished = %v", fin)
		}
	}
	// Junk and commands for unknown runs are ignored.
	a.send(map[string]any{"type": "answer", "run_id": "nope"})
	if _, err := a.in.Write([]byte("not json\n")); err != nil {
		t.Fatal(err)
	}
}

func TestServeStopsOnSignal(t *testing.T) {
	// Run by hand, stdin stays open: a cancelled context (Ctrl-C) must still end Serve.
	inR, inW := io.Pipe()
	defer inW.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, inR, io.Discard, "test", nil) }()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve kept reading stdin after its context ended")
	}
}

func TestReadLinesSkipsLongLines(t *testing.T) {
	in := "a\n" + strings.Repeat("x", 100) + "\nb\r\nc"
	var got []string
	if err := readLines(strings.NewReader(in), 10, func(l []byte) bool { got = append(got, string(l)); return true }); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != "a|b|c" {
		t.Errorf("got %q", got)
	}
}

func TestResolveRepo(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	dir := t.TempDir()
	if got, err := ResolveRepo(ctx, home, dir); err != nil || got != dir {
		t.Errorf("folder: %q %v", got, err)
	}
	for _, bad := range []string{"", "a/b/c", "../x", "a/..", "a/.", "-u/x", "./x", "x y/z", "/no/such/folder"} {
		if _, err := ResolveRepo(ctx, home, bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestInterfaceNames(t *testing.T) {
	got := interfaceNames("- POST /discounts\n\n* discount_cents\n  GET /health  \n")
	want := []string{"POST /discounts", "discount_cents", "GET /health"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", got)
	}
}

func toStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func TestResolveRepoOneRunPerClone(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	origin := filepath.Dir(newRepo(t))
	// A clone made earlier, as gh repo clone would leave it.
	dir := filepath.Join(home, "repos", "acme", "shop")
	sh(t, origin, "git", "--git-dir", filepath.Join(origin, "origin.git"), "symbolic-ref", "HEAD", "refs/heads/main")
	sh(t, origin, "git", "clone", "-q", filepath.Join(origin, "origin.git"), dir)
	got, err := ResolveRepo(ctx, home, "acme/shop")
	if err != nil || got != dir {
		t.Fatalf("first run: %q %v", got, err)
	}
	if _, err := ResolveRepo(ctx, home, "acme/shop"); err == nil || !strings.Contains(err.Error(), "another run") {
		t.Fatalf("second run at the same time: %v", err)
	}
	ReleaseRepo(dir)
	if _, err := ResolveRepo(ctx, home, "acme/shop"); err != nil {
		t.Fatalf("after the first run: %v", err)
	}
	ReleaseRepo(dir)
}
