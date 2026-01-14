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

// agentRegistration holds both factory and static metadata.
type agentRegistration struct {
	factory     AgentFactory
	name        string
	description string
}

var (
	registrations   = make(map[string]agentRegistration)
	registrationsMu sync.RWMutex
)

// RegisterAgent registers an agent with static metadata.
// This avoids creating agents with empty config just to get Name/Description.
func RegisterAgent(id string, factory AgentFactory, name, description string) {
	registrationsMu.Lock()
	defer registrationsMu.Unlock()
	registrations[id] = agentRegistration{
		factory:     factory,
		name:        name,
		description: description,
	}
}

// CreateAgent creates an agent by ID using the registered factory.
func CreateAgent(id string, cfg llm.Config, basePath string) Agent {
	registrationsMu.RLock()
	defer registrationsMu.RUnlock()
	if reg, ok := registrations[id]; ok {
		return reg.factory(cfg, basePath)
	}
	return nil
}

// CreateAllAgents creates all registered agents.
func CreateAllAgents(cfg llm.Config, basePath string) []Agent {
	registrationsMu.RLock()
	defer registrationsMu.RUnlock()
	agents := make([]Agent, 0, len(registrations))
	for _, reg := range registrations {
		agents = append(agents, reg.factory(cfg, basePath))
	}
	return agents
}

// Registry returns metadata for all registered agents.
// Uses static metadata instead of instantiating agents.
func Registry() []AgentInfo {
	registrationsMu.RLock()
	defer registrationsMu.RUnlock()
	infos := make([]AgentInfo, 0, len(registrations))
	for id, reg := range registrations {
		infos = append(infos, AgentInfo{
			ID:          id,
			Name:        reg.name,
			Description: reg.description,
		})
	}
	return infos
}

// GetAgentByID returns agent info by ID.
// Uses static metadata instead of instantiating agents.
func GetAgentByID(id string) *AgentInfo {
	registrationsMu.RLock()
	defer registrationsMu.RUnlock()
	if reg, ok := registrations[id]; ok {
		return &AgentInfo{
			ID:          id,
			Name:        reg.name,
			Description: reg.description,
		}
	}
	return nil
}
