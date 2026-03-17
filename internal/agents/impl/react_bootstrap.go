/*
Package impl provides the shared ReAct helper for bootstrap agents.

runReactMode wraps Eino's react.Agent wiring so that any bootstrap agent
(doc, deps, git) can attempt tool-calling exploration before falling back
to its deterministic path.
*/
package impl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	agenttools "github.com/josephgoksu/TaskWing/internal/agents/tools"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// ErrNoToolCalling indicates the configured model does not support tool calling.
// Callers should fall back to the deterministic analysis path.
var ErrNoToolCalling = errors.New("model does not support tool calling")

// Minimum findings thresholds for ReAct to be considered successful.
// If ReAct produces fewer findings than these thresholds, we fall through
// to the deterministic path which is more thorough for small/focused repos.
const (
	reactMinFindingsDoc  = 5 // Doc deterministic produces 10-20 via parallel tracks
	reactMinFindingsDeps = 3 // Deps deterministic produces 5-10
	reactMinFindingsGit  = 3 // Git deterministic produces 3-8 via chunked analysis
)

// runReactMode runs a ReAct agent with the given system prompt and user message.
// It creates a short-lived chat model, wires up Eino tools, and returns the
// raw text output from the agent. If the model lacks tool-calling support,
// ErrNoToolCalling is returned so callers can fall through to deterministic mode.
func runReactMode(ctx context.Context, cfg llm.Config, basePath, systemPrompt, userMsg string, maxSteps int) (string, time.Duration, error) {
	start := time.Now()

	closeableChatModel, err := llm.NewCloseableChatModel(ctx, cfg)
	if err != nil {
		return "", 0, fmt.Errorf("create chat model: %w", err)
	}
	defer func() { _ = closeableChatModel.Close() }()

	toolCallingModel, ok := closeableChatModel.BaseChatModel.(model.ToolCallingChatModel)
	if !ok {
		return "", 0, ErrNoToolCalling
	}

	einoTools := agenttools.CreateEinoTools(basePath)
	baseTools := make([]tool.BaseTool, len(einoTools))
	for i, t := range einoTools {
		baseTools[i] = t
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: toolCallingModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: baseTools},
		MaxStep:          maxSteps,
		MessageModifier: func(_ context.Context, msgs []*schema.Message) []*schema.Message {
			return append([]*schema.Message{schema.SystemMessage(systemPrompt)}, msgs...)
		},
	})
	if err != nil {
		return "", time.Since(start), fmt.Errorf("create ReAct agent: %w", err)
	}

	resp, err := agent.Generate(ctx, []*schema.Message{schema.UserMessage(userMsg)})
	if err != nil {
		return "", time.Since(start), fmt.Errorf("agent generate: %w", err)
	}

	return resp.Content, time.Since(start), nil
}
