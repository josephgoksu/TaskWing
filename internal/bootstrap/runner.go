package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
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
	llmCfg llm.Config
}

// NewRunner creates a standard runner with default agents
func NewRunner(cfg llm.Config, projectPath string) *Runner {
	return &Runner{
		agents: NewDefaultAgents(cfg, projectPath, nil),
		llmCfg: cfg,
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

// ProviderSupportsBatch returns true if the provider has a batch API with cost savings.
func ProviderSupportsBatch(provider llm.Provider) bool {
	switch provider {
	case llm.ProviderOpenAI, llm.ProviderAnthropic:
		return true
	default:
		return false
	}
}

// Run executes all agents in parallel and returns raw agent outputs.
// Respects context cancellation - returns early if context is cancelled.
func (r *Runner) Run(ctx context.Context, projectPath string) ([]core.Output, error) {
	return r.RunWithOptions(ctx, projectPath, RunOptions{Workspace: "root"})
}

// RunWithOptions executes agents with the given options.
// Automatically uses the Batch API when the provider supports it (OpenAI, Anthropic)
// for 50% cost reduction on batchable agents. Non-batchable agents run in parallel.
// Falls back to sync execution if batch submission fails.
func (r *Runner) RunWithOptions(ctx context.Context, projectPath string, opts RunOptions) ([]core.Output, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Auto-batch when provider supports it and there are batchable agents
	if ProviderSupportsBatch(r.llmCfg.Provider) && hasBatchableAgents(r.agents) {
		results, err := r.runWithBatch(ctx, projectPath, opts)
		if err == nil {
			return results, nil
		}
		slog.Warn("batch execution failed, falling back to sync", "error", err)
	}

	return r.runSync(ctx, projectPath, opts)
}

// runSync is the original wave-based parallel execution path.
func (r *Runner) runSync(ctx context.Context, projectPath string, opts RunOptions) ([]core.Output, error) {
	workspace := opts.Workspace
	if workspace == "" {
		workspace = "root"
	}

	input := core.Input{
		BasePath:    projectPath,
		ProjectName: filepath.Base(projectPath),
		Mode:        core.ModeBootstrap,
		Verbose:     false,
		Workspace:   workspace,
	}

	// Split agents into waves
	wave1Agents, wave2Agents := splitAgentsByWave(r.agents)

	// If no wave2 agents, just run everything in parallel (single wave)
	if len(wave2Agents) == 0 {
		return runParallel(ctx, r.agents, input)
	}

	// Wave 1: doc + deps (parallel)
	wave1Results, wave1Err := runParallel(ctx, wave1Agents, input)
	if wave1Err != nil {
		slog.Debug("wave 1 agents returned errors (non-fatal)", "error", wave1Err)
	}

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
		return append(wave1Results, wave2Results...), err
	}

	return append(wave1Results, wave2Results...), nil
}

// hasBatchableAgents returns true if any agent implements BatchableAgent.
func hasBatchableAgents(agents []core.Agent) bool {
	for _, a := range agents {
		if _, ok := a.(core.BatchableAgent); ok {
			return true
		}
	}
	return false
}

// runWithBatch executes batchable agents via the Batch API and non-batchable agents in parallel.
// Called automatically by RunWithOptions when the provider supports batch.
func (r *Runner) runWithBatch(ctx context.Context, projectPath string, opts RunOptions) ([]core.Output, error) {
	workspace := opts.Workspace
	if workspace == "" {
		workspace = "root"
	}

	input := core.Input{
		BasePath:    projectPath,
		ProjectName: filepath.Base(projectPath),
		Mode:        core.ModeBootstrap,
		Verbose:     false,
		Workspace:   workspace,
	}

	// Split agents into batchable vs non-batchable
	var batchable []core.BatchableAgent
	var nonBatchable []core.Agent
	for _, a := range r.agents {
		if ba, ok := a.(core.BatchableAgent); ok {
			batchable = append(batchable, ba)
		} else {
			nonBatchable = append(nonBatchable, a)
		}
	}

	// If nothing is batchable, fall back to sync
	if len(batchable) == 0 {
		return r.runSync(ctx, projectPath, opts)
	}

	// Phase 1: Prepare batch requests (render prompts without calling LLM)
	var batchRequests []llm.BatchRequest
	var batchAgents []core.BatchableAgent // track which agents mapped to which requests
	for _, ba := range batchable {
		msgs, err := ba.PrepareForBatch(ctx, input)
		if err != nil {
			slog.Debug("agent not batchable, will run sync", "agent", ba.Name(), "error", err)
			nonBatchable = append(nonBatchable, ba)
			continue
		}

		var batchMsgs []llm.BatchMessage
		for _, m := range msgs {
			batchMsgs = append(batchMsgs, llm.BatchMessage{Role: m.Role, Content: m.Content})
		}

		batchRequests = append(batchRequests, llm.BatchRequest{
			CustomID: ba.Name(),
			Model:    r.llmCfg.Model,
			Messages: batchMsgs,
		})
		batchAgents = append(batchAgents, ba)
	}

	// Phase 2: Run non-batchable agents in parallel while batch processes
	var nonBatchResults []core.Output
	var batchResults []core.Output
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Start non-batchable agents immediately
	if len(nonBatchable) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results, _ := runParallel(ctx, nonBatchable, input)
			mu.Lock()
			nonBatchResults = results
			mu.Unlock()
		}()
	}

	// Submit and wait for batch
	if len(batchRequests) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results := r.executeBatch(ctx, batchRequests, batchAgents)
			mu.Lock()
			batchResults = results
			mu.Unlock()
		}()
	}

	wg.Wait()

	return append(nonBatchResults, batchResults...), nil
}

