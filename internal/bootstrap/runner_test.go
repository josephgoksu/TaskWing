package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
)

// mockAgent is a simple agent implementation for testing
type mockAgent struct {
	name        string
	description string
	runFunc     func(ctx context.Context, input core.Input) (core.Output, error)
	closeFn     func()
}

func (m *mockAgent) Name() string        { return m.name }
func (m *mockAgent) Description() string { return m.description }
func (m *mockAgent) Run(ctx context.Context, input core.Input) (core.Output, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, input)
	}
	return core.Output{AgentName: m.name}, nil
}

// Close implements CloseableAgent for testing (optional, called via core.CloseAgents)
func (m *mockAgent) Close() error {
	if m.closeFn != nil {
		m.closeFn()
	}
	return nil
}

func TestRunner_Close(t *testing.T) {
	closed := false
	agent := &mockAgent{
		name:    "test-agent",
		closeFn: func() { closed = true },
	}

	runner := &Runner{agents: []core.Agent{agent}}
	runner.Close()

	if !closed {
		t.Error("Agent was not closed")
	}
}

func TestRunner_Run_ContextCancelled(t *testing.T) {
	agent := &mockAgent{
		name: "slow-agent",
		runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
			// Simulate slow work
			select {
			case <-ctx.Done():
				return core.Output{}, ctx.Err()
			case <-time.After(5 * time.Second):
				return core.Output{AgentName: "slow-agent"}, nil
			}
		},
	}

	runner := &Runner{agents: []core.Agent{agent}}
	defer runner.Close()

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runner.Run(ctx, "/test/path")
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestRunner_Run_Success(t *testing.T) {
	finding := core.Finding{
		Type:        "test",
		Title:       "Test Finding",
		Description: "Test description",
	}

	agent := &mockAgent{
		name: "success-agent",
		runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
			return core.Output{
				AgentName: "success-agent",
				Findings:  []core.Finding{finding},
			}, nil
		},
	}

	runner := &Runner{agents: []core.Agent{agent}}
	defer runner.Close()

	results, err := runner.Run(context.Background(), "/test/path")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].AgentName != "success-agent" {
		t.Errorf("AgentName = %q, want %q", results[0].AgentName, "success-agent")
	}

	if len(results[0].Findings) != 1 {
		t.Errorf("Findings count = %d, want 1", len(results[0].Findings))
	}
}

func TestRunner_Run_MultipleAgents(t *testing.T) {
	agents := []core.Agent{
		&mockAgent{
			name: "agent-1",
			runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				return core.Output{AgentName: "agent-1"}, nil
			},
		},
		&mockAgent{
			name: "agent-2",
			runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				return core.Output{AgentName: "agent-2"}, nil
			},
		},
	}

	runner := &Runner{agents: agents}
	defer runner.Close()

	results, err := runner.Run(context.Background(), "/test/path")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
}

func TestRunner_Run_PartialFailure(t *testing.T) {
	agents := []core.Agent{
		&mockAgent{
			name: "success-agent",
			runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				return core.Output{AgentName: "success-agent"}, nil
			},
		},
		&mockAgent{
			name: "fail-agent",
			runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				return core.Output{}, context.DeadlineExceeded
			},
		},
	}

	runner := &Runner{agents: agents}
	defer runner.Close()

	results, err := runner.Run(context.Background(), "/test/path")
	// Should succeed with partial results
	if err != nil {
		t.Fatalf("Expected nil error for partial failure, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result (partial success), got %d", len(results))
	}
}

func TestRunner_Run_AllFailed(t *testing.T) {
	agents := []core.Agent{
		&mockAgent{
			name: "fail-agent-1",
			runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				return core.Output{}, context.DeadlineExceeded
			},
		},
		&mockAgent{
			name: "fail-agent-2",
			runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
				return core.Output{}, context.Canceled
			},
		},
	}

	runner := &Runner{agents: agents}
	defer runner.Close()

	_, err := runner.Run(context.Background(), "/test/path")
	if err == nil {
		t.Error("Expected error when all agents fail")
	}
}

