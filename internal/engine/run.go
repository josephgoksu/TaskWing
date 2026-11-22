// Package engine runs one TaskWing task: plan, exam, build, join, exam, pull request.
package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

	"github.com/josephgoksu/TaskWing/v2/internal/box"
)

// BoxPrefix starts the name of every box TaskWing creates.
const BoxPrefix = "taskwing"

// RunBoxPrefix starts the name of every box a run creates ("taskwing · tw-<time> · ...").
// doctor only touches boxes with this prefix, never a base box named by hand.
const RunBoxPrefix = BoxPrefix + " · tw-"

// BranchPrefix starts every branch a run makes: taskwing/<run id> for the
// joined work, taskwing/<card id> inside a box.
const BranchPrefix = "taskwing/"

// AgentRefPrefix holds each agent's commits on this machine:
// refs/taskwing/<run id>/<card id>. doctor deletes them.
const AgentRefPrefix = "refs/taskwing/"

// NewRunID names a run: tw-<date>-<time>-<4 hex>. The random part keeps two
// runs started in the same second (two terminals, two engines) apart; they
// would otherwise share a run folder, a branch and box names.
func NewRunID() string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return "tw-" + time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

// Options configure one run.
type Options struct {
	Task         string
	Dir          string // any folder inside the repository
	Provider     box.Provider
	Home         string // state folder, usually ~/.taskwing
	LeadCmd      string // shell command for the lead; prompt on stdin
	ExamCmd      string // shell command for the exam session; prompt on stdin
	MaxCards     int
	Yes          bool   // skip the questions
	NoPR         bool   // stop after the exam; keep the branch locally
	Keep         bool   // keep boxes after the run
	Hunt         bool   // find bugs, prove each with a failing test, then fix them
	HuntCmd      string // shell command for the bug hunter; prompt on stdin
	AgentTimeout time.Duration
	CheckTimeout time.Duration
	Out          io.Writer
	In           io.Reader
	// Stream connects the run to a client (taskwing engine --json).
	// With a stream the run asks the client, not In, and never opens a pull
	// request when a check fails.
	Stream *Stream
	RunID  string // set by the stream; otherwise made from the time
}

// Check is one exam check: a plain sentence and the command behind it.
type Check struct {
	ID      string `json:"id,omitempty"` // k1, k2... or the client's id for a check the user added
	Text    string `json:"text"`
	Command string `json:"command"`
	Passed  bool   `json:"passed"`
	Output  string `json:"output,omitempty"`
	User    bool   `json:"user,omitempty"` // added by the user in the client
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
	early   bool // stopped when the user asked for the exam
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
	if c.state == stStuck {
		v.thought = c.err
	}
	return v
}

type runner struct {
	o      Options
	s      *Stream // nil for taskwing run and hunt
	ui     *ui
	ev     *events
	root   string
	base   string
	branch string // the branch the user is on; the PR targets it
	id     string
	dir    string
	start  time.Time

	boxesMu    sync.Mutex
	boxes      []*box.Box
	secrets    []string
	agentLogin bool // a box held the agent's Claude login (expected)
}

