package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/v2/internal/proc"
)

// bug is one finding from the hunter, with the test that proves it.
type bug struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Where   string   `json:"where"`
	Why     string   `json:"why"`
	Owns    []string `json:"owns"`
	Test    string   `json:"test"`
	Command string   `json:"command"`
	proven  bool
	dropped string
}

func huntPrompt(focus string, max int) string {
	if strings.TrimSpace(focus) == "" {
		focus = "the whole repository"
	}
	return fmt.Sprintf(`You hunt for real bugs in this repository and prove each one with a failing test. Other agents will fix them later without seeing your tests.

Look at: %s

Do this:
1. Read the code and its tests. Look for real defects: wrong results, crashes, races, broken edge cases, code that does not do what its name, docs or callers expect, inconsistencies between parts. Not style, not missing features, not "could be risky".
2. For each bug (at most %d, the most important first), write ONE new test file whose path contains "taskwing_exam" (for example "billing/taskwing_exam_b1_test.go"). Never change an existing file. The test must compile and run on the current code, and FAIL there because of the bug, through an assertion, not a compile error. It must pass once the bug is fixed properly.
3. Run each test yourself and see it fail for the right reason. Delete any test that passes; that is not a bug.
4. For each bug, name the files a fix should touch as path patterns relative to the repository root ("billing/retry.go", "api/**"). Include one NEW test file for the fixer to add its own test, named after the bug, for example "billing/retry_b1_fix_test.go". No pattern may cover any of your taskwing_exam files (so prefer exact files to folders), and none may own the whole repository. Two bugs must not share files; if they do, keep the more important one.
5. Give one command that runs all the existing tests.
6. List every suspect you dropped (its test passed, or it was not a real bug) with one plain reason each.

When the files are written, answer with ONLY this JSON, no other text:
{"bugs": [{"id": "b1", "title": "<3 to 8 plain words>", "where": "<file:line>", "why": "<one plain sentence: what goes wrong and when>", "owns": ["<pattern>"], "test": "<the test file path>", "command": "<shell command from the repository root that runs only this test>"}],
 "regression": "<command that runs all existing tests>",
 "dropped": [{"title": "<suspect>", "why": "<why it was dropped>"}]}
`, focus, max)
}

func fixPrompt(b bug) string {
	return fmt.Sprintf(`Fix this bug: %s

Where: %s
What goes wrong: %s

Find the root cause and fix it properly. Do not special-case one input. Add or update a test in the files you own that shows the fix, and keep all existing tests passing. A hidden test will check the fix later.`, b.Title, b.Where, b.Why)
}

