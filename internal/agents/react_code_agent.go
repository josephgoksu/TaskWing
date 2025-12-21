/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package agents provides the ReAct CodeAgent that uses dynamic tool exploration
to analyze codebases, rather than static hardcoded context gathering.
*/
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// ReactCodeAgent uses ReAct pattern to dynamically explore and analyze codebases
type ReactCodeAgent struct {
	llmConfig    llm.Config
	basePath     string
	maxIters     int // Maximum ReAct iterations before forcing output
	verbose      bool
	modelFactory func(context.Context, llm.Config) (model.BaseChatModel, error)
}

// NewReactCodeAgent creates a new ReAct-powered code analysis agent
func NewReactCodeAgent(cfg llm.Config, basePath string) *ReactCodeAgent {
	return &ReactCodeAgent{
		llmConfig:    cfg,
		basePath:     basePath,
		maxIters:     10, // Reasonable limit to prevent infinite loops
		verbose:      false,
		modelFactory: llm.NewChatModel,
	}
}

// SetVerbose enables detailed logging of agent actions
func (a *ReactCodeAgent) SetVerbose(v bool) {
	a.verbose = v
}

// SetMaxIterations sets the maximum number of tool-use iterations
func (a *ReactCodeAgent) SetMaxIterations(n int) {
	if n > 0 && n <= 20 {
		a.maxIters = n
	}
}

func (a *ReactCodeAgent) Name() string { return "react_code" }
func (a *ReactCodeAgent) Description() string {
	return "Dynamically explores codebase using tools to identify architectural patterns"
}

// systemPrompt provides the ReAct agent with exploration instructions
const reactCodeAgentSystemPrompt = `You are an expert software architect analyzing a codebase to identify architectural patterns and key decisions.

## Your Mission
Discover and document the key architectural decisions, technology choices, and patterns in this codebase.

## Available Tools
- **list_dir**: Explore directory structure to understand project organization
- **read_file**: Read file contents to examine implementations
- **grep_search**: Search for patterns across the codebase
- **exec_command**: Run git/find commands for history and file discovery

## Exploration Strategy
1. START by listing the root directory to understand project structure
2. Read key files: README.md, package.json/go.mod, main entry points
3. Search for patterns: "func main", "import", configuration files
4. Dig deeper into interesting directories (internal/, src/, lib/)
5. When you have enough context, provide your analysis

## Output Format
When you have gathered sufficient information, respond with a JSON analysis:

` + "```json" + `
{
  "decisions": [
    {
      "title": "Short decision title",
      "what": "What technology/pattern was chosen",
      "why": "Why this choice was made (inferred from evidence)",
      "tradeoffs": "What tradeoffs this implies",
      "confidence": "high|medium|low",
      "evidence": ["file1.go:123", "README.md"]
    }
  ]
}
` + "```" + `

## Rules
- Call tools to gather information before making conclusions
- Don't guess - use tools to verify assumptions
- Focus on DECISIONS not just observations
- Explain WHY choices were made, not just WHAT they are
- Stop when you have 5-10 solid findings with evidence`

