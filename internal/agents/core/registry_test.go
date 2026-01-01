package core

import (
	"context"
	"sync"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/llm"
)

// mockAgent is a simple test implementation of the Agent interface.
type mockAgent struct {
	name        string
	description string
}

func (m *mockAgent) Name() string        { return m.name }
func (m *mockAgent) Description() string { return m.description }
func (m *mockAgent) Run(ctx context.Context, input Input) (Output, error) {
	return Output{}, nil
}

// clearRegistry removes all registrations for test isolation.
func clearRegistry() {
	registrationsMu.Lock()
	defer registrationsMu.Unlock()
	registrations = make(map[string]agentRegistration)
}

func TestRegisterAgent(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	factory := func(cfg llm.Config, basePath string) Agent {
		return &mockAgent{name: "Test Agent", description: "Test Description"}
	}

	RegisterAgent("test-agent", factory, "Test Agent", "Test Description")

	// Verify registration
	info := GetAgentByID("test-agent")
	if info == nil {
		t.Fatal("Expected agent to be registered")
	}
	if info.ID != "test-agent" {
		t.Errorf("ID = %q, want %q", info.ID, "test-agent")
	}
	if info.Name != "Test Agent" {
		t.Errorf("Name = %q, want %q", info.Name, "Test Agent")
	}
	if info.Description != "Test Description" {
		t.Errorf("Description = %q, want %q", info.Description, "Test Description")
	}
}

func TestRegisterAgentFactory_Deprecated(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	factory := func(cfg llm.Config, basePath string) Agent {
		return &mockAgent{name: "Legacy", description: ""}
	}

	// Deprecated function should still work, using ID as name
	RegisterAgentFactory("legacy-agent", factory)

	info := GetAgentByID("legacy-agent")
	if info == nil {
		t.Fatal("Expected agent to be registered via deprecated function")
	}
	if info.Name != "legacy-agent" {
		t.Errorf("Name = %q, want %q (should use ID as name)", info.Name, "legacy-agent")
	}
	if info.Description != "" {
		t.Errorf("Description = %q, want empty string", info.Description)
	}
}

func TestCreateAgent(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	factoryCalled := false
	factory := func(cfg llm.Config, basePath string) Agent {
		factoryCalled = true
		return &mockAgent{name: "Created", description: "By Factory"}
	}

	RegisterAgent("create-test", factory, "Create Test", "")

	cfg := llm.Config{Provider: "openai", Model: "gpt-4"}
	agent := CreateAgent("create-test", cfg, "/test/path")

	if !factoryCalled {
		t.Error("Factory function was not called")
	}
	if agent == nil {
		t.Fatal("CreateAgent returned nil for registered agent")
	}
	if agent.Name() != "Created" {
		t.Errorf("Agent.Name() = %q, want %q", agent.Name(), "Created")
	}
}

func TestCreateAgent_NotFound(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	cfg := llm.Config{Provider: "openai", Model: "gpt-4"}
	agent := CreateAgent("non-existent", cfg, "/test/path")

	if agent != nil {
		t.Error("CreateAgent should return nil for unregistered agent")
	}
}

func TestCreateAllAgents(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	// Register multiple agents
	for i, id := range []string{"agent-a", "agent-b", "agent-c"} {
		name := id
		idx := i
		RegisterAgent(id, func(cfg llm.Config, basePath string) Agent {
			return &mockAgent{name: name, description: string(rune('A' + idx))}
		}, name, "")
	}

	cfg := llm.Config{Provider: "openai", Model: "gpt-4"}
	agents := CreateAllAgents(cfg, "/test/path")

	if len(agents) != 3 {
		t.Errorf("CreateAllAgents returned %d agents, want 3", len(agents))
	}

	// Verify all agents were created (order not guaranteed due to map iteration)
	names := make(map[string]bool)
	for _, agent := range agents {
		names[agent.Name()] = true
	}
	for _, expected := range []string{"agent-a", "agent-b", "agent-c"} {
		if !names[expected] {
			t.Errorf("Missing agent with name %q", expected)
		}
	}
}