func TestRunner_Run_DurationTracking(t *testing.T) {
	agent := &mockAgent{
		name: "timed-agent",
		runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
			time.Sleep(10 * time.Millisecond)
			return core.Output{AgentName: "timed-agent"}, nil // Duration not set
		},
	}

	runner := &Runner{agents: []core.Agent{agent}}
	defer runner.Close()

	results, err := runner.Run(context.Background(), "/test/path")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Duration should be set by runner since agent didn't set it
	if results[0].Duration < 10*time.Millisecond {
		t.Errorf("Duration = %v, expected at least 10ms", results[0].Duration)
	}
}

// === Workspace Tagging Tests ===

func TestRunner_RunWithOptions_WorkspacePassed(t *testing.T) {
	var receivedInput core.Input
	agent := &mockAgent{
		name: "workspace-agent",
		runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
			receivedInput = input
			return core.Output{AgentName: "workspace-agent"}, nil
		},
	}

	runner := &Runner{agents: []core.Agent{agent}}
	defer runner.Close()

	_, err := runner.RunWithOptions(context.Background(), "/test/path", RunOptions{Workspace: "osprey"})
	if err != nil {
		t.Fatalf("RunWithOptions failed: %v", err)
	}

	if receivedInput.Workspace != "osprey" {
		t.Errorf("Input.Workspace = %q, want %q", receivedInput.Workspace, "osprey")
	}
}

func TestRunner_RunWithOptions_DefaultsToRoot(t *testing.T) {
	var receivedInput core.Input
	agent := &mockAgent{
		name: "workspace-agent",
		runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
			receivedInput = input
			return core.Output{AgentName: "workspace-agent"}, nil
		},
	}

	runner := &Runner{agents: []core.Agent{agent}}
	defer runner.Close()

	// Empty workspace should default to "root"
	_, err := runner.RunWithOptions(context.Background(), "/test/path", RunOptions{Workspace: ""})
	if err != nil {
		t.Fatalf("RunWithOptions failed: %v", err)
	}

	if receivedInput.Workspace != "root" {
		t.Errorf("Input.Workspace = %q, want %q (default)", receivedInput.Workspace, "root")
	}
}

func TestRunner_Run_UsesRootWorkspace(t *testing.T) {
	var receivedInput core.Input
	agent := &mockAgent{
		name: "workspace-agent",
		runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
			receivedInput = input
			return core.Output{AgentName: "workspace-agent"}, nil
		},
	}

	runner := &Runner{agents: []core.Agent{agent}}
	defer runner.Close()

	// Regular Run() should use "root" workspace
	_, err := runner.Run(context.Background(), "/test/path")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if receivedInput.Workspace != "root" {
		t.Errorf("Input.Workspace = %q, want %q", receivedInput.Workspace, "root")
	}
}

func TestAgentsWorkspaceTagging(t *testing.T) {
	// Test that agents receive workspace and can use it for tagging findings
	tests := []struct {
		name          string
		workspace     string
		wantWorkspace string
	}{
		{"explicit workspace", "osprey", "osprey"},
		{"root workspace", "root", "root"},
		{"empty defaults to root", "", "root"},
		{"different service", "studio", "studio"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedWorkspace string
			agent := &mockAgent{
				name: "tagging-agent",
				runFunc: func(ctx context.Context, input core.Input) (core.Output, error) {
					capturedWorkspace = input.Workspace
					// Agent can use input.Workspace to tag findings
					return core.Output{
						AgentName: "tagging-agent",
						Findings: []core.Finding{
							{
								Type:        "decision",
								Title:       "Test Decision",
								Description: "A test decision",
								Metadata: map[string]any{
									"workspace": input.Workspace, // Agents can tag findings
								},
							},
						},
					}, nil
				},
			}

			runner := &Runner{agents: []core.Agent{agent}}
			defer runner.Close()

			results, err := runner.RunWithOptions(context.Background(), "/test/path", RunOptions{Workspace: tt.workspace})
			if err != nil {
				t.Fatalf("RunWithOptions failed: %v", err)
			}

			if capturedWorkspace != tt.wantWorkspace {
				t.Errorf("captured workspace = %q, want %q", capturedWorkspace, tt.wantWorkspace)
			}

			// Verify finding has workspace metadata
			if len(results) > 0 && len(results[0].Findings) > 0 {
				finding := results[0].Findings[0]
				if ws, ok := finding.Metadata["workspace"].(string); ok {
					if ws != tt.wantWorkspace {
						t.Errorf("finding.Metadata[workspace] = %q, want %q", ws, tt.wantWorkspace)
					}
				}
			}
		})
	}
}