// hunt runs the bug hunter in a throwaway worktree, keeps only the bugs whose
// test really fails on today's code, and turns them into cards and an exam.
func (r *runner) hunt(ctx context.Context) (*Plan, []Check, []byte, error) {
	t0 := time.Now()
	view := &cardRun{n: 1, card: Card{Title: "Bug hunter"}, state: stWorking, thought: "reading the code", started: t0}
	var (
		bugs       []bug
		regression string
		patch      []byte
		herr       error
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		bugs, regression, patch, herr = r.runHunter(ctx, view)
		view.set(func(c *cardRun) {
			c.ended = time.Now()
			if herr != nil {
				c.state, c.err = stStuck, truncate(herr.Error(), 70)
				return
			}
			n := 0
			for _, b := range bugs {
				if b.proven {
					n++
				}
			}
			c.state, c.proof = stDone, fmt.Sprintf("%d %s proven", n, plural(n, "bug", "bugs"))
		})
	}()
	r.watchUntil("Hunt", t0, []*cardRun{view}, done)
	if herr != nil {
		return nil, nil, nil, fmt.Errorf("hunt: %w", herr)
	}

	task := "Fix bugs found by a hunt"
	if f := strings.TrimSpace(r.o.Task); f != "" {
		task = "Fix bugs found in " + f
	}
	plan := &Plan{Task: task}
	var checks []Check
	for _, b := range bugs {
		if !b.proven {
			r.ui.say("  %s dropped: %s (%s)", r.ui.dim("·"), b.Title, b.dropped)
			continue
		}
		plan.Cards = append(plan.Cards, Card{ID: b.ID, Title: b.Title, Owns: b.Owns, Prompt: fixPrompt(b), Where: b.Where, Why: b.Why})
		checks = append(checks, Check{Text: "Fixed: " + b.Title, Command: b.Command})
	}
	if len(plan.Cards) > 0 && strings.TrimSpace(regression) != "" {
		checks = append(checks, Check{Text: "All existing tests still pass", Command: regression})
	}
	for i := range checks {
		checks[i].ID = fmt.Sprintf("k%d", i+1)
	}
	// Each bug passed Validate alone in runHunter; this catches anything the
	// cards break together, with the same rules as a lead's plan.
	if len(plan.Cards) > 0 {
		if err := plan.Validate(r.o.MaxCards); err != nil {
			return nil, nil, nil, fmt.Errorf("hunt: the fix plan is unsafe: %w", err)
		}
	}
	r.saveJSON("plan.json", plan)
	if err := os.MkdirAll(filepath.Join(r.dir, "exam"), 0o700); err != nil {
		return nil, nil, nil, fmt.Errorf("hunt: %w", err)
	}
	b, err := json.MarshalIndent(checks, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(r.dir, "exam", "checks.json"), b, 0o600)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("hunt: could not save the checks: %w", err)
	}
	r.ev.log("hunt.ready", map[string]any{"found": len(bugs), "proven": len(plan.Cards)})
	return plan, checks, patch, nil
}

