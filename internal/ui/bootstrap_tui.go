package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudwego/eino/callbacks"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
)

type AgentStatus int

const (
	StatusWaiting AgentStatus = iota
	StatusRunning
	StatusDone
	StatusError
)

type AgentState struct {
	Name        string
	Status      AgentStatus
	Message     string
	Result      *core.Output
	Err         error
	Spinner     spinner.Model
	StartedAt   time.Time
	LastTool    string
	LastNode    string
	LastEventAt time.Time
}

type AgentResultMsg struct {
	Name   string
	Output *core.Output
	Err    error
}

type BootstrapModel struct {
	Agents      []*AgentState
	Context     context.Context
	Input       core.Input
	RealAgents  []core.Agent
	Quitting    bool
	Results     []core.Output
	Err         error
	ResultsChan *core.StreamingOutput
	ShowDetails bool
	SelectedIdx int
	EventLog    map[string][]core.StreamEvent
}

func NewBootstrapModel(ctx context.Context, input core.Input, agentsList []core.Agent, stream *core.StreamingOutput) BootstrapModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	states := make([]*AgentState, len(agentsList))
	for i, a := range agentsList {
		states[i] = &AgentState{
			Name:    a.Name(),
			Status:  StatusRunning,
			Message: "Starting...",
			Spinner: s,
		}
	}

	return BootstrapModel{
		Agents:      states,
		Context:     ctx,
		Input:       input,
		RealAgents:  agentsList,
		Results:     make([]core.Output, 0),
		ResultsChan: stream,
		ShowDetails: false,
		SelectedIdx: 0,
		EventLog:    make(map[string][]core.StreamEvent),
	}
}

func (m BootstrapModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	if len(m.Agents) > 0 {
		cmds = append(cmds, m.Agents[0].Spinner.Tick)
	}

	// Start all agents concurrently
	for _, a := range m.RealAgents {
		cmds = append(cmds, runAgent(m.Context, a, m.Input, m.ResultsChan))
	}

	// Start listener
	cmds = append(cmds, listenForEvents(m.ResultsChan))

	return tea.Batch(cmds...)
}

func runAgent(ctx context.Context, agent core.Agent, input core.Input, stream *core.StreamingOutput) tea.Cmd {
	return func() tea.Msg {
		// Attach streaming handler
		handler := core.CreateStreamingCallbackHandler(agent.Name(), stream)

		// We need to inject this handler into the context for Eino
		// But our Agent.Run() interface takes Input struct.
		// We'll need to assume agents respect context injection or update Agent.Run signature?
		// Agents usually do: core.NewDeterministicChain(ctx...) -> use ctx
		// So we inject callbacks into ctx here.
		runCtx := callbacks.InitCallbacks(ctx, &callbacks.RunInfo{Name: agent.Name()}, handler.Build())

		// Disable verbose logging for the agent to avoid messing up the TUI
		input.Verbose = false

		output, err := agent.Run(runCtx, input)
		output.AgentName = agent.Name()

		return AgentResultMsg{
			Name:   agent.Name(),
			Output: &output,
			Err:    err,
		}
	}
}

func listenForEvents(stream *core.StreamingOutput) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-stream.Events
		if !ok {
			return nil
		}
		return event
	}
}

