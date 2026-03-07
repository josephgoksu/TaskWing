package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/config"
)

// RunnerClarifier implements app.GoalsClarifier using an AI CLI runner.
type RunnerClarifier struct {
	runner Runner
}

// NewRunnerClarifier creates a clarifier backed by an AI CLI runner.
func NewRunnerClarifier(r Runner) *RunnerClarifier {
	return &RunnerClarifier{runner: r}
}

// Close is a no-op for runner-backed agents (no persistent resources).
func (rc *RunnerClarifier) Close() error { return nil }

// Run executes clarification via the AI CLI runner.
func (rc *RunnerClarifier) Run(ctx context.Context, input core.Input) (core.Output, error) {
	goal, _ := input.ExistingContext["goal"].(string)
	if goal == "" {
		return core.Output{}, fmt.Errorf("missing 'goal' in input context")
	}

	history, _ := input.ExistingContext["history"].(string)
	kgContext, _ := input.ExistingContext["context"].(string)

	prompt, err := RenderTemplate(config.SystemPromptClarifyingAgent, map[string]any{
		"Goal":    goal,
		"History": history,
		"Context": kgContext,
	})
	if err != nil {
		return core.Output{}, fmt.Errorf("render clarify template: %w", err)
	}

	prompt += "\n\nRespond with ONLY a JSON object matching the Output Format above."

	result, err := RetryableInvoke(ctx, rc.runner, InvokeRequest{
		Prompt:  prompt,
		WorkDir: workDir(),
		Timeout: 5 * time.Minute,
	}, defaultRetries)
	if err != nil {
		return core.Output{Error: fmt.Errorf("runner clarify: %w", err)}, nil
	}

	var parsed impl.ClarifyingOutput
	if err := result.Decode(&parsed); err != nil {
		return core.Output{Error: fmt.Errorf("decode clarify output: %w", err)}, nil
	}

	return core.BuildOutput(
		"runner-clarifier",
		[]core.Finding{{
			Type:        "refinement",
			Title:       "Goal Clarification",
			Description: parsed.EnrichedGoal,
			Metadata: map[string]any{
				"questions":        parsed.Questions,
				"is_ready_to_plan": parsed.IsReadyToPlan,
				"goal_summary":     parsed.GoalSummary,
				"enriched_goal":    parsed.EnrichedGoal,
			},
		}},
		"runner-backed",
		0,
	), nil
}

// AutoAnswer uses the runner to autonomously answer clarification questions.
func (rc *RunnerClarifier) AutoAnswer(ctx context.Context, currentSpec string, questions []string, kgContext string) (string, error) {
	var prompt string

	if currentSpec == "" && len(questions) == 1 {
		prompt = fmt.Sprintf(`You are a Senior Architect answering a clarification question.

**Project Context:**
%s

**Question:**
%s

**Instructions:**
- FIRST: Check Project Context above for the answer - extract and use it if found
- Answer in 1-3 sentences maximum
- Be direct and specific - no hedging
- Do not ask follow-up questions
- If context doesn't have the answer, infer from the project's patterns

Answer:`, kgContext, questions[0])
	} else {
		qs := strings.Join(questions, "\n- ")
		prompt = fmt.Sprintf(`You are the Senior Architect of this project.
Your goal is to refine a technical specification by addressing remaining ambiguities using your architectural knowledge.

**Context (Source of Truth):**
%s

**Remaining Questions/Ambiguities:**
- %s

**Current Specification Draft:**
%s

**Your Mission:**
Incorporate the most suitable, professional, and minimal architectural decisions into the specification to address the questions.
Act as if the user said "Yes, proceed with the best practice for these questions".
Respond ONLY with the FULL, UPDATED technical specification. Use professional language.`, kgContext, qs, currentSpec)
	}

	result, err := RetryableInvoke(ctx, rc.runner, InvokeRequest{
		Prompt:  prompt,
		WorkDir: workDir(),
		Timeout: 3 * time.Minute,
	}, defaultRetries)
	if err != nil {
		return "", fmt.Errorf("runner auto-answer: %w", err)
	}

	// AutoAnswer returns raw text, not JSON.
	// For Claude, unwrap the JSON envelope to get the text content.
	raw := result.RawOutput
	if result.CLIType == CLIClaude {
		raw = unwrapClaudeEnvelope(raw)
	}
	return strings.TrimSpace(raw), nil
}

// RunnerPlanner implements app.TaskPlanner using an AI CLI runner.
type RunnerPlanner struct {
	runner Runner
}

// NewRunnerPlanner creates a planner backed by an AI CLI runner.
func NewRunnerPlanner(r Runner) *RunnerPlanner {
	return &RunnerPlanner{runner: r}
}

// Close is a no-op for runner-backed agents.
func (rp *RunnerPlanner) Close() error { return nil }

