package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// geminiRunner implements Runner for Gemini CLI.
// Uses `gemini -p "<prompt>"` for headless invocation.
type geminiRunner struct {
	binaryPath string
}

func (r *geminiRunner) Type() CLIType    { return CLIGemini }
func (r *geminiRunner) BinaryPath() string { return r.binaryPath }

func (r *geminiRunner) Available() bool {
	_, err := exec.LookPath(r.binaryPath)
	return err == nil
}

// Invoke runs Gemini CLI in print mode (read-only, no file changes).
// When req.OnProgress is set, emits periodic heartbeat events (Gemini has no streaming mode).
func (r *geminiRunner) Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	args := []string{"-p", req.Prompt}
	if req.OnProgress != nil {
		return r.runWithHeartbeat(ctx, req, args)
	}
	return r.run(ctx, req, args)
}

// InvokeWithFiles runs Gemini CLI with tool access for file modifications.
// Note: Gemini CLI's -p flag runs in prompt mode which may limit tool access.
// This is functionally equivalent to Invoke until Gemini CLI exposes a
// headless mode with explicit file modification permissions.
// When req.OnProgress is set, emits periodic heartbeat events.
func (r *geminiRunner) InvokeWithFiles(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	args := []string{"-p", req.Prompt}
	if req.OnProgress != nil {
		return r.runWithHeartbeat(ctx, req, args)
	}
	return r.run(ctx, req, args)
}

// runWithHeartbeat wraps run() with a goroutine that emits ProgressHeartbeat events
// every 15 seconds, since Gemini CLI has no streaming output mode.
func (r *geminiRunner) runWithHeartbeat(ctx context.Context, req InvokeRequest, args []string) (*InvokeResult, error) {
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

func (r *geminiRunner) run(ctx context.Context, req InvokeRequest, args []string) (*InvokeResult, error) {
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
		CLIType:   CLIGemini,
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("gemini invocation timed out after %v", timeout)
		}
		return result, fmt.Errorf("gemini invocation failed (exit %d): %w\nstderr: %s",
			result.ExitCode, err, truncate(result.Stderr, 500))
	}

	return result, nil
}
