/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package agents

import (
	"context"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// PromptAgent is a generic, configurable agent that can be used for any LLM task.
// Configure it with a system prompt and optional output parser.
type PromptAgent struct {
	BaseAgent
	systemPrompt string
	parseJSON    bool
}

// PromptAgentConfig configures a PromptAgent.
type PromptAgentConfig struct {
	Name         string
	Description  string
	SystemPrompt string
	LLMConfig    llm.Config
	ParseJSON    bool
}

// NewPromptAgent creates a configurable agent.
func NewPromptAgent(cfg PromptAgentConfig) *PromptAgent {
	return &PromptAgent{
		BaseAgent:    NewBaseAgent(cfg.Name, cfg.Description, cfg.LLMConfig),
		systemPrompt: cfg.SystemPrompt,
		parseJSON:    cfg.ParseJSON,
	}
}

// PromptResult is the output from a PromptAgent.
type PromptResult struct {
	AgentName  string
	Content    string
	Structured map[string]any
	Duration   time.Duration
	Warnings   []string
}

// Run executes the agent with the given user prompt.
func (a *PromptAgent) Run(ctx context.Context, userPrompt string) (*PromptResult, error) {
	messages := []*schema.Message{
		schema.SystemMessage(a.systemPrompt),
		schema.UserMessage(userPrompt),
	}
	return a.runWithMessages(ctx, messages)
}

// RunWithMessages allows full control over the message chain.
func (a *PromptAgent) RunWithMessages(ctx context.Context, messages []*schema.Message) (*PromptResult, error) {
	return a.runWithMessages(ctx, messages)
}

// runWithMessages is the shared implementation (DRY).
func (a *PromptAgent) runWithMessages(ctx context.Context, messages []*schema.Message) (*PromptResult, error) {
	start := time.Now()

	content, err := a.Generate(ctx, messages)
	if err != nil {
		return nil, err
	}

	result := &PromptResult{
		AgentName: a.Name(),
		Content:   content,
		Duration:  time.Since(start),
	}

	if a.parseJSON {
		// Use shared ParseJSONResponseToMap from base_agent.go
		structured, parseErr := ParseJSONResponseToMap(content)
		if parseErr != nil {
			result.Warnings = append(result.Warnings, "Failed to parse JSON: "+parseErr.Error())
		} else {
			result.Structured = structured
		}
	}

	return result, nil
}