func TestRegistry(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	RegisterAgent("reg-1", func(cfg llm.Config, basePath string) Agent {
		return &mockAgent{}
	}, "Registry Agent 1", "First agent")

	RegisterAgent("reg-2", func(cfg llm.Config, basePath string) Agent {
		return &mockAgent{}
	}, "Registry Agent 2", "Second agent")

	infos := Registry()

	if len(infos) != 2 {
		t.Errorf("Registry() returned %d infos, want 2", len(infos))
	}

	// Check that both agents are present (order not guaranteed)
	found := make(map[string]AgentInfo)
	for _, info := range infos {
		found[info.ID] = info
	}

	if info, ok := found["reg-1"]; !ok {
		t.Error("Missing reg-1 in registry")
	} else {
		if info.Name != "Registry Agent 1" {
			t.Errorf("reg-1 Name = %q, want %q", info.Name, "Registry Agent 1")
		}
		if info.Description != "First agent" {
			t.Errorf("reg-1 Description = %q, want %q", info.Description, "First agent")
		}
	}

	if info, ok := found["reg-2"]; !ok {
		t.Error("Missing reg-2 in registry")
	} else {
		if info.Name != "Registry Agent 2" {
			t.Errorf("reg-2 Name = %q, want %q", info.Name, "Registry Agent 2")
		}
	}
}

func TestGetAgentByID(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	RegisterAgent("lookup-test", func(cfg llm.Config, basePath string) Agent {
		return &mockAgent{}
	}, "Lookup Agent", "For testing GetAgentByID")

	tests := []struct {
		name    string
		id      string
		wantNil bool
		wantID  string
	}{
		{
			name:    "existing agent",
			id:      "lookup-test",
			wantNil: false,
			wantID:  "lookup-test",
		},
		{
			name:    "non-existent agent",
			id:      "does-not-exist",
			wantNil: true,
		},
		{
			name:    "empty ID",
			id:      "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := GetAgentByID(tt.id)
			if tt.wantNil {
				if info != nil {
					t.Errorf("GetAgentByID(%q) = %v, want nil", tt.id, info)
				}
			} else {
				if info == nil {
					t.Fatalf("GetAgentByID(%q) = nil, want non-nil", tt.id)
				}
				if info.ID != tt.wantID {
					t.Errorf("GetAgentByID(%q).ID = %q, want %q", tt.id, info.ID, tt.wantID)
				}
			}
		})
	}
}

func TestRegistryEmptyState(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	// Registry should return empty slice, not nil
	infos := Registry()
	if infos == nil {
		t.Error("Registry() returned nil, want empty slice")
	}
	if len(infos) != 0 {
		t.Errorf("Registry() returned %d infos, want 0", len(infos))
	}

	// CreateAllAgents should return empty slice
	agents := CreateAllAgents(llm.Config{}, "")
	if agents == nil {
		t.Error("CreateAllAgents() returned nil, want empty slice")
	}
	if len(agents) != 0 {
		t.Errorf("CreateAllAgents() returned %d agents, want 0", len(agents))
	}
}

func TestConcurrentAccess(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	// Pre-register some agents
	for i := 0; i < 5; i++ {
		id := string(rune('a' + i))
		RegisterAgent(id, func(cfg llm.Config, basePath string) Agent {
			return &mockAgent{name: id}
		}, id, "")
	}

	// Concurrent reads and writes
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			switch n % 4 {
			case 0:
				_ = Registry()
			case 1:
				_ = GetAgentByID("a")
			case 2:
				_ = CreateAgent("b", llm.Config{}, "")
			case 3:
				_ = CreateAllAgents(llm.Config{}, "")
			}
		}(i)
	}
	wg.Wait()
	// If we get here without panics or race conditions, the test passes
}

func TestRegisterOverwrite(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	// Register first version
	RegisterAgent("overwrite-test", func(cfg llm.Config, basePath string) Agent {
		return &mockAgent{name: "Version 1"}
	}, "First Version", "Original")

	// Overwrite with second version
	RegisterAgent("overwrite-test", func(cfg llm.Config, basePath string) Agent {
		return &mockAgent{name: "Version 2"}
	}, "Second Version", "Updated")

	// Should have the second version
	info := GetAgentByID("overwrite-test")
	if info.Name != "Second Version" {
		t.Errorf("Name = %q, want %q (should be overwritten)", info.Name, "Second Version")
	}
	if info.Description != "Updated" {
		t.Errorf("Description = %q, want %q", info.Description, "Updated")
	}

	// Factory should also be updated
	agent := CreateAgent("overwrite-test", llm.Config{}, "")
	if agent.Name() != "Version 2" {
		t.Errorf("Agent.Name() = %q, want %q", agent.Name(), "Version 2")
	}
}
