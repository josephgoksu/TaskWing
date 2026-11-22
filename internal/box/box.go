// Package box runs work in sandboxes. A Provider creates boxes, runs shell
// scripts in them, starts one coding agent per box and destroys them.
package box

import (
	"context"
	"io"
	"strings"
	"unicode/utf8"
)

// Defaults for the agent in a boat box. The engine's flags show the same values.
const (
	DefaultModel  = "opus-5-5"
	DefaultEffort = "medium"
)

// Box is one sandbox.
type Box struct {
	ID      string
	Name    string
	RepoDir string // where the repository lives inside the box, as the box's shell sees it
	dir     string // local provider only
}

// AgentStatus is what the engine knows about a running agent.
type AgentStatus struct {
	Running bool
	Failed  bool
	Thought string // the agent's last action, short
	Final   string // the agent's final answer text
	Err     string
}

// Agent is one coding agent working in a box.
type Agent interface {
	Status() AgentStatus
	Stop()
}

// Provider is a sandbox provider.
type Provider interface {
	Name() string
	// Sandboxed is false when the provider gives no isolation (local test mode).
	Sandboxed() bool
	Create(ctx context.Context, name string) (*Box, error)
	// Exec runs a bash script in the box. stdin may be nil. It returns the script's exit code.
	Exec(ctx context.Context, b *Box, script string, stdin io.Reader, stdout io.Writer) (int, error)
	StartAgent(ctx context.Context, b *Box, prompt, logPath string) (Agent, error)
	Destroy(ctx context.Context, b *Box) error
	// List returns boxes whose name starts with prefix.
	List(ctx context.Context, prefix string) ([]Box, error)
}

// firstLine returns the first line of s, at most 200 bytes, cut on a rune
// boundary so an error message never ends in a broken UTF-8 sequence.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if len(s) > 200 {
		i := 200
		for i > 0 && !utf8.RuneStart(s[i]) {
			i--
		}
		s = s[:i]
	}
	return s
}

// detail formats the first line of s as ": line", or "" when there is none,
// so an error never ends in a dangling ": ".
func detail(s string) string {
	if l := firstLine(s); l != "" {
		return ": " + l
	}
	return ""
}
