package impl

import (
	"testing"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
)

func TestUtilityAgentsRegistered(t *testing.T) {
	// Get all registered agents
	registry := core.Registry()

	// Build a map for easy lookup
	registered := make(map[string]bool)
	for _, info := range registry {
		registered[info.ID] = true
	}

	// Verify our utility agents are registered
	expectedAgents := []string{"simplify", "explain", "debug"}
	for _, id := range expectedAgents {
		if !registered[id] {
			t.Errorf("Agent %q not found in registry", id)
		}
	}
}

func TestSimplifyAgentInfo(t *testing.T) {
	info := core.GetAgentByID("simplify")
	if info == nil {
		t.Fatal("simplify agent not found")
	}
	if info.Name != "Code Simplification" {
		t.Errorf("expected name 'Code Simplification', got %q", info.Name)
	}
}

func TestExplainAgentInfo(t *testing.T) {
	info := core.GetAgentByID("explain")
	if info == nil {
		t.Fatal("explain agent not found")
	}
	if info.Name != "Code Explanation" {
		t.Errorf("expected name 'Code Explanation', got %q", info.Name)
	}
}

func TestDebugAgentInfo(t *testing.T) {
	info := core.GetAgentByID("debug")
	if info == nil {
		t.Fatal("debug agent not found")
	}
	if info.Name != "Debug Helper" {
		t.Errorf("expected name 'Debug Helper', got %q", info.Name)
	}
}
