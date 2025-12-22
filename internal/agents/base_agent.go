/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package agents provides BaseAgent with shared functionality for all agents.
*/
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// BaseAgent provides shared functionality for all LLM-powered agents.
// Embed this struct in your agent to get common methods for free.
type BaseAgent struct {
	name        string
	description string
	llmConfig   llm.Config
}

// NewBaseAgent creates a new BaseAgent with the given configuration.
func NewBaseAgent(name, description string, cfg llm.Config) BaseAgent {
	return BaseAgent{
		name:        name,
		description: description,
		llmConfig:   cfg,
	}
}

// Name returns the agent identifier.
func (b *BaseAgent) Name() string { return b.name }

// Description returns the agent description.
func (b *BaseAgent) Description() string { return b.description }

// LLMConfig returns the LLM configuration for this agent.
func (b *BaseAgent) LLMConfig() llm.Config { return b.llmConfig }

// CreateChatModel creates an LLM chat model using the agent's config.
// This eliminates the repeated pattern of creating models in each agent.
func (b *BaseAgent) CreateChatModel(ctx context.Context) (model.BaseChatModel, error) {
	chatModel, err := llm.NewChatModel(ctx, b.llmConfig)
	if err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}
	return chatModel, nil
}

// Generate sends messages to the LLM and returns the response content.
// This is a convenience method that wraps model creation and generation.
func (b *BaseAgent) Generate(ctx context.Context, messages []*schema.Message) (string, error) {
	chatModel, err := b.CreateChatModel(ctx)
	if err != nil {
		return "", err
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("llm generate: %w", err)
	}

	return resp.Content, nil
}

// GenerateWithTiming sends messages and returns content with duration.
func (b *BaseAgent) GenerateWithTiming(ctx context.Context, messages []*schema.Message) (string, time.Duration, error) {
	start := time.Now()
	content, err := b.Generate(ctx, messages)
	return content, time.Since(start), err
}

// ParseJSONResponse extracts JSON from LLM response, handling markdown code blocks.
// This centralizes the repeated JSON parsing logic across agents.
func ParseJSONResponse[T any](response string) (T, error) {
	var result T

	// Strip markdown code blocks
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return result, fmt.Errorf("parse JSON: %w", err)
	}

	return result, nil
}

// ParseJSONResponseToMap is a non-generic version that returns map[string]any.
// Use this when you don't know the exact structure ahead of time.
func ParseJSONResponseToMap(response string) (map[string]any, error) {
	return ParseJSONResponse[map[string]any](response)
}

// BuildOutput creates a standard Output struct with findings.
func BuildOutput(agentName string, findings []Finding, rawOutput string, duration time.Duration) Output {
	return Output{
		AgentName: agentName,
		Findings:  findings,
		RawOutput: rawOutput,
		Duration:  duration,
	}
}
