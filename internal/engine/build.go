package engine

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

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
	r.watchUntil(stage, started, cards, done)
}

// watchUntil draws the live squares until done is closed.
func (r *runner) watchUntil(stage string, started time.Time, cards []*cardRun, done chan struct{}) {
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
			r.s.card(cr)
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
		cr.set(func(c *cardRun) { c.state, c.err, c.ended = stStuck, msg, time.Now() })
		r.ev.log("box.state", map[string]any{"card": cr.card.ID, "state": stStuck, "why": msg})
	}
	// Once the user asked for the exam, an agent that has not started never will.
	examStarted := func() bool {
		select {
		case <-r.s.earlyExam():
			fail("the exam started before this agent did")
			return true
		default:
			return false
		}
	}
	if examStarted() {
		return
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
	found, err := r.githubCreds(ctx, b)
	if err != nil {
		fail("could not check the sandbox for GitHub credentials: " + err.Error())
		return
	}
	if len(found) > 0 {
		fail("this sandbox holds " + strings.Join(found, ", ") + "; make a base box without them (docs/poc.md)")
		return
	}
	cr.set(func(c *cardRun) { c.thought = "copying the repository" })
	r.ev.log("box.state", map[string]any{"card": cr.card.ID, "box": b.ID, "state": "created"})
	if err := r.upload(ctx, b, bundle, BranchPrefix+cr.card.ID); err != nil {
		fail(err.Error())
		return
	}
	if examStarted() {
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
	early := false
	for running := true; running; {
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
		case <-r.s.earlyExam():
			// The user wants the exam now: keep what this agent has so far.
			agent.Stop()
			cr.set(func(c *cardRun) { c.early, c.proof = true, "stopped early for the exam" })
			r.ev.log("box.state", map[string]any{"card": cr.card.ID, "state": "stopped-early"})
			running, early = false, true
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
	case rc == 5 && early:
		fail("stopped for the exam before it changed anything")
		return
	case rc == 5:
		fail("finished without changing anything")
		return
	case rc != 0:
		fail(fmt.Sprintf("could not bundle its commits (exit %d)", rc))
		return
	}
	ref := AgentRefPrefix + r.id + "/" + cr.card.ID
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
