/*
Package agents provides the agent-driven architecture for codebase analysis.

Agents are specialized analyzers that focus on specific aspects of a project:
- DocAgent: Scans markdown documentation (README, ARCHITECTURE, etc.)
- CodeAgent: Analyzes source code structure and patterns
- GitAgent: Extracts insights from git history
- DepsAgent: Reads dependency files (package.json, go.mod, etc.)

Each agent implements the Agent interface and produces findings that are
converted to knowledge nodes with source attribution.
*/
package agents

// AgentInfo describes an agent for the registry.
type AgentInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Registry contains metadata for all available agents.
// This is used by the API to expose agent information.
var Registry = []AgentInfo{
	{
		ID:          "doc",
		Name:        "DocAgent",
		Description: "Scans markdown documentation (README, ARCHITECTURE, docs/)",
	},
	{
		ID:          "react_code",
		Name:        "CodeAgent",
		Description: "Analyzes source code structure using dynamic ReAct exploration",
	},
	{
		ID:          "git",
		Name:        "GitAgent",
		Description: "Extracts insights from git history and commit patterns",
	},
	{
		ID:          "deps",
		Name:        "DepsAgent",
		Description: "Reads dependency files (package.json, go.mod, Cargo.toml)",
	},
}

// GetAgentByID returns agent info by ID, or nil if not found.
func GetAgentByID(id string) *AgentInfo {
	for _, a := range Registry {
		if a.ID == id {
			return &a
		}
	}
	return nil
}
