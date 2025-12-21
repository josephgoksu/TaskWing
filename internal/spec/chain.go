package spec

import (
	"context"
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/agents"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// ChainConfig configures which personas to run
type ChainConfig struct {
	Skip    []PersonaType
	Verbose bool
}

// Preset returns a ChainConfig for common patterns
func Preset(name string) ChainConfig {
	switch name {
	case "feature":
		return ChainConfig{Skip: []PersonaType{PersonaMonetization}}
	case "bugfix":
		return ChainConfig{Skip: []PersonaType{PersonaPM, PersonaArchitect, PersonaMonetization, PersonaUX}}
	case "refactor":
		return ChainConfig{Skip: []PersonaType{PersonaPM, PersonaMonetization, PersonaUX, PersonaQA}}
	case "full":
		return ChainConfig{Skip: nil}
	default:
		return ChainConfig{Skip: []PersonaType{PersonaMonetization}}
	}
}

// Chain orchestrates persona agents for spec generation
type Chain struct {
	llmConfig llm.Config
	config    ChainConfig
	personas  []PersonaType
}

// NewChain creates a new persona chain
func NewChain(cfg llm.Config, config ChainConfig) *Chain {
	// Default persona order (Engineer always runs)
	allPersonas := []PersonaType{PersonaPM, PersonaArchitect, PersonaEngineer, PersonaQA, PersonaMonetization, PersonaUX}

	skipSet := make(map[PersonaType]bool)
	for _, s := range config.Skip {
		skipSet[s] = true
	}

	var personas []PersonaType
	for _, p := range allPersonas {
		if p == PersonaEngineer || !skipSet[p] {
			personas = append(personas, p)
		}
	}

	return &Chain{
		llmConfig: cfg,
		config:    config,
		personas:  personas,
	}
}

// ChainResult contains all outputs from the chain
type ChainResult struct {
	FeatureRequest string
	Outputs        []agents.PromptResult
	MarkdownSpec   string
}

// Execute runs the persona chain
func (c *Chain) Execute(ctx context.Context, featureRequest string) (*ChainResult, error) {
	result := &ChainResult{
		FeatureRequest: featureRequest,
		Outputs:        make([]agents.PromptResult, 0, len(c.personas)),
	}

	var previousContext strings.Builder
	previousContext.WriteString("## Feature Request\n\n")
	previousContext.WriteString(featureRequest)
	previousContext.WriteString("\n\n")

	for _, persona := range c.personas {
		if c.config.Verbose {
			fmt.Printf("🤖 Running %s agent...\n", persona)
		}

		// Create agent with persona's prompt
		agent := agents.NewPromptAgent(agents.PromptAgentConfig{
			Name:         string(persona),
			Description:  GetDescription(persona),
			SystemPrompt: GetPrompt(persona),
			LLMConfig:    c.llmConfig,
			ParseJSON:    true,
		})

		// Build user prompt with context from previous agents
		userPrompt := previousContext.String()
		userPrompt += "Based on the above, provide your analysis in JSON format."

		output, err := agent.Run(ctx, userPrompt)
		if err != nil {
			if c.config.Verbose {
				fmt.Printf("  ⚠️  %s agent error: %v\n", persona, err)
			}
			continue
		}

		if c.config.Verbose {
			fmt.Printf("  ✓ %s complete (%.1fs)\n", persona, output.Duration.Seconds())
		}

		result.Outputs = append(result.Outputs, *output)

		// Add to context for next agent
		previousContext.WriteString(fmt.Sprintf("## %s Analysis\n\n", toTitle(string(persona))))
		previousContext.WriteString(output.Content)
		previousContext.WriteString("\n\n")
	}

	// Generate combined markdown spec
	result.MarkdownSpec = c.generateMarkdownSpec(featureRequest, result.Outputs)

	return result, nil
}

func (c *Chain) generateMarkdownSpec(request string, outputs []agents.PromptResult) string {
	var sb strings.Builder

	sb.WriteString("# Feature Specification\n\n")
	sb.WriteString(fmt.Sprintf("**Request:** %s\n\n", request))
	sb.WriteString("---\n\n")

	for _, output := range outputs {
		sb.WriteString(fmt.Sprintf("# %s Analysis\n\n", toTitle(output.AgentName)))
		sb.WriteString(output.Content)
		sb.WriteString("\n---\n\n")
	}

	return sb.String()
}

func toTitle(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
