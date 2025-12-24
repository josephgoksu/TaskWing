package agents

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// ClarifyingAgent helps users refine their goals by asking questions.
type ClarifyingAgent struct {
	BaseAgent
}

// ClarifyingOutput defines the structured response from the LLM.
type ClarifyingOutput struct {
	Questions     []string `json:"questions"`
	EnrichedGoal  string   `json:"enriched_goal"`
	IsReadyToPlan bool     `json:"is_ready_to_plan"`
}

// NewClarifyingAgent creates a new agent for goal refinement.
func NewClarifyingAgent(cfg llm.Config) *ClarifyingAgent {
	return &ClarifyingAgent{
		BaseAgent: NewBaseAgent(
			"clarifying",
			"Refines user goals by asking clarifying questions",
			cfg,
		),
	}
}

// Run executes the clarification loop.
// Input.ExistingContext should contain:
// - "goal": string (The user's current goal/request)
// - "history": string (Optional: Previous Q&A transcript)
func (a *ClarifyingAgent) Run(ctx context.Context, input Input) (Output, error) {
	goal, ok := input.ExistingContext["goal"].(string)
	if !ok || goal == "" {
		return Output{}, fmt.Errorf("missing 'goal' in input context")
	}

	history, _ := input.ExistingContext["history"].(string)

	userPrompt := fmt.Sprintf("My Goal: %s\n\n", goal)
	if history != "" {
		userPrompt += fmt.Sprintf("Previous Clarifications:\n%s\n", history)
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: config.SystemPromptClarifyingAgent},
		{Role: schema.User, Content: userPrompt},
	}

	content, duration, err := a.GenerateWithTiming(ctx, messages)
	if err != nil {
		return Output{}, err
	}

	// Parse structured output
	parsed, err := ParseJSONResponse[ClarifyingOutput](content)
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
			Type:        "refinement", // Custom type for this agent
			Title:       "Goal Clarification",
			Description: parsed.EnrichedGoal,
			Metadata: map[string]any{
				"questions":        parsed.Questions,
				"is_ready_to_plan": parsed.IsReadyToPlan,
				"enriched_goal":    parsed.EnrichedGoal,
			},
		}},
		content,
		duration,
	), nil
}

func init() {
	RegisterAgentFactory("clarifying", func(cfg llm.Config) Agent {
		return NewClarifyingAgent(cfg)
	})
}
