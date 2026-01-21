package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// Runner manages the execution of multiple agents
type Runner struct {
	agents []core.Agent
}

// NewRunner creates a standard runner with default agents
func NewRunner(cfg llm.Config, projectPath string) *Runner {
	return &Runner{
		agents: NewDefaultAgents(cfg, projectPath),
	}
}

// Close releases resources held by all agents.
// Should be called when the runner is no longer needed.
func (r *Runner) Close() {
	core.CloseAgents(r.agents)
}

// RunOptions configures a runner execution.
type RunOptions struct {
	Workspace string // Workspace name for monorepo support ('root' for global, service name for scoped)
}

// Run executes all agents in parallel and returns raw agent outputs.
// Respects context cancellation - returns early if context is cancelled.
func (r *Runner) Run(ctx context.Context, projectPath string) ([]core.Output, error) {
	return r.RunWithOptions(ctx, projectPath, RunOptions{Workspace: "root"})
}

// RunWithOptions executes all agents with the given options.
// Respects context cancellation - returns early if context is cancelled.
func (r *Runner) RunWithOptions(ctx context.Context, projectPath string, opts RunOptions) ([]core.Output, error) {
	// Check for early cancellation before starting any work
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Default workspace to 'root' if not specified
	workspace := opts.Workspace
	if workspace == "" {
		workspace = "root"
	}

	input := core.Input{
		BasePath:    projectPath,
		ProjectName: filepath.Base(projectPath),
		Mode:        core.ModeBootstrap,
		Verbose:     true, // or configurable
		Workspace:   workspace,
	}

	var results []core.Output
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, agent := range r.agents {
		wg.Add(1)
		go func(a core.Agent) {
			defer wg.Done()

			// Check for cancellation before running agent
			select {
			case <-ctx.Done():
				mu.Lock()
				errs = append(errs, fmt.Errorf("agent %s cancelled: %w", a.Name(), ctx.Err()))
				mu.Unlock()
				return
			default:
			}

			// Run agent
			start := time.Now()
			out, err := a.Run(ctx, input)
			duration := time.Since(start)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				// We log/collect error but don't stop other agents
				errs = append(errs, fmt.Errorf("agent %s failed: %w", a.Name(), err))
				return
			}

			// Ensure duration is set if agent didn't set it
			if out.Duration == 0 {
				out.Duration = duration
			}

			results = append(results, out)
		}(agent)
	}

	wg.Wait()

	// Check if context was cancelled during execution
	if ctx.Err() != nil {
		return results, ctx.Err() // Return partial results with cancellation error
	}

	if len(results) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all agents failed: %v", errs)
	}

	// Return raw results (caller aggregates)
	return results, nil
}
