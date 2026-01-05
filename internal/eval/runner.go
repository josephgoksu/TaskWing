package eval

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner invokes TaskWing CLI commands as subprocesses.
// This ensures eval tests the actual CLI behavior, not internal reimplementations.
type Runner struct {
	// Binary is the path to the tw binary. Default: "tw" (uses PATH lookup).
	Binary string

	// WorkDir is the working directory for command execution.
	WorkDir string

	// Timeout is the per-command timeout. Default: 10 minutes.
	Timeout time.Duration

	// Env contains additional environment variables to set for commands.
	// These are appended to os.Environ().
	Env []string

	// Quiet suppresses progress output during execution.
	Quiet bool
}

// Result holds the output from a CLI command execution.
type Result struct {
	// Stdout contains the standard output from the command.
	Stdout string

	// Stderr contains the standard error output from the command.
	Stderr string

	// ExitCode is the process exit code (0 = success).
	ExitCode int

	// Duration is how long the command took to execute.
	Duration time.Duration

	// Command is the full command that was executed (for debugging).
	Command string
}

// NewRunner creates a Runner with sensible defaults.
func NewRunner(workDir string) *Runner {
	return &Runner{
		Binary:  "tw",
		WorkDir: workDir,
		Timeout: 10 * time.Minute,
		Env:     nil,
		Quiet:   false,
	}
}

// WithTimeout sets the per-command timeout.
func (r *Runner) WithTimeout(timeout time.Duration) *Runner {
	r.Timeout = timeout
	return r
}

// WithEnv adds environment variables for command execution.
func (r *Runner) WithEnv(env ...string) *Runner {
	r.Env = append(r.Env, env...)
	return r
}

// Execute runs a tw command with the given arguments.
// Example: runner.Execute(ctx, "bootstrap", "--json")
func (r *Runner) Execute(ctx context.Context, args ...string) (Result, error) {
	return r.ExecuteWithInput(ctx, "", args...)
}

// ExecuteWithInput runs a tw command with stdin input.
// Useful for commands that require interactive input.
func (r *Runner) ExecuteWithInput(ctx context.Context, stdin string, args ...string) (Result, error) {
	start := time.Now()

	// Apply timeout
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, r.Binary, args...)
	cmd.Dir = r.WorkDir

	// Set environment
	cmd.Env = os.Environ()
	if len(r.Env) > 0 {
		cmd.Env = append(cmd.Env, r.Env...)
	}

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Provide stdin if given
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	// Build command string for logging
	cmdStr := fmt.Sprintf("%s %s", r.Binary, strings.Join(args, " "))

	if !r.Quiet {
		fmt.Printf("  ▶ Running: %s\n", cmdStr)
	}

	// Run the command
	err := cmd.Run()
	duration := time.Since(start)

	result := Result{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: duration,
		Command:  cmdStr,
	}

	// Extract exit code
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		// Check for timeout
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("command timed out after %v: %s", r.Timeout, cmdStr)
		}
		return result, fmt.Errorf("command failed (exit %d): %w", result.ExitCode, err)
	}

	if !r.Quiet {
		fmt.Printf("  ✓ Completed in %.1fs\n", duration.Seconds())
	}

	return result, nil
}

// Combined returns stdout + stderr combined.
func (r Result) Combined() string {
	if r.Stderr == "" {
		return r.Stdout
	}
	if r.Stdout == "" {
		return r.Stderr
	}
	return r.Stdout + "\n" + r.Stderr
}
