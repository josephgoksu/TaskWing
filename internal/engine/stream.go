package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The engine stream: `taskwing engine --json`. A client sends commands
// as JSON lines on stdin and reads events as JSON lines on stdout. The full
// protocol is in docs/engine-protocol.md.

// ProtocolVersion is sent in engine.hello.
const ProtocolVersion = 1

// Can lists the optional commands this engine handles. A client should not send what is
// not listed (pausing agents is not built yet).
var Can = []string{"plan.approve", "plan.redo", "exam.now"}

// Command is one line from the client.
type Command struct {
	Type       string      `json:"type"`
	RunID      string      `json:"run_id"`
	Repo       string      `json:"repo"`
	Task       string      `json:"task"`
	Sandbox    string      `json:"sandbox"`
	Models     string      `json:"models"`
	QuestionID string      `json:"question_id"`
	Choice     string      `json:"choice"`
	Checks     []WireCheck `json:"checks"`
	NoPR       bool        `json:"no_pr"` // stop after the exam: push nothing
}

// WireCheck is an exam check as the client sees it.
type WireCheck struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	AddedBy string `json:"added_by"`
}

// StartFunc turns a run.start command into run options. It resolves the
// repository and the sandbox; Serve fills in the stream, the run id and the
// output.
type StartFunc func(ctx context.Context, c Command) (Options, error)

// maxCommandLine is the longest command line Serve reads. Longer lines are
// skipped, like any other line that is not a command.
const maxCommandLine = 1 << 20

// Serve reads commands from in and writes events to out until in closes or
// ctx ends. It then stops every run, waits for their sandboxes to be cleaned
// up, and returns.
func Serve(ctx context.Context, in io.Reader, out io.Writer, version string, start StartFunc) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := &server{out: out, start: start, runs: map[string]*Stream{}}
	s.emit(map[string]any{"type": "engine.hello", "version": version, "protocol": ProtocolVersion, "can": Can})

	// Read on a goroutine of its own, so Ctrl-C stops the engine even while
	// stdin stays open (run by hand).
	cmds := make(chan Command)
	readErr := make(chan error, 1)
	go func() {
		defer close(cmds)
		readErr <- readLines(in, maxCommandLine, func(line []byte) bool {
			var c Command
			if err := json.Unmarshal(line, &c); err != nil || c.Type == "" {
				return true
			}
			select {
			case cmds <- c:
				return true
			case <-ctx.Done():
				return false
			}
		})
	}()
	var err error
loop:
	for {
		select {
		case c, ok := <-cmds:
			if !ok {
				err = <-readErr
				break loop
			}
			s.handle(ctx, c)
		case <-ctx.Done():
			break loop
		}
	}
	// stdin closed (the client is quitting) or a signal: stop the agents and clean up.
	cancel()
	s.wg.Wait()
	return err
}

// readLines calls f with each line of r, without its line ending, until r
// ends or f returns false. A line longer than max is skipped, not fatal.
func readLines(r io.Reader, max int, f func(line []byte) bool) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var line []byte
	over := false
	for {
		chunk, more, err := br.ReadLine()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !over && len(line)+len(chunk) <= max {
			line = append(line, chunk...)
		} else {
			over = true
		}
		if more {
			continue
		}
		if !over && !f(line) {
			return nil
		}
		line, over = line[:0], false
	}
}

type server struct {
	mu    sync.Mutex
	out   io.Writer
	start StartFunc
	runs  map[string]*Stream
	wg    sync.WaitGroup
}

func (s *server) emit(ev map[string]any) {
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.out.Write(append(b, '\n'))
}

func (s *server) handle(ctx context.Context, c Command) {
	if c.Type == "run.start" {
		s.startRun(ctx, c)
		return
	}
	s.mu.Lock()
	st := s.runs[c.RunID]
	s.mu.Unlock()
	if st != nil {
		st.deliver(c)
	}
}

