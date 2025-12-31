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
