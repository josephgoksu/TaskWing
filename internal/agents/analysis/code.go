/*
Package analysis provides the ReAct code agent for dynamic codebase exploration.
*/
package analysis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	agenttools "github.com/josephgoksu/TaskWing/internal/agents/tools"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// ReactAgent uses Eino's built-in ReAct agent for dynamic codebase exploration.
type ReactAgent struct {
	core.BaseAgent
	basePath string
	maxSteps int
	verbose  bool
}

// NewReactAgent creates a new ReAct-powered code analysis agent.
func NewReactAgent(cfg llm.Config, basePath string) *ReactAgent {
	return &ReactAgent{
		BaseAgent: core.NewBaseAgent("react", "Dynamically explores codebase using tools to identify architectural patterns", cfg),
		basePath:  basePath,
		maxSteps:  20,
		verbose:   false,
	}
}

// SetVerbose enables detailed logging of agent actions.
func (a *ReactAgent) SetVerbose(v bool) { a.verbose = v }

// SetMaxIterations sets the maximum number of ReAct steps.
func (a *ReactAgent) SetMaxIterations(n int) {
	if n > 0 && n <= 80 {
		a.maxSteps = n
	}
}

// Run executes the agent using Eino's built-in react.Agent.
func (a *ReactAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	var output core.Output
	output.AgentName = a.Name()
	start := time.Now()

	closeableChatModel, err := llm.NewCloseableChatModel(ctx, a.LLMConfig())
	if err != nil {
		return output, fmt.Errorf("create chat model: %w", err)
	}
	defer func() { _ = closeableChatModel.Close() }()

	baseChatModel := closeableChatModel.BaseChatModel
	toolCallingModel, ok := baseChatModel.(model.ToolCallingChatModel)
	if !ok {
		if a.verbose {
			fmt.Printf("  [ReAct] Model doesn't support tool calling, using fallback\n")
		}
		return a.runSimpleFallback(ctx, baseChatModel, input)
	}

	basePath := input.BasePath
	if basePath == "" {
		basePath = a.basePath
	}

	einoTools := agenttools.CreateEinoTools(basePath)
	baseTools := make([]tool.BaseTool, len(einoTools))
	for i, t := range einoTools {
		baseTools[i] = t
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: toolCallingModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: baseTools},
		MaxStep:          a.maxSteps,
		MessageModifier: func(ctx context.Context, msgs []*schema.Message) []*schema.Message {
			return append([]*schema.Message{schema.SystemMessage(config.SystemPromptReactAgent)}, msgs...)
		},
	})
	if err != nil {
		if a.verbose {
			fmt.Printf("  [ReAct] Agent creation failed, falling back: %v\n", err)
		}
		return a.runSimpleFallback(ctx, baseChatModel, input)
	}

	if a.verbose {
		handler := callbacks.NewHandlerBuilder().Build()
		runInfo := &callbacks.RunInfo{Name: a.Name(), Type: "ReActAgent"}
		ctx = callbacks.InitCallbacks(ctx, runInfo, handler)
	}

	userMsg := []*schema.Message{
		schema.UserMessage(fmt.Sprintf(
			"Analyze the architectural patterns and key decisions in project: %s\n\nStart by exploring the directory structure.",
			input.ProjectName,
		)),
	}

	resp, err := agent.Generate(ctx, userMsg)
	if err != nil {
		if a.verbose {
			fmt.Printf("  [ReAct] Generate failed, using fallback: %v\n", err)
		}
		return a.runSimpleFallback(ctx, baseChatModel, input)
	}

	output.RawOutput = resp.Content
	output.Duration = time.Since(start)

	if output.RawOutput != "" {
		findings, err := a.parseFindings(output.RawOutput)
		if err != nil && a.verbose {
			fmt.Printf("  [ReAct] Parse warning: %v\n", err)
		}
		output.Findings = findings
	}

	return output, nil
}

type reactParseResult struct {
	Decisions []struct {
		Title      string              `json:"title"`
		Component  string              `json:"component"`
		What       string              `json:"what"`
		Why        string              `json:"why"`
		Tradeoffs  string              `json:"tradeoffs"`
		Confidence any                 `json:"confidence"`
		Evidence   []core.EvidenceJSON `json:"evidence"`
	} `json:"decisions"`
	Patterns []struct {
		Name         string              `json:"name"`
		Context      string              `json:"context"`
		Solution     string              `json:"solution"`
		Consequences string              `json:"consequences"`
		Confidence   any                 `json:"confidence"`
		Evidence     []core.EvidenceJSON `json:"evidence"`
	} `json:"patterns"`
}

func (a *ReactAgent) parseFindings(response string) ([]core.Finding, error) {
	parsed, err := core.ParseJSONResponse[reactParseResult](response)
	if err != nil {
		return nil, err
	}

	var findings []core.Finding
	for _, d := range parsed.Decisions {
		findings = append(findings, core.NewFindingWithEvidence(
			core.FindingTypeDecision,
			d.Title, d.What, d.Why, d.Tradeoffs,
			d.Confidence, d.Evidence, a.Name(),
			map[string]any{"component": d.Component},
		))
	}

	for _, p := range parsed.Patterns {
		findings = append(findings, core.NewFindingWithEvidence(
			core.FindingTypePattern,
			p.Name, p.Context, "", p.Consequences,
			p.Confidence, p.Evidence, a.Name(),
			map[string]any{"context": p.Context, "solution": p.Solution, "consequences": p.Consequences},
		))
	}

	return findings, nil
}

func (a *ReactAgent) runSimpleFallback(ctx context.Context, chatModel model.BaseChatModel, input core.Input) (core.Output, error) {
	var output core.Output
	output.AgentName = a.Name()
	start := time.Now()

	basePath := input.BasePath
	if basePath == "" {
		basePath = a.basePath
	}

	gatherer := agenttools.NewContextGatherer(basePath)
	var contextBuilder strings.Builder
	contextBuilder.WriteString("## Directory Structure\n")
	contextBuilder.WriteString(gatherer.ListDirectoryTree(5))
	contextBuilder.WriteString("\n\n")
	contextBuilder.WriteString(gatherer.GatherKeyFiles())

	simplePrompt := fmt.Sprintf(`You are an expert software architect. Analyze this codebase context and extract architectural patterns and decisions.

PROJECT: %s

CONTEXT:
%s

CRITICAL: Every finding MUST include evidence with file_path, line numbers, and snippet.
Confidence must be a NUMBER between 0.0 and 1.0.

Respond with JSON only:
`+"```json"+`
{
  "decisions": [{"title": "...", "component": "...", "what": "...", "why": "...", "tradeoffs": "...", "confidence": 0.85, "evidence": [{"file_path": "...", "start_line": 10, "end_line": 20, "snippet": "..."}]}],
  "patterns": [{"name": "...", "context": "...", "solution": "...", "consequences": "...", "confidence": 0.75, "evidence": [...]}]
}
`+"```", input.ProjectName, contextBuilder.String())

	resp, err := chatModel.Generate(ctx, []*schema.Message{schema.UserMessage(simplePrompt)})
	if err != nil {
		return output, fmt.Errorf("simple fallback generate: %w", err)
	}

	output.RawOutput = resp.Content
	output.Duration = time.Since(start)

	if output.RawOutput != "" {
		findings, _ := a.parseFindings(output.RawOutput)
		output.Findings = findings
	}

	return output, nil
}

func init() {
	core.RegisterAgent("react", func(cfg llm.Config, basePath string) core.Agent {
		return NewReactAgent(cfg, basePath)
	}, "ReAct Explorer", "Dynamically explores codebase using tools to identify architectural patterns")
}