// Run executes planning via the AI CLI runner.
func (rp *RunnerPlanner) Run(ctx context.Context, input core.Input) (core.Output, error) {
	goal, _ := input.ExistingContext["enriched_goal"].(string)
	if goal == "" {
		goal, _ = input.ExistingContext["goal"].(string)
	}
	if goal == "" {
		return core.Output{}, fmt.Errorf("missing 'enriched_goal' or 'goal' in input context")
	}

	kgContext, _ := input.ExistingContext["context"].(string)
	if kgContext == "" {
		kgContext = "No specific knowledge graph context provided."
	}

	prompt, err := RenderTemplate(config.SystemPromptPlanningAgent, map[string]any{
		"Goal":    goal,
		"Context": kgContext,
	})
	if err != nil {
		return core.Output{}, fmt.Errorf("render planning template: %w", err)
	}

	prompt += "\n\nRespond with ONLY a JSON object matching the Output Format above."

	result, err := RetryableInvoke(ctx, rp.runner, InvokeRequest{
		Prompt:  prompt,
		WorkDir: workDir(),
		Timeout: 5 * time.Minute,
	}, defaultRetries)
	if err != nil {
		return core.Output{Error: fmt.Errorf("runner planning: %w", err)}, nil
	}

	var parsed impl.PlanningOutput
	if err := result.Decode(&parsed); err != nil {
		return core.Output{Error: fmt.Errorf("decode planning output: %w", err)}, nil
	}

	return core.BuildOutput(
		"runner-planner",
		[]core.Finding{{
			Type:        "plan",
			Title:       "Implementation Plan",
			Description: parsed.Rationale,
			Metadata:    map[string]any{"tasks": parsed.Tasks},
		}},
		"runner-backed",
		0,
	), nil
}

// RunnerDecomposer implements app.PhaseGoalDecomposer using an AI CLI runner.
type RunnerDecomposer struct {
	runner Runner
}

// NewRunnerDecomposer creates a decomposer backed by an AI CLI runner.
func NewRunnerDecomposer(r Runner) *RunnerDecomposer {
	return &RunnerDecomposer{runner: r}
}

// Close is a no-op for runner-backed agents.
func (rd *RunnerDecomposer) Close() error { return nil }

// Run executes decomposition via the AI CLI runner.
func (rd *RunnerDecomposer) Run(ctx context.Context, input core.Input) (core.Output, error) {
	enrichedGoal, _ := input.ExistingContext["enriched_goal"].(string)
	if enrichedGoal == "" {
		return core.Output{}, fmt.Errorf("missing 'enriched_goal' in input context")
	}

	kgContext, _ := input.ExistingContext["context"].(string)
	if kgContext == "" {
		kgContext = "No specific knowledge graph context provided."
	}

	prompt, err := RenderTemplate(config.SystemPromptDecompositionAgent, map[string]any{
		"EnrichedGoal": enrichedGoal,
		"Context":      kgContext,
	})
	if err != nil {
		return core.Output{}, fmt.Errorf("render decomposition template: %w", err)
	}

	prompt += "\n\nRespond with ONLY a JSON object matching the Output Format above."

	result, err := RetryableInvoke(ctx, rd.runner, InvokeRequest{
		Prompt:  prompt,
		WorkDir: workDir(),
		Timeout: 5 * time.Minute,
	}, defaultRetries)
	if err != nil {
		return core.Output{Error: fmt.Errorf("runner decomposition: %w", err)}, nil
	}

	var parsed impl.DecompositionOutput
	if err := result.Decode(&parsed); err != nil {
		return core.Output{Error: fmt.Errorf("decode decomposition output: %w", err)}, nil
	}

	return core.BuildOutput(
		"runner-decomposer",
		[]core.Finding{{
			Type:        "decomposition",
			Title:       "Goal Decomposition",
			Description: parsed.Rationale,
			Metadata: map[string]any{
				"phases":    parsed.Phases,
				"rationale": parsed.Rationale,
			},
		}},
		"runner-backed",
		0,
	), nil
}

// RunnerExpander implements app.PhaseExpander using an AI CLI runner.
type RunnerExpander struct {
	runner Runner
}

// NewRunnerExpander creates an expander backed by an AI CLI runner.
func NewRunnerExpander(r Runner) *RunnerExpander {
	return &RunnerExpander{runner: r}
}

// Close is a no-op for runner-backed agents.
func (re *RunnerExpander) Close() error { return nil }

// Run executes phase expansion via the AI CLI runner.
func (re *RunnerExpander) Run(ctx context.Context, input core.Input) (core.Output, error) {
	phaseTitle, _ := input.ExistingContext["phase_title"].(string)
	if phaseTitle == "" {
		return core.Output{}, fmt.Errorf("missing 'phase_title' in input context")
	}

	phaseDescription, _ := input.ExistingContext["phase_description"].(string)
	enrichedGoal, _ := input.ExistingContext["enriched_goal"].(string)
	kgContext, _ := input.ExistingContext["context"].(string)
	if kgContext == "" {
		kgContext = "No specific knowledge graph context provided."
	}

	prompt, err := RenderTemplate(config.SystemPromptExpandAgent, map[string]any{
		"PhaseTitle":       phaseTitle,
		"PhaseDescription": phaseDescription,
		"EnrichedGoal":     enrichedGoal,
		"Context":          kgContext,
	})
	if err != nil {
		return core.Output{}, fmt.Errorf("render expand template: %w", err)
	}

	prompt += "\n\nRespond with ONLY a JSON object matching the Output Format above."

	result, err := RetryableInvoke(ctx, re.runner, InvokeRequest{
		Prompt:  prompt,
		WorkDir: workDir(),
		Timeout: 5 * time.Minute,
	}, defaultRetries)
	if err != nil {
		return core.Output{Error: fmt.Errorf("runner expand: %w", err)}, nil
	}

	var parsed impl.ExpandOutput
	if err := result.Decode(&parsed); err != nil {
		return core.Output{Error: fmt.Errorf("decode expand output: %w", err)}, nil
	}

	return core.BuildOutput(
		"runner-expander",
		[]core.Finding{{
			Type:        "expansion",
			Title:       "Phase Expansion: " + phaseTitle,
			Description: parsed.Rationale,
			Metadata: map[string]any{
				"tasks":     parsed.Tasks,
				"rationale": parsed.Rationale,
			},
		}},
		"runner-backed",
		0,
	), nil
}

// workDir returns the current working directory, falling back to ".".
func workDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
