// Package box runs work in sandboxes. A Provider creates boxes, runs shell
// scripts in them, starts one coding agent per box and destroys them.
package box

import (
	"context"
	"io"
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