func (s *server) startRun(ctx context.Context, c Command) {
	s.mu.Lock()
	id := NewRunID()
	// Two runs started in the same second may draw the same random part;
	// a second stream under one id would take over the first one's commands.
	for s.runs[id] != nil {
		id = NewRunID()
	}
	st := newStream(id, s.emit)
	s.runs[id] = st
	s.mu.Unlock()

	task := strings.TrimSpace(c.Task)
	repo := strings.TrimSpace(c.Repo)
	s.emit(map[string]any{"type": "run.created", "run": map[string]any{
		"id": id, "repo": repo, "task": task, "created_at": time.Now().UTC().Format(time.RFC3339),
	}})
	st.stage("planning")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.runs, id)
			s.mu.Unlock()
		}()
		res, err := s.run(ctx, st, c)
		// A client that quits (stdin closed) ends ctx: whatever error that
		// caused on the way, the run was stopped.
		if err != nil && ctx.Err() != nil && !errors.Is(err, ctx.Err()) {
			err = fmt.Errorf("%w (%v)", ctx.Err(), err)
		}
		st.finish(res, err)
	}()
}

func (s *server) run(ctx context.Context, st *Stream, c Command) (*Result, error) {
	o, err := s.start(ctx, c)
	if err != nil {
		return nil, err
	}
	o.Stream = st
	o.RunID = st.id
	o.Hunt = false
	o.Yes = false
	o.NoPR = o.NoPR || c.NoPR
	if o.Out == nil {
		o.Out = io.Discard
	}
	o.In = strings.NewReader("")
	defer ReleaseRepo(o.Dir)
	return Run(ctx, o)
}

// Stream connects one run to the client.
type Stream struct {
	id   string
	emit func(map[string]any)

	mu       sync.Mutex
	cur      string            // the stage the client was last told
	approval chan Command      // plan.approve or plan.redo while the plan waits
	sent     map[string]string // the last card.update sent for each card
	examNow  chan struct{}
	examOnce sync.Once
}

func newStream(id string, emit func(map[string]any)) *Stream {
	return &Stream{id: id, emit: emit, sent: map[string]string{}, examNow: make(chan struct{})}
}

func (s *Stream) send(typ string, fields map[string]any) {
	if s == nil {
		return
	}
	ev := map[string]any{"type": typ, "run_id": s.id}
	for k, v := range fields {
		ev[k] = v
	}
	s.emit(ev)
}

func (s *Stream) stage(stage string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.cur = stage
	s.mu.Unlock()
	s.send("run.stage", map[string]any{"stage": stage})
}

func (s *Stream) deliver(c Command) {
	switch c.Type {
	case "plan.approve", "plan.redo":
		s.mu.Lock()
		ch := s.approval
		s.approval = nil
		s.mu.Unlock()
		if ch != nil {
			ch <- c
		}
	case "exam.now":
		s.mu.Lock()
		building := s.cur == "build"
		s.mu.Unlock()
		if building {
			s.examOnce.Do(func() { close(s.examNow) })
		}
	}
}

// earlyExam is closed when the user asks for the exam before every agent is done.
func (s *Stream) earlyExam() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.examNow
}

// askApproval shows the client the plan and blocks until the user approves it or
// asks for a new one. It listens for the answer before the client is told, so an
// answer that comes at once is never dropped.
func (s *Stream) askApproval(ctx context.Context, p *Plan, checks []Check) (Command, error) {
	ch := make(chan Command, 1)
	s.mu.Lock()
	s.approval = ch
	s.mu.Unlock()
	s.planReady(p, checks)
	s.stage("approve")
	select {
	case c := <-ch:
		return c, nil
	case <-ctx.Done():
		s.mu.Lock()
		s.approval = nil
		s.mu.Unlock()
		return Command{}, ctx.Err()
	}
}

func (s *Stream) planReady(p *Plan, checks []Check) {
	cards := make([]map[string]any, len(p.Cards))
	for i, c := range p.Cards {
		cards[i] = map[string]any{"id": c.ID, "title": c.Title, "owns": c.Owns, "proof": map[string]any{"kind": "test"}}
	}
	wc := make([]map[string]any, len(checks))
	for i, c := range checks {
		wc[i] = map[string]any{"id": c.ID, "text": c.Text, "added_by": "exam"}
	}
	s.send("plan.ready", map[string]any{"cards": cards, "names": interfaceNames(p.Interface), "checks": wc})
}

