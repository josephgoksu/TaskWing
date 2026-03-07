package runner

import (
	"context"
	"fmt"
	"sync"
)

// Job represents a single unit of work to be executed by a Runner.
type Job struct {
	ID      string        // Identifier for tracking (e.g., "decisions", "patterns")
	Request InvokeRequest // The invocation request
}

// JobResult holds the outcome of a single job execution.
type JobResult struct {
	JobID    string        // Matches Job.ID
	Result   *InvokeResult // The invocation result (nil on error)
	Error    error         // Non-nil if the job failed
	RunnerType CLIType     // Which CLI handled the job
}

// Pool manages parallel and sequential execution of jobs across multiple runners.
type Pool struct {
	runners     []Runner
	concurrency int
}

// NewPool creates a pool from detected runners.
// Concurrency defaults to len(runners) if not specified.
func NewPool(runners []Runner, concurrency int) *Pool {
	if concurrency <= 0 {
		concurrency = len(runners)
	}
	if concurrency == 0 {
		concurrency = 1
	}
	return &Pool{
		runners:     runners,
		concurrency: concurrency,
	}
}

// Execute runs jobs in parallel, distributing them round-robin across available runners.
// Use this for read-only operations (analysis, planning) where jobs are independent.
func (p *Pool) Execute(ctx context.Context, jobs []Job) []JobResult {
	if len(p.runners) == 0 {
		results := make([]JobResult, len(jobs))
		for i, j := range jobs {
			results[i] = JobResult{JobID: j.ID, Error: fmt.Errorf("no runners available")}
		}
		return results
	}

	results := make([]JobResult, len(jobs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, p.concurrency)

	for i, job := range jobs {
		runner := p.runners[i%len(p.runners)]

		wg.Add(1)
		go func(idx int, j Job, r Runner) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := r.Invoke(ctx, j.Request)
			results[idx] = JobResult{
				JobID:      j.ID,
				Result:     result,
				Error:      err,
				RunnerType: r.Type(),
			}
		}(i, job, runner)
	}

	wg.Wait()
	return results
}

// ExecuteSequential runs jobs one at a time using the first runner.
// Use this for file-modifying operations (task execution) where order matters.
func (p *Pool) ExecuteSequential(ctx context.Context, jobs []Job) []JobResult {
	if len(p.runners) == 0 {
		return []JobResult{{Error: fmt.Errorf("no runners available")}}
	}

	runner := p.runners[0]
	results := make([]JobResult, len(jobs))

	for i, job := range jobs {
		if ctx.Err() != nil {
			results[i] = JobResult{
				JobID: job.ID,
				Error: ctx.Err(),
			}
			break
		}

		result, err := runner.InvokeWithFiles(ctx, job.Request)
		results[i] = JobResult{
			JobID:      job.ID,
			Result:     result,
			Error:      err,
			RunnerType: runner.Type(),
		}

		// Stop on failure
		if err != nil {
			break
		}
	}

	return results
}
