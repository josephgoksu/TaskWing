package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
4. For each bug, name the files a fix should touch as path patterns ("billing/retry.go", "api/**"). Include one NEW test file for the fixer to add its own test, named after the bug, for example "billing/retry_b1_fix_test.go". Never list your taskwing_exam file. Two bugs must not share files; if they do, keep the more important one.
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
				c.state, c.err = stFailed, truncate(herr.Error(), 70)
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
	r.saveJSON("plan.json", plan)
	_ = os.MkdirAll(filepath.Join(r.dir, "exam"), 0o755)
	b, _ := json.MarshalIndent(checks, "", "  ")
	_ = os.WriteFile(filepath.Join(r.dir, "exam", "checks.json"), b, 0o644)
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
		return nil, "", nil, fmt.Errorf("the bug hunter failed: %v", err)
	}
	js, err := extractJSON(out)
	if err != nil {
		return nil, "", nil, fmt.Errorf("the bug hunter gave no findings: %v", err)
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
		return nil, "", nil, fmt.Errorf("the findings are not valid JSON: %v", err)
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
	status, _ := git(ctx, wt, "diff", "--cached", "--name-status", "--no-renames", r.base)
	added := map[string]bool{}
	for _, line := range strings.Split(status, "\n") {
		if f := strings.SplitN(line, "\t", 2); len(f) == 2 && f[0] == "A" && strings.Contains(f[1], "taskwing_exam") {
			added[f[1]] = true
		}
	}
	_, _ = git(ctx, wt, "reset", "-q")
	// Take every test file out, then check each bug with only its own test in
	// place, so one broken test cannot make another bug look proven.
	files := map[string][]byte{}
	for f := range added {
		if b, err := os.ReadFile(filepath.Join(wt, f)); err == nil {
			files[f] = b
			_ = os.Remove(filepath.Join(wt, f))
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
		if !validCardID(b.ID) || seen[b.ID] {
			b.ID = fmt.Sprintf("b%d", i+1)
			for n := i + 2; seen[b.ID]; n++ {
				b.ID = fmt.Sprintf("b%d", n)
			}
		}
		seen[b.ID] = true
		view.set(func(c *cardRun) { c.thought = "checking " + b.ID + " fails today: " + b.Title })
		switch {
		case strings.TrimSpace(b.Command) == "" || len(b.Owns) == 0:
			b.dropped = "no test command or no files to fix"
			continue
		case !added[b.Test]:
			b.dropped = "its test file is missing or not a new taskwing_exam file"
			continue
		}
		if o := r.overlap(b, h.Bugs[:i]); o != "" {
			b.dropped = "touches the same files as " + o
			continue
		}
		testPath := filepath.Join(wt, b.Test)
		_ = os.MkdirAll(filepath.Dir(testPath), 0o755)
		_ = os.WriteFile(testPath, files[b.Test], 0o644)
		cctx, cancel := context.WithTimeout(ctx, r.o.CheckTimeout)
		cmd := exec.CommandContext(cctx, "bash", "-c", "nice -n 10 bash -c "+shq(b.Command))
		cmd.Dir = wt
		cmdOut, err := cmd.CombinedOutput()
		cancel()
		_ = os.Remove(testPath)
		_ = os.WriteFile(filepath.Join(r.dir, "logs", "bug-"+b.ID+".log"), cmdOut, 0o644)
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
		_ = os.MkdirAll(filepath.Dir(filepath.Join(wt, f)), 0o755)
		_ = os.WriteFile(filepath.Join(wt, f), files[f], 0o644)
	}
	if _, err := git(ctx, wt, append([]string{"add", "--"}, keep...)...); err != nil {
		return nil, "", nil, err
	}
	patch, err := gitRaw(ctx, wt, "diff", "--cached", "--binary", r.base)
	if err != nil {
		return nil, "", nil, err
	}
	examDir := filepath.Join(r.dir, "exam")
	_ = os.MkdirAll(examDir, 0o755)
	_ = os.WriteFile(filepath.Join(examDir, "exam.patch"), []byte(patch), 0o644)
	return h.Bugs, h.Regression, []byte(patch), nil
}

// overlap names an earlier proven bug whose files this bug would also touch.
func (r *runner) overlap(b *bug, earlier []bug) string {
	for _, e := range earlier {
		if !e.proven {
			continue
		}
		for _, x := range b.Owns {
			for _, y := range e.Owns {
				if patternsOverlap(x, y) {
					return e.ID
				}
			}
		}
	}
	return ""
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
