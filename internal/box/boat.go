package box

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Boat runs boxes on boat.dev through its CLI. Each box is forked from a base
// box that already has the repository's toolchain and a Claude Code login.
// The agent runs through `boat prompt`, so model usage is billed to the
// account connected to boat. GitHub credentials never enter a box.
type Boat struct {
	Bin         string // path to the boat CLI, default "boat"
	Base        string // box id to fork from
	Environment string // optional boat environment to pin forks to
	Model       string // e.g. opus-5-5
	Effort      string // low|medium|high|xhigh|max
	TTL         int    // box lifetime in seconds
	Poll        time.Duration
}

func (p *Boat) Name() string    { return "boat" }
func (p *Boat) Sandboxed() bool { return true }

func (p *Boat) bin() string {
	if p.Bin != "" {
		return p.Bin
	}
	return "boat"
}

func (p *Boat) run(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, p.bin(), args...)
	cmd.Stdin = stdin
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return out.Bytes(), fmt.Errorf("boat %s: %v: %s", args[0], err, firstLine(errb.String()+" "+out.String()))
	}
	return out.Bytes(), nil
}

func (p *Boat) Create(ctx context.Context, name string) (*Box, error) {
	if p.Base == "" {
		return nil, errors.New("no base box: set TASKWING_BOAT_BASE to a prepared boat box id (see docs/poc.md)")
	}
	ttl := p.TTL
	if ttl == 0 {
		ttl = 7200
	}
	args := []string{"fork", p.Base, "--ttl", strconv.Itoa(ttl), "--wait", "--json"}
	if p.Environment != "" {
		args = append(args, "--environment", p.Environment)
	}
	out, err := p.run(ctx, nil, args...)
	// --json prints one event per line; the id is in the last line that has one.
	var id string
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var ev struct {
			ID    string `json:"id"`
			Event string `json:"event"`
			Error string `json:"error"`
		}
		if dec.Decode(&ev) != nil {
			break
		}
		if ev.Event == "error" && err == nil {
			err = fmt.Errorf("boat fork: %s", ev.Error)
		}
		if ev.ID != "" {
			id = ev.ID
		}
	}
	if err != nil {
		// A fork that failed or was cancelled after the box existed would leak
		// a box without the taskwing name, which doctor cannot find.
		if id != "" {
			dctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			_, _ = p.run(dctx, nil, "delete", "--yes", id)
			cancel()
		}
		return nil, err
	}
	if id == "" {
		return nil, fmt.Errorf("boat fork: no box id in output: %s", firstLine(string(out)))
	}
	_, _ = p.run(ctx, nil, "rename", id, name)
	return &Box{ID: id, Name: name, RepoDir: "$HOME/tw/repo"}, nil
}

var rcLine = regexp.MustCompile(`__TW_RC=(\d+)`)

// Exec sends the script as base64 in the ssh command, so stdin stays free for data.
func (p *Boat) Exec(ctx context.Context, b *Box, script string, stdin io.Reader, stdout io.Writer) (int, error) {
	enc := base64.StdEncoding.EncodeToString([]byte("export TW_REPO=\"" + b.RepoDir + "\"\n" + script))
	remote := "f=$(mktemp); echo " + enc + " | base64 -d > $f; bash $f; rc=$?; rm -f $f; echo __TW_RC=$rc >&2; exit $rc"
	cmd := exec.CommandContext(ctx, p.bin(), "ssh", b.ID, "--", remote)
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	cmd.Stdin = stdin
	if stdout == nil {
		stdout = io.Discard
	}
	var errb bytes.Buffer
	cmd.Stdout = stdout
	cmd.Stderr = &errb
	err := cmd.Run()
	// The wrapper prints its marker last; anything earlier (the script's own
	// stderr) must not be able to set the exit code.
	if m := rcLine.FindAllStringSubmatch(errb.String(), -1); len(m) > 0 {
		rc, _ := strconv.Atoi(m[len(m)-1][1])
		return rc, nil
	}
	if err != nil {
		return -1, fmt.Errorf("boat ssh %s: %v: %s", b.ID, err, firstLine(errb.String()))
	}
	return -1, fmt.Errorf("boat ssh %s: no exit code from the box: %s", b.ID, firstLine(errb.String()))
}

func (p *Boat) StartAgent(ctx context.Context, b *Box, prompt, logPath string) (Agent, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("empty prompt")
	}
	model, effort := p.Model, p.Effort
	if model == "" {
		model = "opus-5-5"
	}
	if effort == "" {
		effort = "medium"
	}
	logf, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(p.bin(), "prompt", b.ID, "--new", "--provider", "claude", "--model", model, "--reasoning-effort", effort, prompt)
	pr, pw := io.Pipe()
	cmd.Stdout = io.MultiWriter(logf, pw)
	cmd.Stderr = logf
	if err := cmd.Start(); err != nil {
		logf.Close()
		return nil, err
	}
	poll := p.Poll
	if poll == 0 {
		poll = 15 * time.Second
	}
	a := &boatAgent{p: p, box: b, cmd: cmd, started: time.Now(), done: make(chan struct{})}
	go func() { _ = cmd.Wait(); pw.Close(); logf.Close(); a.setExited() }()
	go a.read(pr)
	go a.watch(poll)
	return a, nil
}

type boatAgent struct {
	p       *Boat
	box     *Box
	cmd     *exec.Cmd
	started time.Time
	done    chan struct{}
	once    sync.Once

	mu       sync.Mutex
	convID   string
	response bool
	errText  string
	thought  string
	final    string
	finished bool
	exited   bool
	// sawFinished: the stream sent a "finished" status; confirm with boat conversations.
	sawFinished bool
	limited     bool // a usage_limit event: login missing or plan spent
}

