// Package runner provides an abstraction for spawning AI CLI tools (Claude Code,
// Gemini CLI, Codex CLI) as headless worker subprocesses. This inverts the control
// flow: instead of requiring users to configure API keys for direct LLM calls,
// TaskWing orchestrates the user's already-installed AI CLI.
package runner

import (
	"context"
	"fmt"
	"time"
)

// CLIType identifies a supported AI CLI tool.
type CLIType string

const (
	CLIClaude CLIType = "claude"
	CLIGemini CLIType = "gemini"
	CLICodex  CLIType = "codex"
)

// String returns the human-readable name for the CLI type.
func (c CLIType) String() string {
	switch c {
	case CLIClaude:
		return "Claude Code"
	case CLIGemini:
		return "Gemini CLI"
	case CLICodex:
		return "Codex CLI"
	default:
		return string(c)
	}
}

// DefaultTimeout is the default timeout for AI CLI invocations.
const DefaultTimeout = 10 * time.Minute

// ProgressEventType classifies streaming progress events from AI CLIs.
type ProgressEventType int

const (
	ProgressThinking   ProgressEventType = iota // AI is reasoning
	ProgressText                                // AI produced output text
	ProgressToolUse                             // AI is using a tool (Read, Grep, etc.)
	ProgressToolResult                          // Tool returned a result
	ProgressHeartbeat                           // Periodic "still alive" signal
)

// ProgressEvent is a single progress update from a streaming AI CLI invocation.
type ProgressEvent struct {
	Type    ProgressEventType
	Summary string // Human-readable short summary (e.g., "Reading src/main.go", "Thinking...")
}

// ProgressCallback receives streaming progress events during invocation.
type ProgressCallback func(ProgressEvent)

// InvokeRequest configures a single AI CLI invocation.
type InvokeRequest struct {
	Prompt       string           // The prompt to send to the AI CLI
	SystemPrompt string           // Optional system prompt (not all CLIs support this)
	WorkDir      string           // Working directory for the subprocess
	Timeout      time.Duration    // Max time for the invocation (0 = DefaultTimeout)
	OnProgress   ProgressCallback // nil = buffered mode (no streaming)
	Model        string           // Model override (e.g., "sonnet", "opus"). Empty = CLI default.
}

// effectiveTimeout returns the timeout to use, falling back to DefaultTimeout.
func (r *InvokeRequest) effectiveTimeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return DefaultTimeout
}

// Runner is the interface for spawning an AI CLI as a headless subprocess.
type Runner interface {
	// Type returns the CLI type this runner wraps.
	Type() CLIType

	// Available returns true if the CLI binary is found on PATH.
	Available() bool

	// BinaryPath returns the resolved path to the CLI binary.
	BinaryPath() string

	// Invoke runs the AI CLI in read-only mode (structured JSON output, no file changes).
	// Use this for analysis and planning operations.
	Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error)

	// InvokeWithFiles runs the AI CLI with full tool access (can modify files).
	// Use this for task execution where the AI CLI implements changes directly.
	InvokeWithFiles(ctx context.Context, req InvokeRequest) (*InvokeResult, error)
}

// DetectedCLI represents a CLI tool found on the system.
type DetectedCLI struct {
	Type       CLIType
	BinaryPath string
}

// NewRunner creates a Runner for the given CLI type and binary path.
func NewRunner(cli DetectedCLI) Runner {
	switch cli.Type {
	case CLIClaude:
		return &claudeRunner{binaryPath: cli.BinaryPath}
	case CLIGemini:
		return &geminiRunner{binaryPath: cli.BinaryPath}
	case CLICodex:
		return &codexRunner{binaryPath: cli.BinaryPath}
	default:
		return nil
	}
}

// DetectAndCreateRunners finds all installed AI CLIs and creates runners for them.
func DetectAndCreateRunners() ([]Runner, error) {
	detected := DetectCLIs()
	if len(detected) == 0 {
		return nil, fmt.Errorf("no AI CLI found. Install Claude Code, Gemini CLI, or Codex CLI")
	}

	runners := make([]Runner, 0, len(detected))
	for _, cli := range detected {
		if r := NewRunner(cli); r != nil {
			runners = append(runners, r)
		}
	}
	if len(runners) == 0 {
		return nil, fmt.Errorf("no AI CLI found. Install Claude Code, Gemini CLI, or Codex CLI")
	}
	return runners, nil
}

// PreferredRunner returns the best available runner, optionally preferring a specific CLI type.
// Priority: preferred (if set and available) > Claude > Gemini > Codex.
func PreferredRunner(preferred CLIType) (Runner, error) {
	runners, err := DetectAndCreateRunners()
	if err != nil {
		return nil, err
	}

	// If a preference is set, look for it first
	if preferred != "" {
		for _, r := range runners {
			if r.Type() == preferred {
				return r, nil
			}
		}
	}

	// Return the first available (detection order is Claude > Gemini > Codex)
	return runners[0], nil
}
