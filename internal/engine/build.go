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
	repoBundle := filepath.Join(r.dir, "repo.bundle")
	if err := bundle(ctx, r.root, repoBundle); err != nil {
		return nil, err
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
			r.runCard(ctx, plan, cr, repoBundle)
		}(cr)
	}

	// In the terminal, "e" + Enter asks for the exam now, like exam.now
	// does for a client.
	building := make(chan struct{})
	// Only a person at a terminal: piped answers meant for a later question
	// must not be read here.
	if r.s == nil && r.ui.inTTY {
		if r.ui.tty {
			r.ui.hint = "type e + Enter to run the exam now with what the agents have so far"
		}
		go func() {
			for {
				select {
				case line, ok := <-r.ui.lines():
					if !ok {
						return
					}
					if strings.EqualFold(line, "e") {
						r.examNowOnce.Do(func() { close(r.examNow) })
						r.ui.say("  Running the exam now: each agent stops and its work so far is kept.")
						return
					}
				case <-building:
					return
				}
			}
		}()
	}
	// The live board, until every agent has finished. On Ctrl-C each card
	// stops its own agent and the boxes are cleaned up after.
	r.watch("Build", time.Now(), cards, &wg)
	close(building)
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
		r.ui.board(started, views)
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
		case <-r.earlyExam():
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
	cr.set(func(c *cardRun) { c.thought = "checking the sandbox for push credentials" })
	found, err := r.pushCreds(ctx, b)
	if err != nil {
		fail("could not check the sandbox for push credentials: " + err.Error())
		return
	}
	if len(found) > 0 {
		fail("the base box has " + strings.Join(found, ", ") + ": make one without (docs/guide.md)")
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
		c.state, c.thought, c.started = stWorking, "reading its piece", time.Now()
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
			cr.set(func(c *cardRun) { c.proof = proofLine(st.Final) })
			break
		}
		if st.Thought != "" {
			// An agent's last words may already be its "PROOF: ..." line; the
			// board shows the proof, not the marker.
			thought := st.Thought
			if p := proofLine(thought); p != "" {
				thought = p
			}
			cr.set(func(c *cardRun) { c.thought = thought })
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
		case <-r.earlyExam():
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
			derr = os.WriteFile(out, raw, filePerm)
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
	// --no-renames: a rename would list only the new path and hide that the
	// agent deleted a file another card owns. -z: paths with spaces, quotes
	// or non-ASCII letters come back as they are, not quoted.
	files, err := gitRaw(ctx, r.root, "diff", "--name-only", "--no-renames", "-z", r.base, ref)
	if err != nil {
		fail("could not list its changes: " + err.Error())
		return
	}
	list := splitNUL(files)
	outside := cr.card.Outside(list)
	cr.set(func(c *cardRun) {
		c.ref, c.files, c.outside, c.state, c.ended = ref, list, outside, stDone, time.Now()
	})
	r.ev.log("box.state", map[string]any{"card": cr.card.ID, "box": b.ID, "state": stDone, "files": len(list), "outside": outside, "proof": cr.proof})
}

// earlyExam is closed when the user asks for the exam before every agent is
// done: exam.now from a client, or "e" in the terminal.
func (r *runner) earlyExam() <-chan struct{} {
	if r.s != nil {
		return r.s.earlyExam()
	}
	return r.examNow
}
