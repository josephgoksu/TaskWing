/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package agents

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// Orchestrator manages the parallel execution of multiple agents
type Orchestrator struct {
	agents  []Agent
	verbose bool
	out     *os.File
}

// NewOrchestrator creates a new orchestrator with the given agents
func NewOrchestrator(agents []Agent, verbose bool) *Orchestrator {
	return &Orchestrator{
		agents:  agents,
		verbose: verbose,
		out:     os.Stderr,
	}
}

// RunAll executes all agents in parallel and aggregates their outputs
func (o *Orchestrator) RunAll(ctx context.Context, input Input) ([]Output, error) {
	if len(o.agents) == 0 {
		return nil, fmt.Errorf("no agents configured")
	}

	input.Verbose = o.verbose

	fmt.Fprintf(o.out, "   🤖 Running %d agents in parallel...\n", len(o.agents))

	var wg sync.WaitGroup
	outputs := make([]Output, len(o.agents))
	errors := make([]error, len(o.agents))

	for i, agent := range o.agents {
		wg.Add(1)
		go func(idx int, a Agent) {
			defer wg.Done()

			start := time.Now()
			if o.verbose {
				fmt.Fprintf(o.out, "      → Starting %s agent\n", a.Name())
			}

			out, err := a.Run(ctx, input)
			out.AgentName = a.Name()
			out.Duration = time.Since(start)

			if err != nil {
				errors[idx] = err
				out.Error = err
			}

			outputs[idx] = out

			if o.verbose {
				fmt.Fprintf(o.out, "      ✓ %s completed in %.1fs (%d findings)\n",
					a.Name(), out.Duration.Seconds(), len(out.Findings))
			}
		}(i, agent)
	}

	wg.Wait()

	// Report summary
	totalFindings := 0
	for _, out := range outputs {
		totalFindings += len(out.Findings)
	}
	fmt.Fprintf(o.out, "   ✓ All agents completed: %d findings total\n", totalFindings)

	return outputs, nil
}

// AggregateFindings combines findings from all agent outputs
func AggregateFindings(outputs []Output) []Finding {
	var all []Finding
	for _, out := range outputs {
		for _, f := range out.Findings {
			f.SourceAgent = out.AgentName
			all = append(all, f)
		}
	}
	return all
}

// GroupFindingsByType organizes findings by their type
func GroupFindingsByType(findings []Finding) map[FindingType][]Finding {
	grouped := make(map[FindingType][]Finding)
	for _, f := range findings {
		grouped[f.Type] = append(grouped[f.Type], f)
	}
	return grouped
}
