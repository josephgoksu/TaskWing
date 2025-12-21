package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// PromptAgent is a generic, configurable agent that can be used for any LLM task.
// Configure it with a system prompt and optional output parser.
type PromptAgent struct {
	name         string
	description  string
	systemPrompt string
	llmConfig    llm.Config
	parseJSON    bool // if true, attempts to parse response as JSON
}

// PromptAgentConfig configures a PromptAgent
type PromptAgentConfig struct {
	Name         string
	Description  string
	SystemPrompt string
	LLMConfig    llm.Config
	ParseJSON    bool
}

// NewPromptAgent creates a configurable agent
func NewPromptAgent(cfg PromptAgentConfig) *PromptAgent {
	return &PromptAgent{
		name:         cfg.Name,
		description:  cfg.Description,
		systemPrompt: cfg.SystemPrompt,
		llmConfig:    cfg.LLMConfig,
		parseJSON:    cfg.ParseJSON,
	}
}

// Name returns the agent identifier
func (a *PromptAgent) Name() string { return a.name }

// Description returns the agent description
func (a *PromptAgent) Description() string { return a.description }

// PromptResult is the output from a PromptAgent
type PromptResult struct {
	AgentName  string
	Content    string         // Raw or formatted content
	Structured map[string]any // Parsed JSON if ParseJSON was true
	Duration   time.Duration
	Warnings   []string
}

// Run executes the agent with the given user prompt
func (a *PromptAgent) Run(ctx context.Context, userPrompt string) (*PromptResult, error) {
	start := time.Now()

	chatModel, err := llm.NewChatModel(ctx, a.llmConfig)
	if err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}

	messages := []*schema.Message{
		schema.SystemMessage(a.systemPrompt),
		schema.UserMessage(userPrompt),
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	result := &PromptResult{
		AgentName: a.name,
		Content:   resp.Content,
		Duration:  time.Since(start),
	}

	// Parse JSON if requested
	if a.parseJSON {
		structured, parseErr := parseJSONResponse(resp.Content)
		if parseErr != nil {
			result.Warnings = append(result.Warnings, "Failed to parse JSON: "+parseErr.Error())
		} else {
			result.Structured = structured
		}
	}

	return result, nil
}

// RunWithMessages allows full control over the message chain
func (a *PromptAgent) RunWithMessages(ctx context.Context, messages []*schema.Message) (*PromptResult, error) {
	start := time.Now()

	chatModel, err := llm.NewChatModel(ctx, a.llmConfig)
	if err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	result := &PromptResult{
		AgentName: a.name,
		Content:   resp.Content,
		Duration:  time.Since(start),
	}

	if a.parseJSON {
		structured, parseErr := parseJSONResponse(resp.Content)
		if parseErr != nil {
			result.Warnings = append(result.Warnings, "Failed to parse JSON: "+parseErr.Error())
		} else {
			result.Structured = structured
		}
	}

	return result, nil
}

// parseJSONResponse extracts JSON from LLM response (handles markdown code blocks)
func parseJSONResponse(response string) (map[string]any, error) {
	response = strings.TrimSpace(response)

	// Strip markdown code blocks
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result map[string]any
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, err
	}
	return result, nil
}