func (r *runner) runHunter(ctx context.Context, view *cardRun) ([]bug, string, []byte, error) {
	wt := filepath.Join(r.dir, "hunt-worktree")
	if _, err := git(ctx, r.root, "worktree", "add", "-q", "--detach", wt, r.base); err != nil {
		return nil, "", nil, err
	}
	defer func() { _, _ = git(context.Background(), r.root, "worktree", "remove", "--force", wt) }()
	max := r.o.MaxCards
	out, err := hostAgent(ctx, r.o.HuntCmd, wt, huntPrompt(r.o.Task, max), filepath.Join(r.dir, "logs", "hunter.log"))
	if err != nil {
		return nil, "", nil, fmt.Errorf("the bug hunter failed: %w", err)
	}
	js, err := extractJSON(out, "bugs")
	if err != nil {
		return nil, "", nil, fmt.Errorf("the bug hunter gave no findings: %w", err)
	}
	var h struct {
		Bugs       []bug  `json:"bugs"`
		Regression string `json:"regression"`
		Dropped    []struct {
			Title string `json:"title"`
			Why   string `json:"why"`
		} `json:"dropped"`
	}
	if err := json.Unmarshal([]byte(js), &h); err != nil {
		return nil, "", nil, fmt.Errorf("the findings are not valid JSON: %w", err)
	}
	if len(h.Bugs) > max {
		h.Bugs = h.Bugs[:max]
	}
	for _, d := range h.Dropped {
		if strings.TrimSpace(d.Title) != "" {
			h.Bugs = append(h.Bugs, bug{Title: d.Title, dropped: "the hunter dropped it: " + d.Why, ID: "-"})
		}
	}

	// Only new files with "taskwing_exam" in the path count.
	_, _ = git(ctx, wt, "add", "-A")
	// -z: without it git quotes non-ASCII paths ("\303\251"), and a quoted
	// name never matches the test path the hunter gave.
	status, _ := gitRaw(ctx, wt, "diff", "--cached", "--name-status", "--no-renames", "-z", r.base)
	added := map[string]bool{}
	fields := splitNUL(status)
	for i := 0; i+1 < len(fields); i += 2 {
		if st, f := fields[i], fields[i+1]; st == "A" && strings.Contains(f, examMarker) {
			added[f] = true
		}
	}
	_, _ = git(ctx, wt, "reset", "-q")
	// Take every test file out, then check each bug with only its own test in
	// place, so one broken test cannot make another bug look proven.
	files := map[string][]byte{}
	for f := range added {
		p, err := insideTree(wt, f)
		if err != nil {
			continue
		}
		if b, err := os.ReadFile(p); err == nil {
			files[f] = b
			_ = os.Remove(p)
		}
	}
	// Put the worktree back to today's code. Anything else the hunter changed
	// (an edited source file, a helper file) is not in the exam box either, so
	// it must not decide whether a test fails.
	if _, err := git(ctx, wt, "checkout", "-q", "--", "."); err != nil {
		return nil, "", nil, err
	}
	if _, err := git(ctx, wt, "clean", "-fdq"); err != nil {
		return nil, "", nil, err
	}

	// Prove each bug: its test must fail on today's code. Owned files must not overlap.
	var keep []string
	seen := map[string]bool{}
	for i := range h.Bugs {
		b := &h.Bugs[i]
		if b.dropped != "" {
			continue
		}
		b.Test = strings.TrimPrefix(strings.TrimSpace(b.Test), "./")
		// The id names a git ref and a log file: it must be safe and unique.
		// Case-folded: "B1" and "b1" are one ref file on macOS.
		if !validCardID(b.ID) || seen[strings.ToLower(b.ID)] {
			b.ID = fmt.Sprintf("b%d", i+1)
			for n := i + 2; seen[b.ID]; n++ {
				b.ID = fmt.Sprintf("b%d", n)
			}
		}
		seen[strings.ToLower(b.ID)] = true
		view.set(func(c *cardRun) { c.thought = "checking " + b.ID + " fails today: " + b.Title })
		switch {
		case strings.TrimSpace(b.Command) == "" || len(b.Owns) == 0:
			b.dropped = "no test command or no files to fix"
			continue
		case files[b.Test] == nil:
			b.dropped = "its test file is missing or not a new taskwing_exam file"
			continue
		}
		// The same rules as a lead's plan, so a fixer cannot own the whole
		// repository, a path outside it, or the exam's own files.
		card := Card{ID: b.ID, Title: b.Title, Owns: b.Owns, Prompt: fixPrompt(*b)}
		one := &Plan{Cards: []Card{card}}
		if err := one.Validate(0); err != nil {
			b.dropped = "its files are unsafe to hand out: " + err.Error()
			continue
		}
		card = one.Cards[0]
		b.Owns = card.Owns
		if t := ownsAny(card, added); t != "" {
			b.dropped = "its files cover the hidden test " + t
			continue
		}
		if o := overlapsEarlier(card, h.Bugs[:i]); o != "" {
			b.dropped = "touches the same files as " + o
			continue
		}
		testPath, err := insideTree(wt, b.Test)
		if err == nil {
			err = os.MkdirAll(filepath.Dir(testPath), 0o700)
		}
		if err == nil {
			err = os.WriteFile(testPath, files[b.Test], 0o600)
		}
		if err != nil {
			b.dropped = "its test could not be put back: " + err.Error()
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, r.o.CheckTimeout)
		cmd := exec.CommandContext(cctx, "nice", "-n", "10", "bash", "-c", b.Command)
		proc.Group(cmd)
		cmd.Dir = wt
		cmdOut, err := cmd.CombinedOutput()
		// A test that exits 0 but leaves a process holding the output open
		// ends with ErrWaitDelay: it passed. The leftovers are stopped, or
		// they keep running on this machine.
		if errors.Is(err, exec.ErrWaitDelay) {
			err = nil
		}
		_ = proc.Kill(cmd)
		cancel()
		_ = os.Remove(testPath)
		if werr := os.WriteFile(filepath.Join(r.dir, "logs", "bug-"+b.ID+".log"), cmdOut, 0o600); werr != nil {
			r.ui.say("  %s could not save the log of %s: %v", r.ui.amber("!"), b.ID, werr)
		}
		if err == nil {
			b.dropped = "its test passes on today's code"
			continue
		}
		if notCompiled(string(cmdOut)) {
			b.dropped = "its test does not even build on today's code"
			continue
		}
		b.proven = true
		keep = append(keep, b.Test)
		r.ev.log("bug.proven", map[string]any{"id": b.ID, "title": b.Title, "where": b.Where})
	}
	if len(keep) == 0 {
		return h.Bugs, h.Regression, nil, nil
	}
	for _, f := range keep {
		p, err := insideTree(wt, f)
		if err == nil {
			err = os.MkdirAll(filepath.Dir(p), 0o700)
		}
		if err == nil {
			err = os.WriteFile(p, files[f], 0o600)
		}
		if err != nil {
			return nil, "", nil, fmt.Errorf("could not put back the test %s: %w", f, err)
		}
	}
	if _, err := git(ctx, wt, append([]string{"add", "--"}, keep...)...); err != nil {
		return nil, "", nil, err
	}
	patch, err := gitRaw(ctx, wt, "diff", "--cached", "--binary", r.base)
	if err != nil {
		return nil, "", nil, err
	}
	examDir := filepath.Join(r.dir, "exam")
	if err := os.MkdirAll(examDir, 0o700); err != nil {
		return nil, "", nil, err
	}
	if err := os.WriteFile(filepath.Join(examDir, "exam.patch"), []byte(patch), 0o600); err != nil {
		return nil, "", nil, fmt.Errorf("could not save the exam patch: %w", err)
	}
	return h.Bugs, h.Regression, []byte(patch), nil
}

