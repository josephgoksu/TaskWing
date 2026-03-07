package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// claudeRunner implements Runner for Claude Code.
// Uses `claude -p "<prompt>" --output-format json` for headless invocation.
type claudeRunner struct {
	binaryPath string
}

func (r *claudeRunner) Type() CLIType    { return CLIClaude }
func (r *claudeRunner) BinaryPath() string { return r.binaryPath }

func (r *claudeRunner) Available() bool {
	_, err := exec.LookPath(r.binaryPath)
	return err == nil
}

// Invoke runs Claude Code in read-only print mode with JSON output.
// When req.OnProgress is set, emits periodic heartbeat events.
//
// Note: Claude Code's stream-json mode requires --verbose, which changes
// Claude's behavior (spawns subagents, more thorough analysis) and causes
// timeouts on complex prompts. We use buffered json mode with heartbeats
// until Claude Code supports stream-json without --verbose.
func (r *claudeRunner) Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	args := []string{
		"-p", req.Prompt,
		"--output-format", "json",
		"--no-session-persistence",
	}

	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	if req.OnProgress != nil {
		return r.runWithHeartbeat(ctx, req, args)
	}
	return r.run(ctx, req, args)
}

// InvokeWithFiles runs Claude Code with full tool access for file modifications.
// When req.OnProgress is set, emits periodic heartbeat events.
func (r *claudeRunner) InvokeWithFiles(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	args := []string{
		"-p", req.Prompt,
		"--output-format", "json",
		"--no-session-persistence",
		"--allowedTools", "Edit,Write,Bash,Read,Glob,Grep",
	}

	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	if req.OnProgress != nil {
		return r.runWithHeartbeat(ctx, req, args)
	}
	return r.run(ctx, req, args)
}

// runWithHeartbeat wraps run() with a goroutine that emits ProgressHeartbeat events
// every 15 seconds, since Claude Code's stream-json requires --verbose which
// changes behavior and causes timeouts.
func (r *claudeRunner) runWithHeartbeat(ctx context.Context, req InvokeRequest, args []string) (*InvokeResult, error) {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				req.OnProgress(ProgressEvent{
					Type:    ProgressHeartbeat,
					Summary: "still working...",
				})
			case <-done:
				return
			}
		}
	}()

	result, err := r.run(ctx, req, args)
	close(done)
	return result, err
}

func (r *claudeRunner) run(ctx context.Context, req InvokeRequest, args []string) (*InvokeResult, error) {
	timeout := req.effectiveTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.binaryPath, args...)
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &InvokeResult{
		RawOutput: stdout.String(),
		Stderr:    stderr.String(),
		CLIType:   CLIClaude,
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("claude invocation timed out after %v", timeout)
		}
		return result, fmt.Errorf("claude invocation failed (exit %d): %w\nstderr: %s",
			result.ExitCode, err, truncate(result.Stderr, 500))
	}

	return result, nil
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
