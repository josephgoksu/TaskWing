package agents

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// PlanningAgent decomposes goals into actionable tasks.
type PlanningAgent struct {
	BaseAgent
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
		BaseAgent: NewBaseAgent(
			"planning",
			"Decomposes goals into actionable tasks with dependencies",
			cfg,
		),
	}
}

// Run executes the planning logic.
// Input.ExistingContext should contain:
// - "enriched_goal": string
// - "context": string (Relevant knowledge graph nodes summary)
func (a *PlanningAgent) Run(ctx context.Context, input Input) (Output, error) {
	goal, ok := input.ExistingContext["enriched_goal"].(string)
	if !ok || goal == "" {
		// Fallback to "goal" if enriched is missing
		goal, _ = input.ExistingContext["goal"].(string)
	}
	if goal == "" {
		return Output{}, fmt.Errorf("missing 'enriched_goal' or 'goal' in input context")
	}

	kgContext, _ := input.ExistingContext["context"].(string)
	if kgContext == "" {
		kgContext = "No specific knowledge graph context provided."
	}

	// Prepare System Prompt with Template
	tmpl, err := template.New("prompt").Parse(config.SystemPromptPlanningAgent)
	if err != nil {
		return Output{}, fmt.Errorf("parse prompt template: %w", err)
	}

	var systemPromptBuf bytes.Buffer
	if err := tmpl.Execute(&systemPromptBuf, map[string]string{
		"Goal":    goal,
		"Context": kgContext,
	}); err != nil {
		return Output{}, fmt.Errorf("execute prompt template: %w", err)
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: systemPromptBuf.String()},
		{Role: schema.User, Content: "Please generate the implementation plan."},
	}

	content, duration, err := a.GenerateWithTiming(ctx, messages)
	if err != nil {
		return Output{}, err
	}

	// Parse structured output
	parsed, err := ParseJSONResponse[PlanningOutput](content)
	if err != nil {
		return Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("parse output: %w", err),
			RawOutput: content,
			Duration:  duration,
		}, nil
	}

	return BuildOutput(
		a.Name(),
		[]Finding{{
			Type:        "plan",
			Title:       "Implementation Plan",
			Description: parsed.Rationale,
			Metadata: map[string]any{
				"tasks": parsed.Tasks,
			},
		}},
		content,
		duration,
	), nil
}

func init() {
	RegisterAgentFactory("planning", func(cfg llm.Config) Agent {
		return NewPlanningAgent(cfg)
	})
}
