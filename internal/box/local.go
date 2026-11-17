package box

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Local runs every "box" as a folder on this machine. It is for testing the
// engine only: there is no isolation, and the agent runs with your own login.
type Local struct {
	Root     string // folder that holds the boxes
	AgentCmd string // shell command; the prompt arrives on stdin, the working directory is the repo
}

func (l *Local) Name() string    { return "local" }
func (l *Local) Sandboxed() bool { return false }

func (l *Local) Create(ctx context.Context, name string) (*Box, error) {
	if err := os.MkdirAll(l.Root, 0o755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(l.Root, slug(name)+"-")
	if err != nil {
		return nil, err
	}
	return &Box{ID: filepath.Base(dir), Name: name, RepoDir: filepath.Join(dir, "repo"), dir: dir}, nil
}

func (l *Local) boxDir(b *Box) string {
	if b.dir != "" {
		return b.dir
	}
	return filepath.Join(l.Root, b.ID)
}

func (l *Local) Exec(ctx context.Context, b *Box, script string, stdin io.Reader, stdout io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Dir = l.boxDir(b)
	cmd.Env = append(os.Environ(), "TW_REPO="+b.RepoDir)
	cmd.Stdin = stdin
	if stdout == nil {
		stdout = io.Discard
	}
	var stderr bytes.Buffer
	cmd.Stdout = stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil
	}
	if err != nil {
		return -1, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return 0, nil
}

func (l *Local) StartAgent(ctx context.Context, b *Box, prompt, logPath string) (Agent, error) {
	logf, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("bash", "-c", l.AgentCmd)
	cmd.Dir = b.RepoDir
	cmd.Env = append(os.Environ(), "TW_REPO="+b.RepoDir)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = logf
	cmd.Stderr = logf
	if err := cmd.Start(); err != nil {
		logf.Close()
		return nil, err
	}
	a := &localAgent{cmd: cmd, logPath: logPath, done: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		logf.Close()
		a.mu.Lock()
		a.exitErr = err
		a.mu.Unlock()
		close(a.done)
	}()
	return a, nil
}

type localAgent struct {
	cmd     *exec.Cmd
	logPath string
	done    chan struct{}
	mu      sync.Mutex
	exitErr error
}

func (a *localAgent) Status() AgentStatus {
	data, _ := os.ReadFile(a.logPath)
	text := strings.TrimSpace(string(data))
	last := lastLine(text)
	select {
	case <-a.done:
		a.mu.Lock()
		defer a.mu.Unlock()
		st := AgentStatus{Final: text, Thought: last}
		if a.exitErr != nil {
			st.Failed = true
			st.Err = a.exitErr.Error()
		}
		return st
	default:
		return AgentStatus{Running: true, Thought: last}
	}
}

func (a *localAgent) Stop() {
	if a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
	}
	select {
	case <-a.done:
	case <-time.After(5 * time.Second):
	}
}

func (l *Local) Destroy(ctx context.Context, b *Box) error {
	return os.RemoveAll(l.boxDir(b))
}

func (l *Local) List(ctx context.Context, prefix string) ([]Box, error) {
	entries, err := os.ReadDir(l.Root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Box
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), slug(prefix)) {
			out = append(out, Box{ID: e.Name(), Name: e.Name()})
		}
	}
	return out, nil
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	return s
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteByte('-')
			}
		}
		if b.Len() >= 40 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}
