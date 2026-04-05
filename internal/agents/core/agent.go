/*
Package core provides the foundational types and interfaces for TaskWing agents.
*/
package core

import (
	"context"
	"time"
)

// Agent is the interface all specialized agents must implement.
type Agent interface {
	Name() string
	Description() string
	Run(ctx context.Context, input Input) (Output, error)
}

// CloseableAgent is an agent that holds resources requiring cleanup.
// Agents with LLM connections should implement this interface.
type CloseableAgent interface {
	Agent
	Close() error
}

// BatchableAgent is an agent that can render its deterministic prompt for batch submission.
// Agents implement this to support the OpenAI Batch API path (50% cost reduction).
// PrepareForBatch gathers context and renders the prompt without calling the LLM.
// ParseBatchResult takes raw LLM response text and converts it to findings.
type BatchableAgent interface {
	Agent
	PrepareForBatch(ctx context.Context, input Input) (messages []BatchMessage, err error)
	ParseBatchResult(raw string) (Output, error)
}

// BatchMessage represents a chat message for batch submission.
type BatchMessage struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// CloseAgents closes all agents that implement CloseableAgent.
// Safe to call on agents that don't implement CloseableAgent.
func CloseAgents(agents []Agent) {
	for _, a := range agents {
		if c, ok := a.(CloseableAgent); ok {
			_ = c.Close()
		}
	}
}

// AgentMode determines how an agent should behave.
type AgentMode string

const (
	ModeBootstrap AgentMode = "bootstrap" // Full analysis on entire project
	ModeWatch     AgentMode = "watch"     // Incremental analysis on changed files
)

// Input provides the context and configuration for an agent run.
type Input struct {
	BasePath        string
	ProjectName     string
	Mode            AgentMode
	ChangedFiles    []string       // Only used in ModeWatch
	ExistingContext map[string]any // Context from previous agents
	MaxTokens       int
	Verbose         bool
	Workspace       string // Workspace/service name for monorepo support ('root' for global, service name for scoped)
}

// Output captures the results of an agent's analysis.
type Output struct {
	AgentName     string
	Findings      []Finding
	Relationships []Relationship // LLM-extracted relationships between findings
	RawOutput     string
	TokensUsed    int
	Duration      time.Duration
	Error         error
	Coverage      CoverageStats // Files analyzed/skipped by this agent
}

// Relationship represents an LLM-identified relationship between two findings.
type Relationship struct {
	From     string // Name of source finding
	To       string // Name of target finding
	Relation string // depends_on, affects, extends
	Reason   string // Why they are related
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

// BuildOutputWithRelationships creates an Output with findings and relationships.
func BuildOutputWithRelationships(agentName string, findings []Finding, relationships []Relationship, rawOutput string, duration time.Duration) Output {
	return Output{
		AgentName:     agentName,
		Findings:      findings,
		Relationships: relationships,
		RawOutput:     rawOutput,
		Duration:      duration,
	}
}