func (a *boatAgent) setExited() {
	a.mu.Lock()
	a.exited = true
	a.mu.Unlock()
}

// read parses boat's JSONL stream: the first event carries the conversation id,
// chat events carry tool calls and text, an error event means the prompt failed.
func (a *boatAgent) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var ev struct {
			Event  string `json:"event"`
			Status string `json:"status"`
			Data   struct {
				Status         string `json:"status"`
				ConversationID string `json:"conversationId"`
				Type           string `json:"type"`
				Message        string `json:"message"`
				Data           struct {
					Content string `json:"content"`
					Tools   []struct {
						Use struct {
							Name  string                     `json:"name"`
							Input map[string]json.RawMessage `json:"input"`
						} `json:"use"`
					} `json:"tools"`
				} `json:"data"`
			} `json:"data"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		a.mu.Lock()
		if a.convID == "" && ev.Data.ConversationID != "" {
			a.convID = ev.Data.ConversationID
		}
		if ev.Event == "error" {
			a.errText = ev.Data.Message
			if a.errText == "" {
				a.errText = firstLine(sc.Text())
			}
		}
		if ev.Data.Type == "usage_limit" {
			// e.g. "Claude login required" or a spent plan: the agent never ran.
			a.limited = true
			a.errText = "boat: " + firstLine(sc.Text())
			var u struct {
				Data struct {
					Data struct {
						Summary string `json:"summary"`
					} `json:"data"`
				} `json:"data"`
			}
			if json.Unmarshal(sc.Bytes(), &u) == nil && u.Data.Data.Summary != "" {
				a.errText = "boat: " + u.Data.Data.Summary
			}
		}
		if ev.Status == "finished" || ev.Data.Status == "finished" {
			a.sawFinished = true
		}
		if ev.Data.Type == "response" {
			a.response = true
			if c := strings.TrimSpace(ev.Data.Data.Content); c != "" {
				a.final = c
				a.thought = lastLine(c)
			}
			for _, t := range ev.Data.Data.Tools {
				a.thought = describeTool(t.Use.Name, t.Use.Input)
			}
		}
		a.mu.Unlock()
	}
	// After a scan error (an oversized line) keep draining, or boat prompt
	// blocks on its stdout and cmd.Wait never returns.
	_, _ = io.Copy(io.Discard, r)
}

// watch asks boat whether the conversation still runs. boat prompt never exits
// on its own: done = a response arrived and the conversation stopped.
func (a *boatAgent) watch(every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-t.C:
		}
		a.mu.Lock()
		conv, resp, errText, exited := a.convID, a.response, a.errText, a.exited
		a.mu.Unlock()
		if errText != "" && !resp {
			a.finish()
			return
		}
		if exited && !resp {
			a.mu.Lock()
			if a.errText == "" {
				a.errText = "boat prompt exited before the agent answered"
			}
			a.mu.Unlock()
			a.finish()
			return
		}
		a.mu.Lock()
		sawFinished := a.sawFinished
		a.mu.Unlock()
		if sawFinished {
			a.mu.Lock()
			if !resp && a.errText == "" {
				a.errText = "the agent stopped without answering"
			}
			a.mu.Unlock()
			a.finish()
			return
		}
		if conv == "" || !resp {
			continue
		}
		running, ok := a.conversationRunning(conv)
		if ok && !running {
			a.finish()
			return
		}
	}
}

func (a *boatAgent) conversationRunning(conv string) (running, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := a.p.run(ctx, nil, "conversations", a.box.ID, "--json")
	if err != nil {
		return false, false
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var c struct {
			Data struct {
				ID      string `json:"id"`
				Running bool   `json:"running"`
			} `json:"data"`
		}
		if dec.Decode(&c) != nil {
			return false, false
		}
		if c.Data.ID == conv {
			return c.Data.Running, true
		}
	}
}

func (a *boatAgent) finish() {
	a.once.Do(func() {
		a.mu.Lock()
		a.finished = true
		a.mu.Unlock()
		if a.cmd.Process != nil {
			_ = a.cmd.Process.Kill()
		}
		close(a.done)
	})
}

func (a *boatAgent) Status() AgentStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.finished {
		return AgentStatus{Running: true, Thought: a.thought}
	}
	st := AgentStatus{Thought: a.thought, Final: a.final}
	if a.limited || (a.errText != "" && !a.response) {
		st.Failed, st.Err = true, a.errText
	}
	return st
}

func (a *boatAgent) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = a.p.run(ctx, nil, "interrupt", a.box.ID)
	a.finish()
}

func (p *Boat) Destroy(ctx context.Context, b *Box) error {
	_, err := p.run(ctx, nil, "delete", "--yes", b.ID)
	return err
}

func (p *Boat) List(ctx context.Context, prefix string) ([]Box, error) {
	out, err := p.run(ctx, nil, "list", "--all", "--json")
	if err != nil {
		return nil, err
	}
	var l struct {
		Sandboxes []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"sandboxes"`
	}
	if err := json.Unmarshal(out, &l); err != nil {
		return nil, fmt.Errorf("boat list: %v", err)
	}
	var res []Box
	for _, s := range l.Sandboxes {
		if strings.HasPrefix(s.Name, prefix) {
			res = append(res, Box{ID: s.ID, Name: s.Name + " (" + s.State + ")"})
		}
	}
	return res, nil
}

func describeTool(name string, in map[string]json.RawMessage) string {
	for _, k := range []string{"command", "file_path", "path", "pattern", "description"} {
		if raw, ok := in[k]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil && s != "" {
				return name + ": " + firstLine(s)
			}
		}
	}
	return name
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
