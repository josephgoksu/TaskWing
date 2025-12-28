/*
Package planning provides agents for goal refinement and task decomposition.
*/
package planning

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// ClarifyingAgent helps users refine their goals by asking questions.
type ClarifyingAgent struct {
	core.BaseAgent
	chain *core.DeterministicChain[ClarifyingOutput]
}

// ClarifyingOutput defines the structured response from the LLM.
type ClarifyingOutput struct {
	Questions     []string `json:"questions"`
	EnrichedGoal  string   `json:"enriched_goal"` // Best effort draft of the spec
	IsReadyToPlan bool     `json:"is_ready_to_plan"`
}

// NewClarifyingAgent creates a new agent for goal refinement.
func NewClarifyingAgent(cfg llm.Config) *ClarifyingAgent {
	return &ClarifyingAgent{
		BaseAgent: core.NewBaseAgent("clarifying", "Refines user goals by asking clarifying questions", cfg),
	}
}

// Run executes the clarification loop using Eino Chain.
func (a *ClarifyingAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	if a.chain == nil {
		chatModel, err := a.CreateChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		chain, err := core.NewDeterministicChain[ClarifyingOutput](
			ctx,
			a.Name(),
			chatModel,
			config.SystemPromptClarifyingAgent,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	goal, ok := input.ExistingContext["goal"].(string)
	if !ok || goal == "" {
		return core.Output{}, fmt.Errorf("missing 'goal' in input context")
	}

	history, _ := input.ExistingContext["history"].(string)
	context, _ := input.ExistingContext["context"].(string)

	chainInput := map[string]any{
		"Goal":    goal,
		"History": history,
		"Context": context,
	}

	parsed, raw, duration, err := a.chain.Invoke(ctx, chainInput)
	if err != nil {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("chain invoke: %w", err),
			Duration:  duration,
			RawOutput: raw,
		}, nil
	}

	return core.BuildOutput(
		a.Name(),
		[]core.Finding{{
			Type:        "refinement",
			Title:       "Goal Clarification",
			Description: parsed.EnrichedGoal,
			Metadata: map[string]any{
				"questions":        parsed.Questions,
				"is_ready_to_plan": parsed.IsReadyToPlan,
				"enriched_goal":    parsed.EnrichedGoal, // Ensure it's passed out
			},
		}},
		"JSON handled by Eino",
		duration,
	), nil
}

// AutoAnswer (Auto-Refine) uses the LLM to fill in the specification draft based on architectural context.
// When currentSpec is empty and there's only one question, it generates a concise answer.
// Otherwise, it updates the full specification.
func (a *ClarifyingAgent) AutoAnswer(ctx context.Context, currentSpec string, questions []string, kgContext string) (string, error) {
	var prompt string

	// Single question mode: generate concise answer
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
		// Full spec refinement mode
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

	chatModel, err := a.CreateChatModel(ctx)
	if err != nil {
		return "", fmt.Errorf("create model: %w", err)
	}

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate answer: %w", err)
	}

	return resp.Content, nil
}

// PlanningAgent decomposes goals into actionable tasks.
type PlanningAgent struct {
	core.BaseAgent
	chain *core.DeterministicChain[PlanningOutput]
}

// PlanningTask represents a single task in the plan.
type PlanningTask struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	ValidationSteps    []string `json:"validation_steps"`
	Priority           int      `json:"priority"`
	AssignedAgent      string   `json:"assigned_agent"`
}

// PlanningOutput defines the structured response from the LLM.
type PlanningOutput struct {
	Tasks     []PlanningTask `json:"tasks"`
	Rationale string         `json:"rationale"`
}

// NewPlanningAgent creates a new agent for task decomposition.
func NewPlanningAgent(cfg llm.Config) *PlanningAgent {
	return &PlanningAgent{
		BaseAgent: core.NewBaseAgent("planning", "Decomposes goals into actionable tasks with dependencies", cfg),
	}
}

// Run executes the planning logic using Eino Chain.
func (a *PlanningAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	if a.chain == nil {
		chatModel, err := a.CreateChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		chain, err := core.NewDeterministicChain[PlanningOutput](
			ctx,
			a.Name(),
			chatModel,
			config.SystemPromptPlanningAgent,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	goal, ok := input.ExistingContext["enriched_goal"].(string)
	if !ok || goal == "" {
		goal, _ = input.ExistingContext["goal"].(string)
	}
	if goal == "" {
		return core.Output{}, fmt.Errorf("missing 'enriched_goal' or 'goal' in input context")
	}

	kgContext, _ := input.ExistingContext["context"].(string)
	if kgContext == "" {
		kgContext = "No specific knowledge graph context provided."
	}

	chainInput := map[string]any{
		"Goal":    goal,
		"Context": kgContext,
	}

	parsed, raw, duration, err := a.chain.Invoke(ctx, chainInput)
	if err != nil {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("chain invoke: %w", err),
			Duration:  duration,
			RawOutput: raw,
		}, nil
	}

	return core.BuildOutput(
		a.Name(),
		[]core.Finding{{
			Type:        "plan",
			Title:       "Implementation Plan",
			Description: parsed.Rationale,
			Metadata:    map[string]any{"tasks": parsed.Tasks},
		}},
		"JSON handled by Eino",
		duration,
	), nil
}

func init() {
	core.RegisterAgentFactory("clarifying", func(cfg llm.Config, basePath string) core.Agent {
		return NewClarifyingAgent(cfg)
	})
	core.RegisterAgentFactory("planning", func(cfg llm.Config, basePath string) core.Agent {
		return NewPlanningAgent(cfg)
	})
}
