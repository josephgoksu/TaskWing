// Package engine runs one TaskWing task: plan, exam, build, join, exam, pull request.
package engine

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/josephgoksu/TaskWing/internal/box"
)

// BoxPrefix starts the name of every box TaskWing creates.
const BoxPrefix = "taskwing"

// RunBoxPrefix starts the name of every box a run creates ("taskwing · tw-<time> · ...").
// doctor only touches boxes with this prefix, never a base box named by hand.
const RunBoxPrefix = BoxPrefix + " · tw-"

// Options configure one run.
type Options struct {
	Task         string
	Dir          string // any folder inside the repository
	Provider     box.Provider
	Home         string // state folder, usually ~/.taskwing
	LeadCmd      string // shell command for the lead; prompt on stdin
	ExamCmd      string // shell command for the exam session; prompt on stdin
	MaxCards     int
	Yes          bool // skip the questions
	NoPR         bool // stop after the exam; keep the branch locally
	Keep         bool // keep boxes after the run
	AgentTimeout time.Duration
	CheckTimeout time.Duration
	Out          io.Writer
	In           io.Reader
}

// Check is one exam check: a plain sentence and the command behind it.
type Check struct {
	Text    string `json:"text"`
	Command string `json:"command"`
	Passed  bool   `json:"passed"`
	Output  string `json:"output,omitempty"`
}

// Result is what a run produced.
type Result struct {
	RunID   string
	RunDir  string
	Branch  string
	PRURL   string
	Passed  int
	Total   int
	Secrets []string
	Stopped bool
}

type cardRun struct {
	mu      sync.Mutex
	card    Card
	n       int
	box     *box.Box
	agent   box.Agent
	state   string
	thought string
	final   string
	proof   string
	err     string
	started time.Time
	ended   time.Time
	ref     string
	files   []string
	outside []string
	joined  bool
}

func (c *cardRun) set(f func(c *cardRun)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f(c)
}

func (c *cardRun) view() cardView {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := cardView{n: c.n, title: c.card.Title, state: c.state, thought: c.thought}
	if c.state == stDone {
		v.thought = c.proof
		if v.thought == "" {
			v.thought = "finished"
		}
		v.took = c.ended.Sub(c.started)
	}
	if c.state == stFailed {
		v.thought = c.err
	}
	return v
}

type runner struct {
	o      Options
	ui     *ui
	ev     *events
	root   string
	base   string
	branch string // the branch the user is on; the PR targets it
	id     string
	dir    string
	start  time.Time

	boxesMu sync.Mutex
	boxes   []*box.Box
	secrets []string
}