// executeBatch submits batch requests and parses results back into agent outputs.
// Falls back to sync execution for individual agents if batch fails.
func (r *Runner) executeBatch(ctx context.Context, requests []llm.BatchRequest, agents []core.BatchableAgent) []core.Output {
	client := llm.NewBatchClient(r.llmCfg.APIKey, r.llmCfg.BaseURL)

	batchID, err := client.Submit(ctx, requests)
	if err != nil {
		slog.Warn("batch submission failed, running agents synchronously", "error", err)
		return r.runBatchableFallback(ctx, agents)
	}

	slog.Debug("batch submitted", "batch_id", batchID, "agents", len(requests))

	// Poll for completion (5 second intervals)
	results, err := client.WaitForCompletion(ctx, batchID, 5*time.Second, nil)
	if err != nil {
		slog.Warn("batch failed, running agents synchronously", "error", err)
		return r.runBatchableFallback(ctx, agents)
	}

	// Map results back to agents by custom_id
	resultMap := make(map[string]llm.BatchResult)
	for _, r := range results {
		resultMap[r.CustomID] = r
	}

	var outputs []core.Output
	for _, agent := range agents {
		result, ok := resultMap[agent.Name()]
		if !ok || result.StatusCode != 200 || result.Content == "" {
			slog.Warn("batch result missing or failed for agent", "agent", agent.Name(),
				"found", ok, "status", result.StatusCode, "error", result.Error)
			continue
		}

		output, err := agent.ParseBatchResult(result.Content)
		if err != nil {
			slog.Warn("failed to parse batch result", "agent", agent.Name(), "error", err)
			continue
		}
		outputs = append(outputs, output)
	}

	return outputs
}

// runBatchableFallback runs batchable agents via their normal Run method.
func (r *Runner) runBatchableFallback(ctx context.Context, agents []core.BatchableAgent) []core.Output {
	var regularAgents []core.Agent
	for _, a := range agents {
		regularAgents = append(regularAgents, a)
	}
	input := core.Input{
		Mode: core.ModeBootstrap,
	}
	results, _ := runParallel(ctx, regularAgents, input)
	return results
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
	const maxDescLen = 400      // Truncate individual descriptions
	const maxSummaryLen = 12000 // Cap total summary (~3k tokens)

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