func (m BootstrapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.Quitting = true
			return m, tea.Quit
		case "t":
			m.ShowDetails = !m.ShowDetails
		case "tab":
			if len(m.Agents) > 0 {
				m.SelectedIdx = (m.SelectedIdx + 1) % len(m.Agents)
			}
		}

	case spinner.TickMsg:
		var cmds []tea.Cmd
		for i, state := range m.Agents {
			if state.Status == StatusRunning {
				var cmd tea.Cmd
				state.Spinner, cmd = state.Spinner.Update(msg)
				cmds = append(cmds, cmd)
				m.Agents[i] = state
			}
		}
		// Continue listening loop if we have a stream
		// (stream checking is handled by the recursive cmd from StreamEvent)
		return m, tea.Batch(cmds...)

	case core.StreamEvent:
		// Handle stream event
		var cmds []tea.Cmd

		// Track event history per agent (last 12), but ignore events after error.
		if msg.Agent != "" {
			for _, state := range m.Agents {
				if state.Name == msg.Agent {
					if state.Status == StatusError && msg.Type != core.EventAgentError {
						break
					}
					events := m.EventLog[msg.Agent]
					events = append(events, msg)
					if len(events) > 12 {
						events = events[len(events)-12:]
					}
					m.EventLog[msg.Agent] = events
					break
				}
			}
		}

		// Update specific agent state
		for i, state := range m.Agents {
			if state.Name == msg.Agent {
				if state.Status == StatusError && msg.Type != core.EventAgentError {
					continue
				}
				switch msg.Type {
				case core.EventAgentStart:
					if state.StartedAt.IsZero() {
						state.StartedAt = msg.Timestamp
					}
					state.LastEventAt = msg.Timestamp
				case core.EventAgentError:
					if state.StartedAt.IsZero() {
						state.StartedAt = msg.Timestamp
					}
					state.Status = StatusError
					state.Err = fmt.Errorf("%s", msg.Content)
					state.LastEventAt = msg.Timestamp
					state.Message = fmt.Sprintf("Error: %s", msg.Content)
				case core.EventNodeStart:
					// "The Pulse": Update message with current activity
					// logic moved to callbacks.go, so Content is "Thinking", "Templating", etc.
					state.Message = fmt.Sprintf("%s...", msg.Content)
					state.LastNode = msg.Content
					state.LastEventAt = msg.Timestamp

				case core.EventToolCall:
					// For ReAct agents
					state.Message = fmt.Sprintf("Tool: %s", msg.Content)
					state.LastTool = msg.Content
					state.LastEventAt = msg.Timestamp
				case core.EventToolResult:
					state.LastEventAt = msg.Timestamp
				}
				m.Agents[i] = state
			}
		}

		// Continue listening
		cmds = append(cmds, listenForEvents(m.ResultsChan))
		return m, tea.Batch(cmds...)

	case AgentResultMsg:
		allDone := true
		for i, state := range m.Agents {
			if state.Name == msg.Name {
				// AgentResultMsg is the final result - it overrides any intermediate
				// errors captured during retry attempts. This fixes the bug where
				// retryable errors (like JSON parse errors) would mark the agent as
				// failed even after successful retry.
				if msg.Err != nil {
					state.Status = StatusError
					state.Err = msg.Err
					state.Message = fmt.Sprintf("Error: %v", msg.Err)
				} else if msg.Output != nil && msg.Output.Error != nil {
					// Agent returned successfully but with an embedded error/warning
					// This happens when agent processes data but finds nothing meaningful
					state.Status = StatusDone // Show as done (not error) since agent completed
					state.Err = nil           // Clear any intermediate retry errors
					state.Result = msg.Output
					state.Message = fmt.Sprintf("Warning: %v", msg.Output.Error)
					m.Results = append(m.Results, *msg.Output)
				} else {
					// Agent completed successfully - clear any intermediate errors
					state.Status = StatusDone
					state.Err = nil // Clear intermediate retry errors (e.g., JSON parse errors that were retried)
					state.Result = msg.Output
					count := len(msg.Output.Findings)
					state.Message = fmt.Sprintf("Found %d items", count)
					m.Results = append(m.Results, *msg.Output)
				}
				m.Agents[i] = state
			}
			if m.Agents[i].Status == StatusRunning {
				allDone = false
			}
		}

		if allDone {
			// Close the stream to unblock any pending listenForEvents commands
			// before quitting. This prevents the TUI from hanging.
			m.ResultsChan.Close()
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m BootstrapModel) View() string {
	if m.Quitting {
		return ""
	}

	var s strings.Builder
	s.WriteString(" 🤖 Running agents... ")
	s.WriteString(StyleSubtle.Render("(t details • tab cycle)"))
	s.WriteString("\n")

	for _, state := range m.Agents {
		s.WriteString(" ")
		switch state.Status {
		case StatusRunning:
			s.WriteString(state.Spinner.View())
			s.WriteString("  ")
			s.WriteString(StyleTitle.Render(fmt.Sprintf("%-12s", state.Name))) // Fixed width for alignment
			s.WriteString(" ")
			s.WriteString(StyleSubtle.Render(state.Message))
			if !state.StartedAt.IsZero() {
				elapsed := time.Since(state.StartedAt).Round(time.Second)
				s.WriteString(StyleSubtle.Render(fmt.Sprintf(" • %s", elapsed)))
			}
		case StatusDone:
			s.WriteString(StyleSuccess.Render("✓"))
			s.WriteString("  ")
			s.WriteString(StyleTitle.Render(fmt.Sprintf("%-12s", state.Name)))
			s.WriteString(" ")
			s.WriteString(StyleSuccess.Render(state.Message))
		case StatusError:
			s.WriteString(StyleError.Render("✗"))
			s.WriteString("  ")
			s.WriteString(StyleTitle.Render(fmt.Sprintf("%-12s", state.Name)))
			s.WriteString(" ")
			s.WriteString(StyleError.Render(state.Message))
		}
		s.WriteString("\n")
	}

	if m.ShowDetails && len(m.Agents) > 0 {
		s.WriteString("\n")
		agent := m.Agents[m.SelectedIdx]
		s.WriteString(StyleTitle.Render(fmt.Sprintf(" Details: %s", agent.Name)))
		s.WriteString("\n")
		events := m.EventLog[agent.Name]
		if len(events) == 0 {
			s.WriteString(StyleSubtle.Render("  (no events yet)\n"))
		} else {
			for _, ev := range events {
				ts := ev.Timestamp.Format("15:04:05")
				content := ev.Content
				if content == "" && ev.Metadata != nil {
					if name, ok := ev.Metadata["node_name"].(string); ok && name != "" {
						content = name
					} else if ntype, ok := ev.Metadata["node_type"].(string); ok && ntype != "" {
						content = ntype
					}
				}
				meta := ""
				if ev.Metadata != nil {
					if model, ok := ev.Metadata["model"].(string); ok && model != "" {
						if !strings.Contains(content, model) {
							meta = fmt.Sprintf(" (%s)", model)
						}
					}
					switch total := ev.Metadata["total_tokens"].(type) {
					case int:
						if total > 0 {
							meta = fmt.Sprintf("%s tok:%d", meta, total)
						}
					case int64:
						if total > 0 {
							meta = fmt.Sprintf("%s tok:%d", meta, total)
						}
					case float64:
						if total > 0 {
							meta = fmt.Sprintf("%s tok:%d", meta, int(total))
						}
					}
				}
				line := fmt.Sprintf("  [%s] %s: %s%s", ts, ev.Type, content, meta)
				s.WriteString(StyleSubtle.Render(line))
				s.WriteString("\n")
			}
		}
	}

	return s.String()
}
