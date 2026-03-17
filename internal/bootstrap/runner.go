package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
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
		agents: NewDefaultAgents(cfg, projectPath, nil),
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

// RunWithOptions executes agents with the given options.
// For bootstrap mode, uses two-wave execution: doc+deps first, then code+git
// with context from wave 1. Watch mode uses single-wave parallel execution.
func (r *Runner) RunWithOptions(ctx context.Context, projectPath string, opts RunOptions) ([]core.Output, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	workspace := opts.Workspace
	if workspace == "" {
		workspace = "root"
	}

	input := core.Input{
		BasePath:    projectPath,
		ProjectName: filepath.Base(projectPath),
		Mode:        core.ModeBootstrap,
		Verbose:     true,
		Workspace:   workspace,
	}

	// Split agents into waves
	wave1Agents, wave2Agents := splitAgentsByWave(r.agents)

	// If no wave2 agents, just run everything in parallel (single wave)
	if len(wave2Agents) == 0 {
		return runParallel(ctx, r.agents, input)
	}

	// Wave 1: doc + deps (parallel)
	wave1Results, _ := runParallel(ctx, wave1Agents, input)
	// Wave 1 errors are non-fatal: code and git work independently.
	// Partial wave1 results still provide useful context for wave 2.

	// Build context from wave 1 findings for wave 2
	wave1Context := buildWaveContext(wave1Results)

	// Wave 2: code + git (parallel, with wave 1 context)
	wave2Input := input
	if wave2Input.ExistingContext == nil {
		wave2Input.ExistingContext = make(map[string]any)
	}
	for k, v := range wave1Context {
		wave2Input.ExistingContext[k] = v
	}

	wave2Results, err := runParallel(ctx, wave2Agents, wave2Input)
	if err != nil {
		// Return wave 1 results even if wave 2 fails
		return append(wave1Results, wave2Results...), err
	}

	return append(wave1Results, wave2Results...), nil
}

// splitAgentsByWave separates agents into wave 1 (doc, deps) and wave 2 (code, git).
func splitAgentsByWave(agents []core.Agent) (wave1, wave2 []core.Agent) {
	for _, a := range agents {
		switch a.Name() {
		case "doc", "deps":
			wave1 = append(wave1, a)
		default:
			wave2 = append(wave2, a)
		}
	}
	return wave1, wave2
}

// buildWaveContext converts wave 1 outputs into context for wave 2 agents.
// Truncates descriptions and total size to avoid blowing up the code agent's context budget.
func buildWaveContext(results []core.Output) map[string]any {
	const maxDescLen = 200   // Truncate individual descriptions
	const maxSummaryLen = 6000 // Cap total summary (~1.5k tokens)

	var summaryParts []string
	totalLen := 0
	for _, r := range results {
		if r.Error != nil || len(r.Findings) == 0 {
			continue
		}
		for _, f := range r.Findings {
			desc := f.Description
			if len(desc) > maxDescLen {
				desc = desc[:maxDescLen] + "..."
			}
			part := fmt.Sprintf("- [%s] %s: %s", f.Type, f.Title, desc)
			totalLen += len(part) + 1
			if totalLen > maxSummaryLen {
				summaryParts = append(summaryParts, "... (truncated)")
				break
			}
			summaryParts = append(summaryParts, part)
		}
		if totalLen > maxSummaryLen {
			break
		}
	}
	if len(summaryParts) == 0 {
		return nil
	}
	return map[string]any{
		"wave1_summary": strings.Join(summaryParts, "\n"),
	}
}

// runParallel executes agents concurrently and collects results.
func runParallel(ctx context.Context, agents []core.Agent, input core.Input) ([]core.Output, error) {
	var results []core.Output
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, agent := range agents {
		wg.Add(1)
		go func(a core.Agent) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				mu.Lock()
				errs = append(errs, fmt.Errorf("agent %s cancelled: %w", a.Name(), ctx.Err()))
				mu.Unlock()
				return
			default:
			}

			start := time.Now()
			out, err := a.Run(ctx, input)
			duration := time.Since(start)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errs = append(errs, fmt.Errorf("agent %s failed: %w", a.Name(), err))
				return
			}

			if out.Duration == 0 {
				out.Duration = duration
			}

			results = append(results, out)
		}(agent)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return results, ctx.Err()
	}

	if len(results) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all agents failed: %v", errs)
	}

	return results, nil
}
