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
	Yes          bool     // skip the questions
	PlanOnly     bool     // stop after the plan and the exam; start it later with Approve
	Approve      string   // run id of a plan saved by PlanOnly: build it without planning again
	Checks       []string // checks of the user's own, added with Approve
	NoPR         bool     // stop after the exam; keep the branch locally
	Keep         bool     // keep boxes after the run
	Hunt         bool     // find bugs, prove each with a failing test, then fix them
	HuntCmd      string   // shell command for the bug hunter; prompt on stdin
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
	Scanned bool // the boxes were scanned for secrets (false in local mode)
	Stopped bool
}

type cardRun struct {
	mu      sync.Mutex
	card    Card
	n       int
	state   string
	thought string
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

	examNow     chan struct{} // closed by "e" in the terminal
	examNowOnce sync.Once

	boxesMu    sync.Mutex
	boxes      []*box.Box
	secrets    []string
	agentLogin bool // a box held the agent's own login (expected)
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
	if o.Approve != "" {
		p, err := readPlanned(o.Home, o.Approve)
		if err != nil {
			return nil, err
		}
		o.Task, o.Hunt, o.RunID = p.Task, p.Hunt, o.Approve
	}
	r := &runner{o: o, s: o.Stream, ui: newUI(o.Out, o.In), start: time.Now(), examNow: make(chan struct{})}
	res, err := r.run(ctx)
	// Whatever failed on the way, a run that ended because ctx did is a stop.
	if err != nil && ctx.Err() != nil && !errors.Is(err, ctx.Err()) {
		err = fmt.Errorf("%w (%v)", ctx.Err(), err)
	}
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

// preflight checks everything a run needs before it starts the lead, so a
// missing login or setup fails in a second, not after the plan.
func (r *runner) preflight(ctx context.Context) error {
	o := r.o
	where := "run taskwing inside a git repository"
	if r.s != nil {
		where = o.Dir + " is not a git repository"
	}
	var err error
	if r.root, err = git(ctx, o.Dir, "rev-parse", "--show-toplevel"); err != nil {
		return ctxOr(ctx, errors.New(where))
	}
	st, err := git(ctx, r.root, "status", "--porcelain")
	if err != nil {
		return ctxOr(ctx, fmt.Errorf("could not read the repository's status: %w", err))
	}
	if st != "" {
		return errors.New("you have uncommitted changes; commit or stash them first, because the agents start from your last commit")
	}
	if r.base, err = git(ctx, r.root, "rev-parse", "HEAD"); err != nil {
		return ctxOr(ctx, errors.New("the repository has no commits yet"))
	}
	// Agents commit in their boxes and the engine merges here, both with the
	// user's git name and email.
	if _, err := git(ctx, r.root, "var", "GIT_COMMITTER_IDENT"); err != nil {
		return ctxOr(ctx, errors.New(`git has no name and email for commits: git config --global user.name "Your Name"; git config --global user.email you@example.com`))
	}
	r.branch, _ = git(ctx, r.root, "rev-parse", "--abbrev-ref", "HEAD")
	if !o.NoPR {
		noPR := ", or pass --no-pr"
		if r.s != nil {
			noPR = `, or start the run with "no_pr": true`
		}
		if _, err := git(ctx, r.root, "remote", "get-url", "origin"); err != nil {
			return ctxOr(ctx, errors.New("no origin remote to push to; add one"+noPR))
		}
		if out, err := exec.CommandContext(ctx, "gh", "auth", "status").CombinedOutput(); err != nil {
			return ctxOr(ctx, withDetail(errors.New("the GitHub CLI is not logged in (gh auth login)"+noPR), lastLines(string(out), 2)))
		}
		if r.branch == "HEAD" {
			return errors.New("you are on a detached HEAD; check out the branch the pull request should target")
		}
	}
	if c, ok := o.Provider.(interface{ Check() error }); ok {
		if err := c.Check(); err != nil {
			return err
		}
	}
	return claudeLoggedIn(ctx, o)
}

// ctxOr returns the context's error when it ended (Ctrl-C, or the client
// quit), so a stop is reported as a stop and not as the git error it caused.
func ctxOr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
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
	if err := r.preflight(ctx); err != nil {
		return nil, err
	}
	var err error
	r.id = o.RunID
	if r.id == "" {
		r.id = NewRunID()
	}
	r.dir = filepath.Join(o.Home, "runs", r.id)
	if err := os.MkdirAll(filepath.Join(r.dir, "logs"), dirPerm); err != nil {
		return nil, err
	}
	if r.ev, err = openEvents(r.dir); err != nil {
		return nil, err
	}
	res := &Result{RunID: r.id, RunDir: r.dir, Scanned: o.Provider.Sandboxed()}
	// An approve logs run.approved once the saved plan is accepted, below.
	if o.Approve == "" {
		r.ev.log("run.created", map[string]any{"task": o.Task, "repo": r.root, "base": r.base, "provider": o.Provider.Name(), "agent": r.agent()})
	}
	r.ui.say("%s %s  %s", r.ui.bold("TaskWing"), r.id, r.ui.dim(o.Provider.Name()+" · "+filepath.Base(r.root)+" @ "+r.base[:8]))
	if !o.Provider.Sandboxed() {
		r.ui.say("%s", r.ui.amber("Local mode: agents run on this machine with your own login. This is for testing, not a sandbox."))
	}

	// 1. Plan and exam (or hunt), then the user's yes. The exam session never
	// sees the plan, and the lead never sees the exam.
	var (
		plan       *Plan
		checks     []Check
		patch      []byte
		what       string
		userChecks []WireCheck // checks the user typed at the prompt
		userExam   chan userExamResult
	)
	// The session writing the user's checks stops when the run does, however
	// it ends, and is waited for, so it never outlives the run.
	uctx, stopUserExam := context.WithCancel(ctx)
	defer func() {
		stopUserExam()
		if userExam != nil {
			<-userExam
		}
	}()
	if o.Hunt {
		r.ui.step(1, "Hunt")
	} else {
		r.ui.step(1, "Plan")
	}
	if o.Approve != "" {
		// The plan and the exam were made and shown by an earlier
		// --plan-only run; the user approved them since.
		if plan, checks, patch, err = r.loadPlanned(); err != nil {
			return res, err
		}
		r.ev.log("run.approved", map[string]any{"base": r.base, "provider": o.Provider.Name(), "user_checks": len(o.Checks)})
		if o.Hunt {
			r.showBugs(plan)
		} else {
			r.showPlan(plan, checks)
		}
		for i, text := range o.Checks {
			if text = strings.TrimSpace(text); text != "" {
				userChecks = append(userChecks, WireCheck{ID: fmt.Sprintf("u%d", i+1), Text: text, AddedBy: "user"})
				r.ui.say("  %s %s %s", r.ui.green("+"), text, r.ui.dim("(its tests are written while the agents build; they never see them)"))
			}
		}
	}
	for approved := o.Approve != ""; !approved; {
		if o.Hunt {
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
			r.ui.say("  The lead agent is splitting the task. A separate session is writing the exam.")
			if plan, checks, patch, err = r.planAndExam(ctx); err != nil {
				return res, err
			}
			r.showPlan(plan, checks)
			what = fmt.Sprintf("Start %d %s in %s? This is the exam they must pass.", len(plan.Cards), plural(len(plan.Cards), "agent", "agents"), sandboxWord(o.Provider))
		}
		var redo bool
		switch {
		case r.s != nil:
			// The client shows the plan; the user approves it there (with
			// checks of their own) or asks the lead for a new one.
			c, err := r.s.askApproval(ctx, plan, checks)
			if err != nil {
				return res, err
			}
			if c.Type == "plan.approve" {
				userChecks, approved = c.Checks, true
			} else {
				redo = true
			}
		case o.PlanOnly:
			if err := r.savePlanned(); err != nil {
				return res, err
			}
			r.ev.log("run.planned", nil)
			approve := "taskwing approve " + r.id + r.approveFlags()
			r.ui.say("\n  Plan ready. Nothing has started.")
			r.ui.say("  Approve it:  %s", approve)
			r.ui.say("  %s", r.ui.dim(`Add a check of your own: `+approve+` --check "<sentence>"`))
			res.Stopped = true
			return res, nil
		case o.Yes:
			approved = true
		default:
			answer, err := r.askPlan(ctx, what, &userChecks)
			if err != nil {
				return res, err
			}
			switch answer {
			case "y":
				approved = true
			case "r":
				redo = true
			default:
				r.ev.log("run.stopped", nil)
				r.ui.say("Stopped. The plan and the exam are saved in %s", r.dir)
				res.Stopped = true
				return res, nil
			}
		}
		if redo {
			r.ev.log("plan.redo", nil)
			r.forgetExam()
			r.s.stage("planning")
			r.ui.say("\n  Asking for a new plan.")
		}
	}
	userExam = r.startUserExam(uctx, plan.Interface, userChecks)
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
	// Every box is scanned and deleted by now (agent boxes in build, the exam
	// box when exam returned): the summary and the pull request report this.
	r.boxesMu.Lock()
	res.Secrets = append([]string(nil), r.secrets...)
	r.boxesMu.Unlock()
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
	if res.Passed < res.Total && !o.Yes && o.Approve != "" && !r.ui.inTTY {
		// approve is run by coding agents, whose shell cannot answer: asking
		// would wait forever on an input that never ends.
		r.ui.say("  Not opened: %d of %d checks failed. The joined work is on branch %s.", res.Total-res.Passed, res.Total, res.Branch)
		r.summary(res)
		return res, nil
	}
	if res.Passed < res.Total && !o.Yes {
		yes, err := r.ui.ask(ctx, fmt.Sprintf("  %d of %d checks failed. Open the draft pull request anyway?", res.Total-res.Passed, res.Total))
		if err != nil {
			return res, err
		}
		if !yes {
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

// approveFlags repeats the flags of this run that the saved plan does not
// keep, so the printed approve command builds where the plan was made for
// (without them approve falls back to boat and opens a pull request).
func (r *runner) approveFlags() string {
	f := " --provider " + r.o.Provider.Name()
	if b, ok := r.o.Provider.(*box.Boat); ok && b.Base != "" {
		f += " --boat-base " + shq(b.Base)
	}
	if r.o.NoPR {
		f += " --no-pr"
	}
	if r.o.Keep {
		f += " --keep"
	}
	return f
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

// askPlan asks for the user's yes on a plan. Besides y and n, the user can
// type r for a new plan, or any sentence to add a check of their own; it is
// listed and the question comes again. It returns "y", "n" or "r".
func (r *runner) askPlan(ctx context.Context, what string, mine *[]WireCheck) (string, error) {
	hint := r.ui.dim("  y start · n stop · r new plan · or type a check of your own to add it")
	for {
		line, err := r.ui.prompt(ctx, "\n"+what+"\n"+hint+"\n> ")
		if err != nil {
			return "", err
		}
		switch strings.ToLower(line) {
		case "y", "yes":
			return "y", nil
		case "r", "redo":
			return "r", nil
		case "", "n", "no", "q":
			return "n", nil
		}
		c := WireCheck{ID: fmt.Sprintf("u%d", len(*mine)+1), Text: line, AddedBy: "user"}
		*mine = append(*mine, c)
		r.ev.log("exam.user.added", map[string]any{"id": c.ID})
		r.ui.say("  %s %s %s", r.ui.green("+"), line, r.ui.dim("(its tests are written while the agents build; they never see them)"))
	}
}

// saveJSON writes v into the run folder. The run goes on if it fails (these
// files are records, not inputs), but the user hears about it.
func (r *runner) saveJSON(name string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(r.dir, name), b, filePerm)
	}
	if err != nil {
		r.ui.say("  %s could not save %s: %v", r.ui.amber("!"), name, err)
	}
}

// forgetExam deletes the last exam before the lead plans again: the lead's
// tools can read files on this machine, and it must never see the tests.
func (r *runner) forgetExam() {
	_ = os.RemoveAll(filepath.Join(r.dir, "exam"))
	_ = os.Remove(filepath.Join(r.dir, "logs", "exam-writer.log"))
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
				msg += " (only the agents' own login, as expected)"
			}
			r.ui.say("%s", msg)
		} else {
			r.ui.say("  %s Secret scan found: %s", r.ui.red("!"), strings.Join(res.Secrets, "; "))
		}
	}
	r.ui.say("  Run log: %s", r.dir)
}