// overlapsEarlier names an earlier proven bug whose files c would also touch.
func overlapsEarlier(c Card, earlier []bug) string {
	for _, e := range earlier {
		if !e.proven {
			continue
		}
		if _, _, ok := c.overlaps(Card{Owns: e.Owns}); ok {
			return e.ID
		}
	}
	return ""
}

// ownsAny names a hidden test file that one of c's patterns covers. A fixer
// owning it may write the same path, and the hidden test then fails to apply
// in the exam box.
func ownsAny(c Card, tests map[string]bool) string {
	for t := range tests {
		for _, p := range c.Owns {
			if MatchOwn(strings.ToLower(p), strings.ToLower(t)) {
				return t
			}
		}
	}
	return ""
}

// insideTree joins rel onto root and refuses anything that could land outside
// it: an absolute path, a .. segment, or a symlink on the way (the hunter can
// plant "pkg -> /home/you" and turn our write of "pkg/x_test.go" into a write
// anywhere on this machine).
func insideTree(root, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("%q is not a path inside the repository", rel)
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts {
		if part == ".." || part == "" || part == "." {
			return "", fmt.Errorf("%q is not a clean path inside the repository", rel)
		}
	}
	p := root
	for _, part := range parts {
		p = filepath.Join(p, part)
		fi, err := os.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) {
			break // nothing further down exists yet, so nothing can redirect
		}
		if err != nil {
			return "", err
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			return "", fmt.Errorf("%q goes through a symlink", rel)
		}
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

func (r *runner) showBugs(p *Plan) {
	r.ui.say("\n  %s", r.ui.bold("Bugs proven by a failing test")+r.ui.dim("  (the fixers never see the tests)"))
	for i, c := range p.Cards {
		r.ui.say("  %02d %s %s  %s", i+1, r.ui.red("✗"), c.Title, r.ui.dim(c.Where))
		r.ui.say("     %s", c.Why)
	}
}

// notCompiled spots a test that failed because it does not build, not
// because of the bug. It is a rough check across common toolchains.
func notCompiled(out string) bool {
	for _, m := range []string{"[build failed]", "[setup failed]", "build constraints exclude", "error TS", "SyntaxError", "ModuleNotFoundError", "ImportError", "error[E", "Cannot find module", "cannot find package"} {
		if strings.Contains(out, m) {
			return true
		}
	}
	return false
}
