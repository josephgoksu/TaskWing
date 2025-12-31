package eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Executor runs evaluation tasks using the CLI Runner pattern.
// This replaces the internal reimplementation with subprocess invocation.
type Executor struct {
	Runner    *Runner
	OutDir    string
	Tasks     []Task
	Quiet     bool
	ResetRepo bool
}

// NewExecutor creates an Executor for running CLI-based eval tasks.
func NewExecutor(runner *Runner, outDir string, tasks []Task) *Executor {
	return &Executor{
		Runner: runner,
		OutDir: outDir,
		Tasks:  tasks,
		Quiet:  viper.GetBool("quiet"),
	}
}

// RunBootstrap invokes `tw bootstrap` as a subprocess.
func (e *Executor) RunBootstrap(ctx context.Context) error {
	if !e.Quiet {
		fmt.Println("\n==> Bootstrap (via CLI)")
	}

	args := []string{"bootstrap"}
	if e.Quiet {
		args = append(args, "--quiet")
	}

	result, err := e.Runner.Execute(ctx, args...)
	if err != nil {
		return fmt.Errorf("bootstrap failed: %w\nOutput: %s", err, result.Combined())
	}

	return nil
}

// TaskResult holds the result of executing a single eval task.
type TaskExecResult struct {
	TaskID   string
	Command  string
	Output   string
	Duration float64
	Success  bool
	Error    string
}

// RunTasks executes all CLI-based tasks and writes outputs.
func (e *Executor) RunTasks(ctx context.Context) ([]TaskExecResult, error) {
	var results []TaskExecResult

	// Ensure prompts directory exists
	promptsDir := filepath.Join(e.OutDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return nil, fmt.Errorf("create prompts dir: %w", err)
	}

	totalTasks := len(e.Tasks)
	for i, task := range e.Tasks {
		// Reset repo state between tasks if requested
		if e.ResetRepo && i > 0 {
			if _, err := e.Runner.ExecuteRaw(ctx, "git checkout . && git clean -fd"); err != nil {
				if !e.Quiet {
					fmt.Printf("  ⚠️  git reset failed: %v\n", err)
				}
			} else if !e.Quiet {
				fmt.Printf("  🔄 Reset repo state\n")
			}
		}

		if !e.Quiet {
			fmt.Printf("  [%d/%d] %s...", i+1, totalTasks, task.ID)
		}

		var result TaskExecResult
		result.TaskID = task.ID

		// Determine if this is a CLI command task or a legacy prompt task
		if task.Command != "" {
			// CLI command task - parse and execute
			result.Command = task.Command
			cmdResult, err := e.executeCommand(ctx, task)
			result.Output = cmdResult.Combined()
			result.Duration = cmdResult.Duration.Seconds()
			result.Success = err == nil
			if err != nil {
				result.Error = err.Error()
			}
		} else if task.Prompt != "" {
			// Legacy prompt task - use tw context or tw plan
			// Build a context query from the prompt
			result.Command = fmt.Sprintf("tw context '%s'", escapeShellArg(task.Prompt))
			cmdResult, err := e.Runner.Execute(ctx, "context", task.Prompt)
			result.Output = cmdResult.Combined()
			result.Duration = cmdResult.Duration.Seconds()
			result.Success = err == nil
			if err != nil {
				result.Error = err.Error()
			}
		} else {
			result.Error = "task has neither command nor prompt"
			result.Success = false
		}

		// Write output to file
		outputPath := filepath.Join(e.OutDir, fmt.Sprintf("task-%s.txt", task.ID))
		outputContent := fmt.Sprintf("# Task: %s\n# Command: %s\n\n%s\n\n[meta] duration=%.1fs success=%v\n",
			task.ID, result.Command, result.Output, result.Duration, result.Success)
		if err := os.WriteFile(outputPath, []byte(outputContent), 0644); err != nil {
			return results, fmt.Errorf("write output for %s: %w", task.ID, err)
		}

		if !e.Quiet {
			if result.Success {
				fmt.Printf(" ✓ (%.1fs)\n", result.Duration)
			} else {
				fmt.Printf(" ✗ (%.1fs)\n", result.Duration)
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// executeCommand parses and executes a CLI command from a task.
func (e *Executor) executeCommand(ctx context.Context, task Task) (Result, error) {
	// Parse the command string
	// Expected formats:
	//   "tw bootstrap"
	//   "tw context 'query here'"
	//   "tw plan new 'goal here'"
	cmd := strings.TrimSpace(task.Command)

	// If command starts with "tw ", strip it and use Runner
	if strings.HasPrefix(cmd, "tw ") {
		args := parseCommandArgs(strings.TrimPrefix(cmd, "tw "))
		return e.Runner.Execute(ctx, args...)
	}

	// Otherwise, execute as raw shell command
	return e.Runner.ExecuteRaw(ctx, cmd)
}

// parseCommandArgs splits a command string into arguments, respecting quotes.
func parseCommandArgs(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range cmd {
		switch {
		case (r == '\'' || r == '"') && !inQuote:
			inQuote = true
			quoteChar = r
		case r == quoteChar && inQuote:
			inQuote = false
			quoteChar = 0
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// escapeShellArg escapes a string for safe use in shell commands.
func escapeShellArg(s string) string {
	return strings.ReplaceAll(s, "'", "'\"'\"'")
}
