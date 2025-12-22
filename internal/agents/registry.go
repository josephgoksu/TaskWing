/*
Package agents provides the agent-driven architecture for codebase analysis.

Agents are specialized analyzers that focus on specific aspects of a project:
- DocAgent: Scans markdown documentation (README, ARCHITECTURE, etc.)
- CodeAgent: Analyzes source code structure and patterns
- GitAgent: Extracts insights from git history
- DepsAgent: Reads dependency files (package.json, go.mod, etc.)

Each agent implements the Agent interface and produces findings that are
converted to knowledge nodes with source attribution.

## Adding a New Agent

To add a new agent, simply:

 1. Create your agent file (e.g., `my_agent.go`)

 2. Embed BaseAgent for shared functionality

 3. Register with the factory in an init() function:

    func init() {
    RegisterAgentFactory("my_agent", func(cfg llm.Config) Agent {
    return NewMyAgent(cfg)
    })
    }

That's it! The registry will automatically include your agent.
*/
package agents

import (
	"sync"

	"github.com/josephgoksu/TaskWing/internal/llm"
)

// AgentFactory is a function that creates an Agent with the given LLM config.
type AgentFactory func(cfg llm.Config) Agent

// AgentInfo describes an agent for the registry.
type AgentInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

var (
	factories   = make(map[string]AgentFactory)
	factoriesMu sync.RWMutex
)

// RegisterAgentFactory registers a factory function for creating an agent.
// Call this in an init() function in your agent file.
func RegisterAgentFactory(id string, factory AgentFactory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[id] = factory
}

// CreateAgent creates an agent by ID using the registered factory.
// Returns nil if no factory is registered for the given ID.
func CreateAgent(id string, cfg llm.Config) Agent {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	if factory, ok := factories[id]; ok {
		return factory(cfg)
	}
	return nil
}

// CreateAllAgents creates all registered agents.
func CreateAllAgents(cfg llm.Config) []Agent {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()

	agents := make([]Agent, 0, len(factories))
	for _, factory := range factories {
		agents = append(agents, factory(cfg))
	}
	return agents
}

// Registry returns metadata for all registered agents.
// This is used by the API to expose agent information.
func Registry() []AgentInfo {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()

	infos := make([]AgentInfo, 0, len(factories))
	for id, factory := range factories {
		// Create a temporary agent to get its name and description
		agent := factory(llm.Config{})
		infos = append(infos, AgentInfo{
			ID:          id,
			Name:        agent.Name(),
			Description: agent.Description(),
		})
	}
	return infos
}

// GetAgentByID returns agent info by ID, or nil if not found.
func GetAgentByID(id string) *AgentInfo {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()

	if factory, ok := factories[id]; ok {
		agent := factory(llm.Config{})
		return &AgentInfo{
			ID:          id,
			Name:        agent.Name(),
			Description: agent.Description(),
		}
	}
	return nil
}

// init registers the built-in agents.
// Each agent could also self-register in its own file's init(), but this
// centralizes registration for the core agents.
func init() {
	RegisterAgentFactory("doc", func(cfg llm.Config) Agent {
		return NewDocAgent(cfg)
	})
	RegisterAgentFactory("git", func(cfg llm.Config) Agent {
		return NewGitAgent(cfg)
	})
	RegisterAgentFactory("deps", func(cfg llm.Config) Agent {
		return NewDepsAgent(cfg)
	})
	RegisterAgentFactory("react_code", func(cfg llm.Config) Agent {
		return NewReactCodeAgent(cfg, "")
	})
}
