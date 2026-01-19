/*
Package impl provides utility agents for code simplification, explanation, and debugging.
*/
package impl

import (
	"context"
	"fmt"
	"io"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// =============================================================================
// SimplifyAgent
// =============================================================================

// SimplifyAgent reduces code complexity and line count.
// Call Close() when done to release resources.
type SimplifyAgent struct {
	core.BaseAgent
	chain       *core.DeterministicChain[SimplifyOutput]
	modelCloser io.Closer
}

// SimplifyChange describes a single simplification made.
type SimplifyChange struct {
	What string `json:"what"`
	Why  string `json:"why"`
	Risk string `json:"risk"`
}

// SimplifyOutput defines the structured response from the LLM.
type SimplifyOutput struct {
	SimplifiedCode      string           `json:"simplified_code"`
	OriginalLines       int              `json:"original_lines"`
	SimplifiedLines     int              `json:"simplified_lines"`
	ReductionPercentage int              `json:"reduction_percentage"`
	Changes             []SimplifyChange `json:"changes"`
	RiskAssessment      string           `json:"risk_assessment"`
}

// NewSimplifyAgent creates a new agent for code simplification.
func NewSimplifyAgent(cfg llm.Config) *SimplifyAgent {
	return &SimplifyAgent{
		BaseAgent: core.NewBaseAgent("simplify", "Reduces code complexity and line count", cfg),
	}
}

// Close releases LLM resources. Safe to call multiple times.
func (a *SimplifyAgent) Close() error {
	if a.modelCloser != nil {
		return a.modelCloser.Close()
	}
	return nil
}

// Run executes the simplification using Eino Chain.
func (a *SimplifyAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	if a.chain == nil {
		chatModel, err := a.CreateCloseableChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		a.modelCloser = chatModel
		chain, err := core.NewDeterministicChain[SimplifyOutput](
			ctx,
			a.Name(),
			chatModel.BaseChatModel,
			config.SystemPromptSimplifyAgent,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	code, ok := input.ExistingContext["code"].(string)
	if !ok || code == "" {
		return core.Output{}, fmt.Errorf("missing 'code' in input context")
	}

	filePath, _ := input.ExistingContext["file_path"].(string)
	kgContext, _ := input.ExistingContext["context"].(string)

	chainInput := map[string]any{
		"Code":     code,
		"FilePath": filePath,
		"Context":  kgContext,
	}

	parsed, raw, duration, err := a.chain.Invoke(ctx, chainInput)
	if err != nil {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("chain invoke: %w", err),
			Duration:  duration,
			RawOutput: raw,
		}, nil
	}

	return core.BuildOutput(
		a.Name(),
		[]core.Finding{{
			Type:        "simplification",
			Title:       "Code Simplification",
			Description: fmt.Sprintf("Reduced from %d to %d lines (%d%% reduction)", parsed.OriginalLines, parsed.SimplifiedLines, parsed.ReductionPercentage),
			Metadata: map[string]any{
				"simplified_code":      parsed.SimplifiedCode,
				"original_lines":       parsed.OriginalLines,
				"simplified_lines":     parsed.SimplifiedLines,
				"reduction_percentage": parsed.ReductionPercentage,
				"changes":              parsed.Changes,
				"risk_assessment":      parsed.RiskAssessment,
			},
		}},
		"JSON handled by Eino",
		duration,
	), nil
}

// =============================================================================
// ExplainAgent
// =============================================================================

// ExplainAgent provides deep-dive explanations of code and concepts.
// Call Close() when done to release resources.
type ExplainAgent struct {
	core.BaseAgent
	chain       *core.DeterministicChain[ExplainOutput]
	modelCloser io.Closer
}

// ExplainConnection describes a relationship to another component.
type ExplainConnection struct {
	Target       string `json:"target"`
	Relationship string `json:"relationship"`
	Description  string `json:"description"`
}

// ExplainExample provides a usage example.
type ExplainExample struct {
	Description string `json:"description"`
	Code        string `json:"code"`
}

// ExplainOutput defines the structured response from the LLM.
type ExplainOutput struct {
	Summary     string              `json:"summary"`
	Explanation string              `json:"explanation"`
	Connections []ExplainConnection `json:"connections"`
	Pitfalls    []string            `json:"pitfalls"`
	Examples    []ExplainExample    `json:"examples"`
}

// NewExplainAgent creates a new agent for code explanation.
func NewExplainAgent(cfg llm.Config) *ExplainAgent {
	return &ExplainAgent{
		BaseAgent: core.NewBaseAgent("explain", "Provides deep-dive explanations of code and concepts", cfg),
	}
}

// Close releases LLM resources. Safe to call multiple times.
func (a *ExplainAgent) Close() error {
	if a.modelCloser != nil {
		return a.modelCloser.Close()
	}
	return nil
}

// Run executes the explanation using Eino Chain.
func (a *ExplainAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	if a.chain == nil {
		chatModel, err := a.CreateCloseableChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		a.modelCloser = chatModel
		chain, err := core.NewDeterministicChain[ExplainOutput](
			ctx,
			a.Name(),
			chatModel.BaseChatModel,
			config.SystemPromptExplainAgent,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	query, ok := input.ExistingContext["query"].(string)
	if !ok || query == "" {
		return core.Output{}, fmt.Errorf("missing 'query' in input context")
	}

	symbol, _ := input.ExistingContext["symbol"].(string)
	code, _ := input.ExistingContext["code"].(string)
	kgContext, _ := input.ExistingContext["context"].(string)

	chainInput := map[string]any{
		"Query":   query,
		"Symbol":  symbol,
		"Code":    code,
		"Context": kgContext,
	}

	parsed, raw, duration, err := a.chain.Invoke(ctx, chainInput)
	if err != nil {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("chain invoke: %w", err),
			Duration:  duration,
			RawOutput: raw,
		}, nil
	}

	return core.BuildOutput(
		a.Name(),
		[]core.Finding{{
			Type:        "explanation",
			Title:       parsed.Summary,
			Description: parsed.Explanation,
			Metadata: map[string]any{
				"connections": parsed.Connections,
				"pitfalls":    parsed.Pitfalls,
				"examples":    parsed.Examples,
			},
		}},
		"JSON handled by Eino",
		duration,
	), nil
}

// =============================================================================
// DebugAgent
// =============================================================================

// DebugAgent helps developers diagnose issues systematically.
// Call Close() when done to release resources.
type DebugAgent struct {
	core.BaseAgent
	chain       *core.DeterministicChain[DebugOutput]
	modelCloser io.Closer
}

// DebugHypothesis represents a possible cause of the issue.
type DebugHypothesis struct {
	Cause         string   `json:"cause"`
	Likelihood    string   `json:"likelihood"`
	Reasoning     string   `json:"reasoning"`
	CodeLocations []string `json:"code_locations"`
}

// DebugInvestigationStep is a step to investigate the issue.
type DebugInvestigationStep struct {
	Step            int    `json:"step"`
	Action          string `json:"action"`
	Command         string `json:"command"`
	ExpectedFinding string `json:"expected_finding"`
}

// DebugQuickFix is a quick fix suggestion.
type DebugQuickFix struct {
	Fix  string `json:"fix"`
	When string `json:"when"`
}

// DebugOutput defines the structured response from the LLM.
type DebugOutput struct {
	Hypotheses         []DebugHypothesis        `json:"hypotheses"`
	InvestigationSteps []DebugInvestigationStep `json:"investigation_steps"`
	QuickFixes         []DebugQuickFix          `json:"quick_fixes"`
}

// NewDebugAgent creates a new agent for issue debugging.
func NewDebugAgent(cfg llm.Config) *DebugAgent {
	return &DebugAgent{
		BaseAgent: core.NewBaseAgent("debug", "Helps diagnose issues systematically", cfg),
	}
}

// Close releases LLM resources. Safe to call multiple times.
func (a *DebugAgent) Close() error {
	if a.modelCloser != nil {
		return a.modelCloser.Close()
	}
	return nil
}

// Run executes the debugging analysis using Eino Chain.
func (a *DebugAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	if a.chain == nil {
		chatModel, err := a.CreateCloseableChatModel(ctx)
		if err != nil {
			return core.Output{}, err
		}
		a.modelCloser = chatModel
		chain, err := core.NewDeterministicChain[DebugOutput](
			ctx,
			a.Name(),
			chatModel.BaseChatModel,
			config.SystemPromptDebugAgent,
		)
		if err != nil {
			return core.Output{}, fmt.Errorf("create chain: %w", err)
		}
		a.chain = chain
	}

	problem, ok := input.ExistingContext["problem"].(string)
	if !ok || problem == "" {
		return core.Output{}, fmt.Errorf("missing 'problem' in input context")
	}

	errorMsg, _ := input.ExistingContext["error"].(string)
	stackTrace, _ := input.ExistingContext["stack_trace"].(string)
	kgContext, _ := input.ExistingContext["context"].(string)

	chainInput := map[string]any{
		"Problem":    problem,
		"Error":      errorMsg,
		"StackTrace": stackTrace,
		"Context":    kgContext,
	}

	parsed, raw, duration, err := a.chain.Invoke(ctx, chainInput)
	if err != nil {
		return core.Output{
			AgentName: a.Name(),
			Error:     fmt.Errorf("chain invoke: %w", err),
			Duration:  duration,
			RawOutput: raw,
		}, nil
	}

	// Build summary from hypotheses
	summary := "Unknown issue"
	if len(parsed.Hypotheses) > 0 {
		summary = parsed.Hypotheses[0].Cause
	}

	return core.BuildOutput(
		a.Name(),
		[]core.Finding{{
			Type:        "debug",
			Title:       "Debug Analysis",
			Description: summary,
			Metadata: map[string]any{
				"hypotheses":          parsed.Hypotheses,
				"investigation_steps": parsed.InvestigationSteps,
				"quick_fixes":         parsed.QuickFixes,
			},
		}},
		"JSON handled by Eino",
		duration,
	), nil
}

// =============================================================================
// Agent Registration
// =============================================================================

func init() {
	core.RegisterAgent("simplify", func(cfg llm.Config, basePath string) core.Agent {
		return NewSimplifyAgent(cfg)
	}, "Code Simplification", "Reduces code complexity and line count")

	core.RegisterAgent("explain", func(cfg llm.Config, basePath string) core.Agent {
		return NewExplainAgent(cfg)
	}, "Code Explanation", "Provides deep-dive explanations of code and concepts")

	core.RegisterAgent("debug", func(cfg llm.Config, basePath string) core.Agent {
		return NewDebugAgent(cfg)
	}, "Debug Helper", "Helps diagnose issues systematically")
}