// interfaceNames turns the lead's pinned interface into short names for the client.
func interfaceNames(iface string) []string {
	names := []string{}
	for _, line := range strings.Split(iface, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*• "))
		if line == "" {
			continue
		}
		names = append(names, truncate(line, 80))
		if len(names) == 20 {
			break
		}
	}
	return names
}

// card sends one agent's square to the client. Engine states map onto the client's:
// a finished agent "passed" (its own tests), one stopped early for the exam is
// "done", a failed one is "stuck".
func (s *Stream) card(cr *cardRun) {
	if s == nil || cr.card.ID == "" {
		return
	}
	cr.mu.Lock()
	state, detail := cr.state, cr.thought
	var proof map[string]any
	switch cr.state {
	case stDone:
		state, detail = "passed", "Its own work is in"
		if cr.early {
			state, detail = "done", "Stopped for the exam"
		}
		if cr.proof != "" {
			proof = map[string]any{"kind": "test", "text": truncate(cr.proof, 60)}
		}
	case stStuck:
		detail = cr.err
	}
	cr.mu.Unlock()
	card := map[string]any{"id": cr.card.ID, "state": state, "detail": truncate(detail, 200)}
	if proof != nil {
		card["proof"] = proof
	}
	s.sendCard(card)
}

// leftOut tells the client that a finished card's work conflicted with the
// others in the join and is not in the joined work.
func (s *Stream) leftOut(cr *cardRun) {
	if s == nil || cr.card.ID == "" {
		return
	}
	s.sendCard(map[string]any{"id": cr.card.ID, "state": stStuck, "detail": "Conflicted with the other pieces; left out of the joined work"})
}

// sendCard sends a card.update unless the client already has exactly this one
// (the board polls every card each second).
func (s *Stream) sendCard(card map[string]any) {
	b, _ := json.Marshal(card)
	id, _ := card["id"].(string)
	s.mu.Lock()
	same := s.sent[id] == string(b)
	s.sent[id] = string(b)
	s.mu.Unlock()
	if !same {
		s.send("card.update", map[string]any{"card": card})
	}
}

func (s *Stream) examResult(checks []Check) {
	out := make([]map[string]any, len(checks))
	for i, c := range checks {
		r := "fail"
		if c.Passed {
			r = "pass"
		}
		out[i] = map[string]any{"id": c.ID, "result": r}
	}
	s.send("exam.result", map[string]any{"checks": out})
}

var prNumberRe = regexp.MustCompile(`/pull/(\d+)`)

// prOpened tells the client the pull request's link. gh printed something
// else when the URL is not https or has no number; the client is not handed
// a link it cannot open (the run log has gh's output).
func (s *Stream) prOpened(url string) {
	n := 0
	if m := prNumberRe.FindStringSubmatch(url); m != nil {
		n, _ = strconv.Atoi(m[1])
	}
	if !strings.HasPrefix(url, "https://") || n == 0 {
		return
	}
	s.send("pr.opened", map[string]any{"draft": true, "url": url, "number": n})
}

// finish sends the end of a run: run.finished, plus run.error when the run
// stopped on an error instead of an exam result.
func (s *Stream) finish(res *Result, err error) {
	ended := time.Now().UTC().Format(time.RFC3339)
	secrets := 0
	if res != nil {
		secrets = len(res.Secrets)
	}
	if err != nil {
		msg := err.Error()
		if errors.Is(err, context.Canceled) {
			msg = "stopped; every sandbox was cleaned up"
		}
		if res != nil && res.RunDir != "" {
			msg += " (run log: " + res.RunDir + ")"
		}
		s.send("run.error", map[string]any{"message": truncate(msg, 400)})
		s.send("run.finished", map[string]any{"ok": false, "secrets_found": secrets, "ended_at": ended})
		return
	}
	ok := res != nil && !res.Stopped && res.Total > 0 && res.Passed == res.Total
	if ok {
		s.stage("done")
	}
	s.send("run.finished", map[string]any{"ok": ok, "secrets_found": secrets, "ended_at": ended})
}