// Run executes one task end to end.
func Run(ctx context.Context, o Options) (*Result, error) {
	if o.MaxCards == 0 {
		o.MaxCards = 4
	}
	if o.AgentTimeout == 0 {
		o.AgentTimeout = 90 * time.Minute
	}
	if o.CheckTimeout == 0 {
		o.CheckTimeout = 20 * time.Minute
	}
	r := &runner{o: o, ui: newUI(o.Out, o.In), start: time.Now()}
	res, err := r.run(ctx)
	r.cleanup()
	r.ev.log("run.done", map[string]any{"error": errString(err)})
	r.ev.close()
	return res, err
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (r *runner) run(ctx context.Context) (*Result, error) {
	o := r.o
	if strings.TrimSpace(o.Task) == "" {
		return nil, errors.New("no task given")
	}
	var err error
	if r.root, err = git(ctx, o.Dir, "rev-parse", "--show-toplevel"); err != nil {
		return nil, errors.New("run taskwing inside a git repository")
	}
	if st, _ := git(ctx, r.root, "status", "--porcelain"); st != "" {
		return nil, errors.New("you have uncommitted changes; commit or stash them first, because the agents start from your last commit")
	}
	if r.base, err = git(ctx, r.root, "rev-parse", "HEAD"); err != nil {
		return nil, errors.New("the repository has no commits yet")
	}
	r.branch, _ = git(ctx, r.root, "rev-parse", "--abbrev-ref", "HEAD")
	if !o.NoPR {
		if _, err := git(ctx, r.root, "remote", "get-url", "origin"); err != nil {
			return nil, errors.New("no origin remote to push to; add one, or pass --no-pr")
		}
		if out, err := exec.CommandContext(ctx, "gh", "auth", "status").CombinedOutput(); err != nil {
			return nil, fmt.Errorf("GitHub CLI is not logged in (gh auth login), or pass --no-pr: %s", lastLines(string(out), 2))
		}
		if r.branch == "HEAD" {
			return nil, errors.New("you are on a detached HEAD; check out the branch the pull request should target")
		}
	}
	r.id = "tw-" + time.Now().Format("20060102-150405")
	r.dir = filepath.Join(o.Home, "runs", r.id)
	if err := os.MkdirAll(filepath.Join(r.dir, "logs"), 0o755); err != nil {
		return nil, err
	}
	if r.ev, err = openEvents(r.dir); err != nil {
		return nil, err
	}
	res := &Result{RunID: r.id, RunDir: r.dir}
	r.ev.log("run.created", map[string]any{"task": o.Task, "repo": r.root, "base": r.base, "provider": o.Provider.Name()})
	r.ui.say("%s %s  %s", r.ui.bold("TaskWing"), r.id, r.ui.dim(o.Provider.Name()+" · "+filepath.Base(r.root)+" @ "+r.base[:8]))
	if !o.Provider.Sandboxed() {
		r.ui.say(r.ui.amber("Local mode: agents run on this machine with your own login. This is for testing, not a sandbox."))
	}

	// 1. Plan and exam, side by side. The exam session never sees the plan.
	r.ui.step(1, "Plan")
	r.ui.say("  The lead agent is splitting the task. A separate session is writing the exam.")
	plan, checks, patch, err := r.planAndExam(ctx)
	if err != nil {
		return res, err
	}
	res.Total = len(checks)
	r.showPlan(plan, checks)
	what := fmt.Sprintf("Start %d %s in %s?", len(plan.Cards), plural(len(plan.Cards), "agent", "agents"), sandboxWord(o.Provider))
	if !o.Yes && !r.ui.ask("\n"+what+" This is the exam they must pass.") {
		r.ev.log("run.stopped", nil)
		r.ui.say("Stopped. The plan and the exam are saved in %s", r.dir)
		res.Stopped = true
		return res, nil
	}
	r.ev.log("exam.approved", map[string]any{"checks": len(checks)})

	// 2. Build: one box and one agent per card.
	r.ui.step(2, "Build")
	cards, err := r.build(ctx, plan)
	if err != nil {
		return res, err
	}

	// 3. Join the work on one branch and run the exam in a clean box.
	r.ui.step(3, "Exam")
	res.Branch = "taskwing/" + r.id
	if err := r.join(ctx, cards, res.Branch); err != nil {
		return res, err
	}
	// Free the branch so the user can check it out; it stays in the repository.
	defer func() {
		_, _ = git(context.Background(), r.root, "worktree", "remove", "--force", filepath.Join(r.dir, "join"))
	}()
	if err := r.exam(ctx, checks, patch); err != nil {
		return res, err
	}
	for _, c := range checks {
		if c.Passed {
			res.Passed++
		}
	}
	res.Secrets = r.secrets
	r.saveJSON("exam-results.json", checks)

	// 4. Pull request, pushed from this machine.
	r.ui.step(4, "Pull request")
	if o.NoPR {
		r.ui.say("  Skipped (--no-pr). The joined work is on branch %s.", res.Branch)
		r.summary(res)
		return res, nil
	}
	if res.Passed < res.Total && !o.Yes {
		if !r.ui.ask(fmt.Sprintf("  %d of %d checks failed. Open the draft pull request anyway?", res.Total-res.Passed, res.Total)) {
			r.ui.say("  Not opened. The joined work is on branch %s.", res.Branch)
			r.summary(res)
			return res, nil
		}
	}
	url, err := r.openPR(ctx, plan, cards, checks, res)
	if err != nil {
		return res, err
	}
	res.PRURL = url
	r.ev.log("pr.opened", map[string]any{"url": url})
	r.summary(res)
	return res, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func sandboxWord(p box.Provider) string {
	if p.Sandboxed() {
		return p.Name() + " sandboxes"
	}
	return "local folders"
}

func (r *runner) saveJSON(name string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(filepath.Join(r.dir, name), b, 0o644)
}

func (r *runner) planAndExam(ctx context.Context) (*Plan, []Check, []byte, error) {
	var (
		wg      sync.WaitGroup
		plan    *Plan
		planErr error
		checks  []Check
		patch   []byte
		examErr error
	)
	t0 := time.Now()
	views := []*cardRun{
		{n: 1, card: Card{Title: "Lead: split the task"}, state: stWorking, thought: "reading the repository", started: t0},
		{n: 2, card: Card{Title: "Exam writer"}, state: stWorking, thought: "writing the hidden tests", started: t0},
	}
	finish := func(cr *cardRun, err error, proof string) {
		cr.set(func(c *cardRun) {
			c.ended = time.Now()
			if err != nil {
				c.state, c.err = stFailed, truncate(err.Error(), 70)
			} else {
				c.state, c.proof = stDone, proof
			}
		})
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		plan, planErr = r.lead(ctx)
		if planErr == nil {
			finish(views[0], nil, fmt.Sprintf("%d %s", len(plan.Cards), plural(len(plan.Cards), "piece", "pieces")))
		} else {
			finish(views[0], planErr, "")
		}
	}()
	go func() {
		defer wg.Done()
		checks, patch, examErr = r.writeExam(ctx)
		if examErr == nil {
			finish(views[1], nil, fmt.Sprintf("%d checks", len(checks)))
		} else {
			finish(views[1], examErr, "")
		}
	}()
	r.watch("Plan", t0, views, &wg)
	if planErr != nil {
		return nil, nil, nil, fmt.Errorf("plan: %w", planErr)
	}
	if examErr != nil {
		return nil, nil, nil, fmt.Errorf("exam: %w", examErr)
	}
	return plan, checks, patch, nil
}

func (r *runner) lead(ctx context.Context) (*Plan, error) {
	feedback := ""
	for attempt := 1; attempt <= 2; attempt++ {
		out, err := hostAgent(ctx, r.o.LeadCmd, r.root, leadPrompt(r.o.Task, r.o.MaxCards, feedback), filepath.Join(r.dir, "logs", fmt.Sprintf("lead-%d.log", attempt)))
		if err != nil {
			return nil, fmt.Errorf("the lead agent failed: %v", err)
		}
		plan, err := parsePlan(out)
		if err == nil {
			err = plan.Validate(r.o.MaxCards)
		}
		if err == nil {
			if plan.Task == "" {
				plan.Task = r.o.Task
			}
			r.saveJSON("plan.json", plan)
			r.ev.log("plan.ready", map[string]any{"cards": len(plan.Cards)})
			return plan, nil
		}
		r.ev.log("plan.refused", map[string]any{"reason": err.Error()})
		r.ui.say("  %s Plan refused, sent back to the lead: %s", r.ui.amber("!"), truncate(err.Error(), 120))
		feedback = err.Error()
	}
	return nil, fmt.Errorf("the lead's plan was refused twice: %s", feedback)
}

// writeExam runs the exam session in a throwaway worktree, then keeps its new
// files as a patch on this machine. The patch only enters the exam box.
func (r *runner) writeExam(ctx context.Context) ([]Check, []byte, error) {
	wt := filepath.Join(r.dir, "exam-worktree")
	if _, err := git(ctx, r.root, "worktree", "add", "-q", "--detach", wt, r.base); err != nil {
		return nil, nil, err
	}
	defer func() {
		_, _ = git(context.Background(), r.root, "worktree", "remove", "--force", wt)
	}()
	out, err := hostAgent(ctx, r.o.ExamCmd, wt, examPrompt(r.o.Task), filepath.Join(r.dir, "logs", "exam-writer.log"))
	if err != nil {
		return nil, nil, fmt.Errorf("the exam session failed: %v", err)
	}
	js, err := extractJSON(out)
	if err != nil {
		return nil, nil, fmt.Errorf("the exam session gave no checks: %v", err)
	}
	var e struct {
		Checks []Check `json:"checks"`
	}
	if err := json.Unmarshal([]byte(js), &e); err != nil {
		return nil, nil, fmt.Errorf("the exam's checks are not valid JSON: %v", err)
	}
	var checks []Check
	for _, c := range e.Checks {
		if strings.TrimSpace(c.Text) != "" && strings.TrimSpace(c.Command) != "" {
			checks = append(checks, Check{Text: c.Text, Command: c.Command})
		}
	}
	if len(checks) == 0 {
		return nil, nil, errors.New("the exam has no checks")
	}
	// Keep only new files with "taskwing_exam" in the path. Anything else the
	// session touched (caches, lock files, edits to existing files) is dropped.
	if _, err := git(ctx, wt, "add", "-A"); err != nil {
		return nil, nil, err
	}
	status, _ := git(ctx, wt, "diff", "--cached", "--name-status", "--no-renames", r.base)
	var keep, dropped []string
	for _, line := range strings.Split(status, "\n") {
		f := strings.SplitN(line, "\t", 2)
		if len(f) != 2 {
			continue
		}
		if f[0] == "A" && strings.Contains(f[1], "taskwing_exam") {
			keep = append(keep, f[1])
		} else {
			dropped = append(dropped, f[1])
		}
	}
	if len(keep) == 0 {
		return nil, nil, errors.New(`the exam session wrote no test files (their path must contain "taskwing_exam")`)
	}
	if len(dropped) > 0 {
		r.ev.log("exam.dropped", map[string]any{"files": dropped})
	}
	if _, err := git(ctx, wt, "reset", "-q"); err != nil {
		return nil, nil, err
	}
	if _, err := git(ctx, wt, append([]string{"add", "--"}, keep...)...); err != nil {
		return nil, nil, err
	}
	patch, err := git(ctx, wt, "diff", "--cached", "--binary", r.base)
	if err != nil {
		return nil, nil, err
	}
	if patch != "" {
		patch += "\n"
	}
	examDir := filepath.Join(r.dir, "exam")
	_ = os.MkdirAll(examDir, 0o755)
	_ = os.WriteFile(filepath.Join(examDir, "exam.patch"), []byte(patch), 0o644)
	b, _ := json.MarshalIndent(checks, "", "  ")
	_ = os.WriteFile(filepath.Join(examDir, "checks.json"), b, 0o644)
	r.ev.log("exam.ready", map[string]any{"checks": len(checks), "files": keep})
	return checks, []byte(patch), nil
}

func (r *runner) showPlan(p *Plan, checks []Check) {
	r.ui.say("\n  %s", r.ui.bold("The plan"))
	for i, c := range p.Cards {
		r.ui.say("  %02d %s  %s", i+1, c.Title, r.ui.dim(strings.Join(c.Owns, ", ")))
	}
	for _, s := range p.Seams {
		r.ui.say("     %s %s (owner %s)", r.ui.dim("seam:"), s.Contract, s.Owner)
	}
	r.ui.say("\n  %s", r.ui.bold("The exam")+r.ui.dim("  (written by a separate session; the agents never see it)"))
	for i, c := range checks {
		r.ui.say("  %d. %s", i+1, c.Text)
	}
}

func (r *runner) track(b *box.Box) {
	r.boxesMu.Lock()
	r.boxes = append(r.boxes, b)
	r.boxesMu.Unlock()
}

func (r *runner) untrack(b *box.Box) {
	r.boxesMu.Lock()
	defer r.boxesMu.Unlock()
	for i, x := range r.boxes {
		if x == b {
			r.boxes = append(r.boxes[:i], r.boxes[i+1:]...)
			return
		}
	}
}

// destroy scans a box for secrets, then deletes it (unless --keep).
func (r *runner) destroy(b *box.Box) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if r.o.Provider.Sandboxed() {
		var sb strings.Builder
		if _, err := r.o.Provider.Exec(ctx, b, secretScanScript, nil, &sb); err == nil {
			if found := findSecrets(sb.String()); len(found) > 0 {
				r.boxesMu.Lock()
				for _, f := range found {
					r.secrets = append(r.secrets, f+" in "+b.Name)
				}
				r.boxesMu.Unlock()
				r.ev.log("secret.found", map[string]any{"box": b.ID, "kinds": found})
			}
		}
	}
	if r.o.Keep {
		r.ev.log("box.kept", map[string]any{"box": b.ID})
		r.untrack(b)
		return
	}
	if err := r.o.Provider.Destroy(ctx, b); err != nil {
		r.ui.say("  %s could not delete box %s: %v (taskwing doctor finds it)", r.ui.amber("!"), b.ID, err)
	}
	r.ev.log("box.state", map[string]any{"box": b.ID, "state": "deleted"})
	r.untrack(b)
}

// githubCreds lists the GitHub credentials a fresh box already holds. Agents
// never start in such a box: they could push as the user.
func (r *runner) githubCreds(ctx context.Context, b *box.Box) []string {
	if !r.o.Provider.Sandboxed() {
		return nil
	}
	var sb strings.Builder
	if _, err := r.o.Provider.Exec(ctx, b, secretScanScript, nil, &sb); err != nil {
		return nil
	}
	var gh []string
	for _, f := range findSecrets(sb.String()) {
		if strings.Contains(f, "GitHub") || strings.Contains(f, "gh ") {
			gh = append(gh, f)
		}
	}
	return gh
}

// cleanup deletes any box still alive, for example after Ctrl-C or an error.
func (r *runner) cleanup() {
	r.boxesMu.Lock()
	left := append([]*box.Box(nil), r.boxes...)
	r.boxesMu.Unlock()
	var wg sync.WaitGroup
	for _, b := range left {
		wg.Add(1)
		go func(b *box.Box) { defer wg.Done(); r.destroy(b) }(b)
	}
	wg.Wait()
}

func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// upload copies a git bundle into the box and checks it out on a new branch.
func (r *runner) upload(ctx context.Context, b *box.Box, bundle, branch string) error {
	f, err := os.Open(bundle)
	if err != nil {
		return err
	}
	defer f.Close()
	// Base64 both ways, so no terminal layer in between can damage the bytes.
	pr, pw := io.Pipe()
	go func() {
		enc := base64.NewEncoder(base64.StdEncoding, pw)
		_, err := io.Copy(enc, f)
		if err == nil {
			err = enc.Close()
		}
		pw.CloseWithError(err)
	}()
	name, _ := git(ctx, r.root, "config", "user.name")
	email, _ := git(ctx, r.root, "config", "user.email")
	script := fmt.Sprintf(`set -e
rm -rf "$TW_REPO" "$TW_REPO.bundle"
mkdir -p "$(dirname "$TW_REPO")"
base64 -d > "$TW_REPO.bundle"
git clone -q "$TW_REPO.bundle" "$TW_REPO" </dev/null
cd "$TW_REPO"
git checkout -q -B %s </dev/null
git remote remove origin </dev/null || true
git config user.name %s
git config user.email %s
`, shq(branch), shq(name), shq(email))
	var out strings.Builder
	rc, err := r.o.Provider.Exec(ctx, b, script, pr, &out)
	pr.Close()
	if err != nil {
		return err
	}
	if rc != 0 {
		return fmt.Errorf("copying the repository into box %s failed (exit %d): %s", b.ID, rc, lastLines(out.String(), 3))
	}
	return nil
}

func (r *runner) build(ctx context.Context, plan *Plan) ([]*cardRun, error) {
	bundle := filepath.Join(r.dir, "repo.bundle")
	if _, err := git(ctx, r.root, "bundle", "create", "-q", bundle, "HEAD"); err != nil {
		if _, err2 := git(ctx, r.root, "bundle", "create", bundle, "HEAD"); err2 != nil {
			return nil, err
		}
	}
	cards := make([]*cardRun, len(plan.Cards))
	for i, c := range plan.Cards {
		cards[i] = &cardRun{card: c, n: i + 1, state: stWaiting, thought: "starting a sandbox", started: time.Now()}
	}
	var wg sync.WaitGroup
	for _, cr := range cards {
		wg.Add(1)
		go func(cr *cardRun) {
			defer wg.Done()
			r.runCard(ctx, plan, cr, bundle)
		}(cr)
	}

	// The live board, until every agent has finished. On Ctrl-C each card
	// stops its own agent and the boxes are cleaned up after.
	r.watch("Build", time.Now(), cards, &wg)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	var ok int
	for _, cr := range cards {
		if cr.state == stDone {
			ok++
		}
	}
	if ok == 0 {
		return cards, errors.New("no agent finished its piece; logs are in " + filepath.Join(r.dir, "logs"))
	}
	return cards, nil
}

// watch draws the live squares until wg is done.
func (r *runner) watch(stage string, started time.Time, cards []*cardRun, wg *sync.WaitGroup) {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	last := map[int]string{}
	for loop := true; loop; {
		select {
		case <-done:
			loop = false
		case <-tick.C:
		}
		views := make([]cardView, len(cards))
		for i, cr := range cards {
			views[i] = cr.view()
			if !r.ui.tty && last[i] != views[i].state {
				last[i] = views[i].state
				r.ui.say("  %02d %s %s  %s", views[i].n, views[i].state, views[i].title, views[i].thought)
			}
		}
		r.ui.board(stage, started, views)
	}
	r.ui.release()
}

// runCard creates the card's box, runs its agent, and pulls its commits back.
func (r *runner) runCard(ctx context.Context, plan *Plan, cr *cardRun, bundle string) {
	fail := func(msg string) {
		cr.set(func(c *cardRun) { c.state, c.err, c.ended = stFailed, msg, time.Now() })
		r.ev.log("box.state", map[string]any{"card": cr.card.ID, "state": stFailed, "why": msg})
	}
	name := fmt.Sprintf("%s · %s · %02d %s", BoxPrefix, r.id, cr.n, cr.card.Title)
	b, err := r.o.Provider.Create(ctx, name)
	if err != nil {
		fail("no sandbox: " + err.Error())
		return
	}
	r.track(b)
	defer r.destroy(b)
	cr.set(func(c *cardRun) { c.box, c.thought = b, "checking the sandbox for GitHub credentials" })
	if found := r.githubCreds(ctx, b); len(found) > 0 {
		fail("this sandbox holds " + strings.Join(found, ", ") + "; make a base box without them (docs/poc.md)")
		return
	}
	cr.set(func(c *cardRun) { c.thought = "copying the repository" })
	r.ev.log("box.state", map[string]any{"card": cr.card.ID, "box": b.ID, "state": "created"})
	if err := r.upload(ctx, b, bundle, "taskwing/"+cr.card.ID); err != nil {
		fail(err.Error())
		return
	}
	shown := strings.Replace(b.RepoDir, "$HOME", "~", 1)
	logPath := filepath.Join(r.dir, "logs", fmt.Sprintf("agent-%02d.log", cr.n))
	agent, err := r.o.Provider.StartAgent(ctx, b, workerPrompt(plan, cr.card, cr.n, shown), logPath)
	if err != nil {
		fail("the agent did not start: " + err.Error())
		return
	}
	cr.set(func(c *cardRun) {
		c.agent, c.state, c.thought, c.started = agent, stWorking, "reading its piece", time.Now()
	})
	r.ev.log("box.state", map[string]any{"card": cr.card.ID, "box": b.ID, "state": stWorking})

	deadline := time.Now().Add(r.o.AgentTimeout)
	for {
		st := agent.Status()
		if !st.Running {
			if st.Failed {
				fail("the agent failed: " + truncate(st.Err, 80))
				return
			}
			cr.set(func(c *cardRun) { c.final, c.proof = st.Final, proofLine(st.Final) })
			break
		}
		if st.Thought != "" {
			cr.set(func(c *cardRun) { c.thought = st.Thought })
		}
		if time.Now().After(deadline) {
			agent.Stop()
			fail(fmt.Sprintf("ran over %s and was stopped", dur(r.o.AgentTimeout)))
			return
		}
		select {
		case <-ctx.Done():
			agent.Stop()
			fail("stopped")
			return
		case <-time.After(time.Second):
		}
	}

	// Pull the agent's commits back to this machine. Uncommitted work is committed for it.
	cr.set(func(c *cardRun) { c.thought = "sending its commits back" })
	collect := fmt.Sprintf(`cd "$TW_REPO" || exit 3
if [ -n "$(git status --porcelain)" ]; then git add -A && git commit -q -m %s </dev/null || exit 4; fi
[ "$(git rev-list --count %s..HEAD)" -gt 0 ] || exit 5
git bundle create "$TW_REPO.out" %s..HEAD >/dev/null 2>&1 </dev/null || exit 6
base64 < "$TW_REPO.out"
`, shq(cr.card.Title), r.base, r.base)
	out := filepath.Join(r.dir, "logs", fmt.Sprintf("agent-%02d.bundle", cr.n))
	var enc bytes.Buffer
	rc, err := r.o.Provider.Exec(ctx, b, collect, nil, &enc)
	if err == nil && rc == 0 {
		raw, derr := base64.StdEncoding.DecodeString(strings.Map(func(c rune) rune {
			if c == '\n' || c == '\r' || c == ' ' {
				return -1
			}
			return c
		}, enc.String()))
		if derr == nil {
			derr = os.WriteFile(out, raw, 0o644)
		}
		if derr != nil {
			fail("could not read its commits: " + derr.Error())
			return
		}
	}
	switch {
	case err != nil:
		fail("could not read its commits: " + err.Error())
		return
	case rc == 5:
		fail("finished without changing anything")
		return
	case rc != 0:
		fail(fmt.Sprintf("could not bundle its commits (exit %d)", rc))
		return
	}
	ref := fmt.Sprintf("refs/taskwing/%s/%s", r.id, cr.card.ID)
	if _, err := git(ctx, r.root, "fetch", "-q", out, "HEAD:"+ref); err != nil {
		fail("could not fetch its commits: " + err.Error())
		return
	}
	files, _ := git(ctx, r.root, "diff", "--name-only", r.base, ref)
	var list []string
	if files != "" {
		list = strings.Split(files, "\n")
	}
	outside := cr.card.Outside(list)
	cr.set(func(c *cardRun) {
		c.ref, c.files, c.outside, c.state, c.ended = ref, list, outside, stDone, time.Now()
	})
	r.ev.log("box.state", map[string]any{"card": cr.card.ID, "box": b.ID, "state": stDone, "files": len(list), "outside": outside, "proof": cr.proof})
}

func (r *runner) join(ctx context.Context, cards []*cardRun, branch string) error {
	wt := filepath.Join(r.dir, "join")
	if _, err := git(ctx, r.root, "worktree", "add", "-q", "-b", branch, wt, r.base); err != nil {
		return err
	}
	var joined, conflicts []string
	for _, cr := range cards {
		if cr.state != stDone {
			continue
		}
		msg := fmt.Sprintf("Join %02d: %s", cr.n, cr.card.Title)
		if _, err := git(ctx, wt, "merge", "--no-ff", "--no-edit", "-m", msg, cr.ref); err != nil {
			_, _ = git(ctx, wt, "merge", "--abort")
			conflicts = append(conflicts, fmt.Sprintf("%02d %s", cr.n, cr.card.Title))
			r.ev.log("join.conflict", map[string]any{"card": cr.card.ID})
			continue
		}
		cr.joined = true
		joined = append(joined, fmt.Sprintf("%02d", cr.n))
	}
	r.ui.say("  Joined %d %s on branch %s", len(joined), plural(len(joined), "piece", "pieces"), branch)
	for _, c := range conflicts {
		r.ui.say("  %s %s conflicts with the others and was left out", r.ui.red("✗"), c)
	}
	for _, cr := range cards {
		if len(cr.outside) > 0 {
			r.ui.say("  %s %02d changed files outside its area: %s", r.ui.amber("!"), cr.n, truncate(strings.Join(cr.outside, ", "), 100))
		}
	}
	if len(joined) == 0 {
		return errors.New("nothing could be joined")
	}
	return nil
}

func (r *runner) exam(ctx context.Context, checks []Check, patch []byte) error {
	wt := filepath.Join(r.dir, "join")
	bundle := filepath.Join(r.dir, "joined.bundle")
	if _, err := git(ctx, wt, "bundle", "create", "-q", bundle, "HEAD"); err != nil {
		if _, err2 := git(ctx, wt, "bundle", "create", bundle, "HEAD"); err2 != nil {
			return err
		}
	}
	r.ui.say("  Starting a clean sandbox for the exam...")
	b, err := r.o.Provider.Create(ctx, fmt.Sprintf("%s · %s · exam", BoxPrefix, r.id))
	if err != nil {
		return fmt.Errorf("no exam sandbox: %w", err)
	}
	r.track(b)
	defer r.destroy(b)
	if err := r.upload(ctx, b, bundle, "taskwing/exam"); err != nil {
		return err
	}
	if len(patch) > 0 {
		var out strings.Builder
		rc, err := r.o.Provider.Exec(ctx, b, `cd "$TW_REPO" && git apply --index --whitespace=nowarn`, strings.NewReader(string(patch)), &out)
		if err != nil || rc != 0 {
			return fmt.Errorf("the exam's tests did not apply on the joined work (exit %d): %v %s", rc, err, lastLines(out.String(), 3))
		}
	}
	for i := range checks {
		c := &checks[i]
		cctx, cancel := context.WithTimeout(ctx, r.o.CheckTimeout)
		var out strings.Builder
		script := fmt.Sprintf("cd \"$TW_REPO\" || exit 99\n( %s ) > \"$TW_REPO.check.log\" 2>&1 </dev/null\nrc=$?\ntail -n 40 \"$TW_REPO.check.log\"\nexit $rc\n", c.Command)
		rc, err := r.o.Provider.Exec(cctx, b, script, nil, &out)
		cancel()
		c.Output = lastLines(out.String(), 40)
		c.Passed = err == nil && rc == 0
		mark := r.ui.green("✓")
		if !c.Passed {
			mark = r.ui.red("✗")
		}
		r.ui.say("  %s %s", mark, c.Text)
		r.ev.log("exam.check", map[string]any{"n": i + 1, "passed": c.Passed})
	}
	return nil
}

func (r *runner) summary(res *Result) {
	r.ui.say("")
	score := fmt.Sprintf("%d of %d checks passed", res.Passed, res.Total)
	if res.Passed == res.Total {
		score = r.ui.green(score)
	} else {
		score = r.ui.red(score)
	}
	r.ui.say("%s  %s · %s", r.ui.bold("Done"), score, dur(time.Since(r.start)))
	if res.PRURL != "" {
		r.ui.say("  Pull request: %s", res.PRURL)
	}
	if r.o.Provider.Sandboxed() {
		if len(res.Secrets) == 0 {
			r.ui.say("  Secret scan: nothing found in any sandbox")
		} else {
			r.ui.say("  %s Secret scan found: %s", r.ui.red("!"), strings.Join(res.Secrets, "; "))
		}
	}
	r.ui.say("  Run log: %s", r.dir)
}