// Run executes the ReAct agent with tool-calling loop
func (a *ReactCodeAgent) Run(ctx context.Context, input Input) (Output, error) {
	var output Output
	output.AgentName = a.Name()
	start := time.Now()

	// Create chat model
	chatModel, err := a.modelFactory(ctx, a.llmConfig)
	if err != nil {
		return output, fmt.Errorf("create chat model: %w", err)
	}

	// Create Eino tools
	tools := CreateEinoTools(a.basePath)

	// Convert to BaseTool slice for ToolsNodeConfig
	baseTools := make([]tool.BaseTool, len(tools))
	for i, t := range tools {
		baseTools[i] = t
	}

	// Create ToolsNode for executing tool calls
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: baseTools,
	})
	if err != nil {
		return output, fmt.Errorf("create tools node: %w", err)
	}

	// Build tool infos for model binding
	toolInfos := make([]*schema.ToolInfo, 0, len(tools))
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		toolInfos = append(toolInfos, info)
	}

	// Initialize conversation
	messages := []*schema.Message{
		schema.SystemMessage(reactCodeAgentSystemPrompt),
		schema.UserMessage(fmt.Sprintf(
			"Analyze the architectural patterns and key decisions in project: %s\n\nStart by exploring the directory structure.",
			input.ProjectName,
		)),
	}

	// ReAct loop: LLM -> (tool call -> tool result -> LLM)* -> final answer
	for iter := 0; iter < a.maxIters; iter++ {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return output, ctx.Err()
		default:
		}

		if a.verbose {
			fmt.Printf("  [ReAct iter %d] Calling LLM...\n", iter+1)
		}

		// Call LLM with tool bindings
		resp, err := chatModel.Generate(ctx, messages, model.WithTools(toolInfos))
		if err != nil {
			return output, fmt.Errorf("generate (iter %d): %w", iter+1, err)
		}

		// Add assistant response to history
		messages = append(messages, resp)

		// Check if LLM wants to call tools
		if len(resp.ToolCalls) == 0 {
			// No tool calls = final answer
			if a.verbose {
				fmt.Printf("  [ReAct] Final answer after %d iterations\n", iter+1)
			}
			output.RawOutput = resp.Content
			break
		}

		// Execute tool calls
		if a.verbose {
			for _, tc := range resp.ToolCalls {
				fmt.Printf("  [ReAct] Tool call: %s(%s)\n", tc.Function.Name, truncate(tc.Function.Arguments, 50))
			}
		}

		toolResults, err := toolsNode.Invoke(ctx, resp)
		if err != nil {
			// Don't fail on tool errors - let LLM know and continue
			toolResults = []*schema.Message{
				schema.ToolMessage(fmt.Sprintf("Error executing tools: %v", err), "error"),
			}
		}

		// Add tool results to conversation
		messages = append(messages, toolResults...)

		if a.verbose {
			for _, tr := range toolResults {
				fmt.Printf("  [ReAct] Tool result: %s\n", truncate(tr.Content, 100))
			}
		}
	}

	// Warn if we hit max iterations without a final answer
	if output.RawOutput == "" {
		if a.verbose {
			fmt.Printf("  [ReAct] Warning: max iterations (%d) reached without final answer\n", a.maxIters)
		}
		// Try to extract any useful content from last message
		if len(messages) > 0 {
			lastMsg := messages[len(messages)-1]
			if lastMsg.Content != "" {
				output.RawOutput = lastMsg.Content
			}
		}
	}

	// Parse findings from final output
	if output.RawOutput != "" {
		findings, err := a.parseFindings(output.RawOutput)
		if err != nil && a.verbose {
			fmt.Printf("  [ReAct] Parse warning: %v\n", err)
		}
		output.Findings = findings
	}

	output.Duration = time.Since(start)
	return output, nil
}

// parseFindings extracts structured findings from the LLM response
func (a *ReactCodeAgent) parseFindings(response string) ([]Finding, error) {
	// Try to extract JSON from response
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var parsed struct {
		Decisions []struct {
			Title      string   `json:"title"`
			What       string   `json:"what"`
			Why        string   `json:"why"`
			Tradeoffs  string   `json:"tradeoffs"`
			Confidence string   `json:"confidence"`
			Evidence   []string `json:"evidence"`
		} `json:"decisions"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	var findings []Finding
	for _, d := range parsed.Decisions {
		findings = append(findings, Finding{
			Type:        FindingTypeDecision,
			Title:       d.Title,
			Description: d.What,
			Why:         d.Why,
			Tradeoffs:   d.Tradeoffs,
			Confidence:  d.Confidence,
			SourceFiles: d.Evidence,
			SourceAgent: a.Name(),
		})
	}

	return findings, nil
}

// truncate shortens a string for logging
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
