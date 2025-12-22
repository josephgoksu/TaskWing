package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// Runner manages the execution of multiple agents
type Runner struct {
	agents []agents.Agent
}

// NewRunner creates a standard runner with default agents
func NewRunner(cfg llm.Config, projectPath string) *Runner {
	return &Runner{
		agents: NewDefaultAgents(cfg, projectPath),
	}
}

// Run executes all agents in parallel and returns aggregated findings
func (r *Runner) Run(ctx context.Context, projectPath string) ([]agents.Finding, error) {
	input := agents.Input{
		BasePath:    projectPath,
		ProjectName: filepath.Base(projectPath),
		Mode:        agents.ModeBootstrap,
		Verbose:     true, // or configurable
	}

	var results []agents.Output
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, agent := range r.agents {
		wg.Add(1)
		go func(a agents.Agent) {
			defer wg.Done()

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

	if len(results) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all agents failed: %v", errs)
	}

	// Aggregate findings
	return agents.AggregateFindings(results), nil
}
