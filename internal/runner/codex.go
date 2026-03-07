package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// codexRunner implements Runner for OpenAI Codex CLI.
// Uses `codex -q "<prompt>"` for quiet/headless invocation.
type codexRunner struct {
	binaryPath string
}

func (r *codexRunner) Type() CLIType    { return CLICodex }
func (r *codexRunner) BinaryPath() string { return r.binaryPath }

func (r *codexRunner) Available() bool {
	_, err := exec.LookPath(r.binaryPath)
	return err == nil
}

// Invoke runs Codex CLI in quiet mode (read-only, no file changes).
// When req.OnProgress is set, uses exec --json mode for real-time progress.
func (r *codexRunner) Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	if req.OnProgress != nil {
		args := []string{"exec", req.Prompt, "--json"}
		return r.runStreaming(ctx, req, args)
	}
	args := []string{"-q", req.Prompt}
	return r.run(ctx, req, args)
}

// InvokeWithFiles runs Codex CLI with full-auto approval for file modifications.
// When req.OnProgress is set, uses exec --json mode for real-time progress.
func (r *codexRunner) InvokeWithFiles(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	if req.OnProgress != nil {
		args := []string{"exec", req.Prompt, "--json", "--approval-mode", "full-auto"}
		return r.runStreaming(ctx, req, args)
	}
	args := []string{
		"-q", req.Prompt,
		"--approval-mode", "full-auto",
	}
	return r.run(ctx, req, args)
}

func (r *codexRunner) run(ctx context.Context, req InvokeRequest, args []string) (*InvokeResult, error) {
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
		CLIType:   CLICodex,
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("codex invocation timed out after %v", timeout)
		}
		return result, fmt.Errorf("codex invocation failed (exit %d): %w\nstderr: %s",
			result.ExitCode, err, truncate(result.Stderr, 500))
	}

	return result, nil
}
