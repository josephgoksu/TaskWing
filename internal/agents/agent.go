/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package agents provides a framework for specialized LLM-powered agents
that can analyze different aspects of a codebase using tool calling.
*/
package agents

import (
	"context"
	"time"
)

// Agent is the interface all specialized agents must implement
type Agent interface {
	// Name returns the unique identifier for this agent
	Name() string

	// Description returns a human-readable description of what this agent does
	Description() string

	// Run executes the agent's analysis and returns findings
	Run(ctx context.Context, input Input) (Output, error)
}

// Input provides the context and configuration for an agent run
type Input struct {
	// BasePath is the root directory of the project being analyzed
	BasePath string

	// ProjectName is the name of the project (derived from directory)
	ProjectName string

	// ExistingContext is any context gathered by previous agents
	ExistingContext map[string]any

	// MaxTokens limits the output size
	MaxTokens int

	// Verbose enables detailed logging
	Verbose bool
}

// Output captures the results of an agent's analysis
type Output struct {
	// AgentName identifies which agent produced this output
	AgentName string

	// Findings are the structured discoveries from analysis
	Findings []Finding

	// RawOutput is the unprocessed LLM response (for debugging)
	RawOutput string

	// TokensUsed tracks token consumption
	TokensUsed int

	// Duration is how long the agent took to run
	Duration time.Duration

	// Error captures any non-fatal issues encountered
	Error error
}

// Finding represents a single discovery made by an agent
type Finding struct {
	// Type categorizes the finding (feature, decision, dependency, pattern, etc.)
	Type FindingType

	// Title is the short summary
	Title string

	// Description is the detailed explanation
	Description string

	// Why explains the reasoning or purpose (for decisions)
	Why string

	// Tradeoffs lists accepted compromises (for decisions)
	Tradeoffs string

	// Confidence indicates how certain the agent is (high, medium, low)
	Confidence string

	// SourceFiles lists files that informed this finding
	SourceFiles []string

	// Metadata holds agent-specific additional data
	Metadata map[string]any
}

// FindingType categorizes what kind of discovery was made
type FindingType string

const (
	FindingTypeFeature    FindingType = "feature"
	FindingTypeDecision   FindingType = "decision"
	FindingTypeDependency FindingType = "dependency"
	FindingTypePattern    FindingType = "pattern"
	FindingTypeRisk       FindingType = "risk"
	FindingTypeTodo       FindingType = "todo"
)

// Tool represents a capability an agent can invoke
type Tool interface {
	// Name returns the tool identifier
	Name() string

	// Description returns what the tool does (for LLM)
	Description() string

	// Execute runs the tool with given arguments
	Execute(ctx context.Context, args map[string]any) (any, error)
}
