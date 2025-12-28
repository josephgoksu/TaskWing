package bootstrap

import (
	"github.com/josephgoksu/TaskWing/internal/agents/analysis"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// NewDefaultAgents returns the standard set of agents for a bootstrap run.
func NewDefaultAgents(cfg llm.Config, projectPath string) []core.Agent {
	return []core.Agent{
		analysis.NewDocAgent(cfg),
		analysis.NewCodeAgent(cfg, projectPath),
		analysis.NewGitAgent(cfg),
		analysis.NewDepsAgent(cfg),
	}
}
