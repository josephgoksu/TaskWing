package bootstrap

import (
	"github.com/josephgoksu/TaskWing/internal/agents"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// NewDefaultAgents returns the standard set of agents for a bootstrap run.
func NewDefaultAgents(cfg llm.Config, projectPath string) []agents.Agent {
	return []agents.Agent{
		agents.NewDocAgent(cfg),
		agents.NewReactCodeAgent(cfg, projectPath),
		agents.NewGitAgent(cfg),
		agents.NewDepsAgent(cfg),
	}
}