// Run executes one task end to end.
func Run(ctx context.Context, o Options) (*Result, error) {
	if o.MaxCards <= 0 {
		o.MaxCards = 4
	}
	if o.AgentTimeout == 0 {
		o.AgentTimeout = 90 * time.Minute
	}
	if o.CheckTimeout == 0 {
		o.CheckTimeout = 20 * time.Minute
	}
	r := &runner{o: o, s: o.Stream, ui: newUI(o.Out, o.In), start: time.Now()}
	res, err := r.run(ctx)
	r.cleanup()
	if res != nil {
		// Boxes deleted on the way to an error were scanned too.
		r.boxesMu.Lock()
		res.Secrets = append([]string(nil), r.secrets...)
		r.boxesMu.Unlock()
	}
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
	if strings.TrimSpace(o.Task) == "" && !o.Hunt {
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
	if err := claudeLoggedIn(ctx, o); err != nil {
		return nil, err
	}
	r.id = o.RunID
	if r.id == "" {
		r.id = NewRunID()
	}
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
	var (
		plan   *Plan
		checks []Check
		patch  []byte
		what   string
	)
	if o.Hunt {
		r.ui.step(1, "Hunt")
		r.ui.say("  A session is reading the code and writing a failing test for each bug it suspects.")
		if plan, checks, patch, err = r.hunt(ctx); err != nil {
			return res, err
		}
		if len(plan.Cards) == 0 {
			r.ui.say("\n  No bug could be proven with a failing test. Nothing to fix.")
			res.Stopped = true
			return res, nil
		}
		r.showBugs(plan)
		what = fmt.Sprintf("Fix %d %s, one agent each in %s?", len(plan.Cards), plural(len(plan.Cards), "bug", "bugs"), sandboxWord(o.Provider))
	} else {
		r.ui.step(1, "Plan")
		r.ui.say("  The lead agent is splitting the task. A separate session is writing the exam.")
		if plan, checks, patch, err = r.planAndExam(ctx); err != nil {
			return res, err
		}
		r.showPlan(plan, checks)
		what = fmt.Sprintf("Start %d %s in %s? This is the exam they must pass.", len(plan.Cards), plural(len(plan.Cards), "agent", "agents"), sandboxWord(o.Provider))
	}
	// With a client the user approves the plan there, may add checks of their
	// own, or asks the lead for a new plan.
	var userExam chan userExamResult
	// The session writing the user's checks stops when the run does, however
	// it ends, and is waited for, so it never outlives the run.
	uctx, stopUserExam := context.WithCancel(ctx)
	defer func() {
		stopUserExam()
		if userExam != nil {
			<-userExam
		}
	}()
	if r.s != nil {
		for {
			c, err := r.s.askApproval(ctx, plan, checks)
			if err != nil {
				return res, err
			}
			if c.Type == "plan.approve" {
				userExam = r.startUserExam(uctx, plan.Interface, c.Checks)
				break
			}
			r.ev.log("plan.redo", nil)
			r.s.stage("planning")
			if plan, checks, patch, err = r.planAndExam(ctx); err != nil {
				return res, err
			}
		}
	} else if !o.Yes && !r.ui.ask("\n"+what) {
		r.ev.log("run.stopped", nil)
		r.ui.say("Stopped. The plan and the exam are saved in %s", r.dir)
		res.Stopped = true
		return res, nil
	}
	// Set only once the exam will run: a stop above is not a failed exam (exit 3).
	res.Total = len(checks)
	r.ev.log("exam.approved", map[string]any{"checks": len(checks)})

	// 2. Build: one box and one agent per card.
	r.ui.step(2, "Build")
	r.s.stage("build")
	cards, err := r.build(ctx, plan)
	if err != nil {
		return res, err
	}

	// The user's own checks were written into tests while the agents built.
	var userPatch []byte
	if userExam != nil {
		var u userExamResult
		select {
		case u = <-userExam:
			userExam = nil
		case <-ctx.Done():
			return res, ctx.Err()
		}
		checks = append(checks, u.checks...)
		userPatch = u.patch
		res.Total = len(checks)
	}

	// 3. Join the work on one branch and run the exam in a clean box.
	r.ui.step(3, "Exam")
	r.s.stage("exam")
	res.Branch = BranchPrefix + r.id
	// Free the branch so the user can check it out (or delete it), even when
	// the join fails; it stays in the repository.
	defer func() {
		_, _ = git(context.Background(), r.root, "worktree", "remove", "--force", filepath.Join(r.dir, "join"))
	}()
	if err := r.join(ctx, cards, res.Branch); err != nil {
		return res, err
	}
	for _, cr := range cards {
		if cr.state == stDone && !cr.joined {
			r.s.leftOut(cr)
		}
	}
	if err := r.exam(ctx, checks, patch, userPatch); err != nil {
		return res, err
	}
	r.s.examResult(checks)
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
	if res.Passed < res.Total && r.s != nil {
		// With a client, a failed exam pushes nothing (taskwing run asks first).
		r.ui.say("  Not opened: %d of %d checks failed. The joined work is on branch %s.", res.Total-res.Passed, res.Total, res.Branch)
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
	r.s.stage("pr")
	url, err := r.openPR(ctx, plan, cards, checks, res)
	if err != nil {
		return res, err
	}
	res.PRURL = url
	r.ev.log("pr.opened", map[string]any{"url": url})
	r.s.prOpened(url)
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
			msg := "  Secret scan: nothing found in any sandbox"
			if r.agentLogin {
				msg += " (only the agents' own Claude login, as expected)"
			}
			r.ui.say(msg)
		} else {
			r.ui.say("  %s Secret scan found: %s", r.ui.red("!"), strings.Join(res.Secrets, "; "))
		}
	}
	r.ui.say("  Run log: %s", r.dir)
}
