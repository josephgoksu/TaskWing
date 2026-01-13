package bootstrap

import (
	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// NewDefaultAgents returns the standard set of agents for a bootstrap run.
func NewDefaultAgents(cfg llm.Config, projectPath string) []core.Agent {
	return []core.Agent{
		impl.NewDocAgent(cfg),
		impl.NewCodeAgent(cfg, projectPath),
		impl.NewGitAgent(cfg),
		impl.NewDepsAgent(cfg),
	}
}
