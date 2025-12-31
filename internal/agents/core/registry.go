/*
Package core provides the agent factory registry.
*/
package core

import (
	"sync"

	"github.com/josephgoksu/TaskWing/internal/llm"
)

// AgentFactory creates an Agent with the given LLM config.
type AgentFactory func(cfg llm.Config, basePath string) Agent

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
func RegisterAgentFactory(id string, factory AgentFactory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[id] = factory
}

// CreateAgent creates an agent by ID using the registered factory.
func CreateAgent(id string, cfg llm.Config, basePath string) Agent {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	if factory, ok := factories[id]; ok {
		return factory(cfg, basePath)
	}
	return nil
}

// CreateAllAgents creates all registered agents.
func CreateAllAgents(cfg llm.Config, basePath string) []Agent {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	agents := make([]Agent, 0, len(factories))
	for _, factory := range factories {
		agents = append(agents, factory(cfg, basePath))
	}
	return agents
}

// Registry returns metadata for all registered agents.
func Registry() []AgentInfo {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	infos := make([]AgentInfo, 0, len(factories))
	for id, factory := range factories {
		agent := factory(llm.Config{}, "")
		infos = append(infos, AgentInfo{
			ID:          id,
			Name:        agent.Name(),
			Description: agent.Description(),
		})
	}
	return infos
}

// GetAgentByID returns agent info by ID.
func GetAgentByID(id string) *AgentInfo {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	if factory, ok := factories[id]; ok {
		agent := factory(llm.Config{}, "")
		return &AgentInfo{
			ID:          id,
			Name:        agent.Name(),
			Description: agent.Description(),
		}
	}
	return nil
}
