/*
Package core provides BaseAgent with shared functionality for all agents.
*/
package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/logger"
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

	// Track last prompt for crash logging
	logger.SetLastPrompt(formatMessagesForLogging(messages))

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("llm generate: %w", err)
	}
	return resp.Content, nil
}

// formatMessagesForLogging formats messages for crash log context.
func formatMessagesForLogging(messages []*schema.Message) string {
	var parts []string
	for _, m := range messages {
		var role string
		switch m.Role {
		case schema.User:
			role = "user"
		case schema.Assistant:
			role = "assistant"
		case schema.System:
			role = "system"
		default:
			role = "unknown"
		}
		parts = append(parts, fmt.Sprintf("[%s]: %s", role, m.Content))
	}
	return strings.Join(parts, "\n---\n")
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
