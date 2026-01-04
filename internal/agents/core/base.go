/*
Package core provides BaseAgent with shared functionality for all agents.
*/
package core

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// BaseAgent provides shared functionality for all LLM-powered agents.
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
// Deprecated: Use CreateCloseableChatModel and call Close() when done.
func (b *BaseAgent) CreateChatModel(ctx context.Context) (model.BaseChatModel, error) {
	chatModel, err := llm.NewCloseableChatModel(ctx, b.llmConfig)
	if err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}
	return chatModel.BaseChatModel, nil
}

// CreateCloseableChatModel creates an LLM chat model with proper resource management.
// Callers MUST call Close() when done to release resources.
func (b *BaseAgent) CreateCloseableChatModel(ctx context.Context) (*llm.CloseableChatModel, error) {
	chatModel, err := llm.NewCloseableChatModel(ctx, b.llmConfig)
	if err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}
	return chatModel, nil
}

// Generate sends messages to the LLM and returns the response content.
func (b *BaseAgent) Generate(ctx context.Context, messages []*schema.Message) (string, error) {
	chatModel, err := b.CreateCloseableChatModel(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = chatModel.Close() }()

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("llm generate: %w", err)
	}
	return resp.Content, nil
}

// GenerateFromPrompt is a convenience method for single-prompt calls.
func (b *BaseAgent) GenerateFromPrompt(ctx context.Context, prompt string) (string, error) {
	return b.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
}

// GenerateWithTiming sends messages and returns content with duration.
func (b *BaseAgent) GenerateWithTiming(ctx context.Context, messages []*schema.Message) (string, time.Duration, error) {
	start := time.Now()
	content, err := b.Generate(ctx, messages)
	return content, time.Since(start), err
}

// RunInfo is a type alias for callback injection.
type RunInfo = callbacks.RunInfo
