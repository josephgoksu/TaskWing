package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/agents"
)

type AgentStatus int

const (
	StatusWaiting AgentStatus = iota
	StatusRunning
	StatusDone
	StatusError
)

type AgentState struct {
	Name    string
	Status  AgentStatus
	Message string
	Result  *agents.Output
	Err     error
	Spinner spinner.Model
}

type AgentResultMsg struct {
	Name   string
	Output *agents.Output
	Err    error
}

type BootstrapModel struct {
	Agents     []*AgentState
	Context    context.Context
	Input      agents.Input
	RealAgents []agents.Agent
	Quitting   bool
	Results    []agents.Output
	Err        error
}

func NewBootstrapModel(ctx context.Context, input agents.Input, agentsList []agents.Agent) BootstrapModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	states := make([]*AgentState, len(agentsList))
	for i, a := range agentsList {
		states[i] = &AgentState{
			Name:    a.Name(),
			Status:  StatusRunning, // Start all immediately
			Message: "Analyzing...",
			Spinner: s,
		}
	}

	return BootstrapModel{
		Agents:     states,
		Context:    ctx,
		Input:      input,
		RealAgents: agentsList,
		Results:    make([]agents.Output, 0),
	}
}

func (m BootstrapModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, spinner.Tick)

	// Start all agents concurrently
	for _, a := range m.RealAgents {
		cmds = append(cmds, runAgent(m.Context, a, m.Input))
	}

	return tea.Batch(cmds...)
}

func runAgent(ctx context.Context, agent agents.Agent, input agents.Input) tea.Cmd {
	return func() tea.Msg {
		// Disable verbose logging for the agent to avoid messing up the TUI
		input.Verbose = false
		output, err := agent.Run(ctx, input)
		return AgentResultMsg{
			Name:   agent.Name(),
			Output: &output,
			Err:    err,
		}
	}
}

func (m BootstrapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.Quitting = true
			return m, tea.Quit
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
		return m, tea.Batch(cmds...)

	case AgentResultMsg:
		allDone := true
		for i, state := range m.Agents {
			if state.Name == msg.Name {
				if msg.Err != nil {
					state.Status = StatusError
					state.Err = msg.Err
					state.Message = fmt.Sprintf("Error: %v", msg.Err)
				} else {
					state.Status = StatusDone
					state.Result = msg.Output
					count := len(msg.Output.Findings)
					state.Message = fmt.Sprintf("Completed (%d findings)", count)
					m.Results = append(m.Results, *msg.Output)
				}
				m.Agents[i] = state
			}
			if m.Agents[i].Status == StatusRunning {
				allDone = false
			}
		}

		if allDone {
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
	s.WriteString(" 🤖 Running agents...\n")

	for _, state := range m.Agents {
		s.WriteString(" ")
		switch state.Status {
		case StatusRunning:
			s.WriteString(state.Spinner.View())
			s.WriteString(" ")
			s.WriteString(StyleTitle.Render(state.Name))
			s.WriteString(" ")
			s.WriteString(state.Message)
		case StatusDone:
			s.WriteString(StyleSuccess.Render("✓"))
			s.WriteString(" ")
			s.WriteString(StyleTitle.Render(state.Name))
			s.WriteString(" ")
			s.WriteString(StyleSubtle.Render(state.Message))
		case StatusError:
			s.WriteString(StyleError.Render("✗"))
			s.WriteString(" ")
			s.WriteString(StyleTitle.Render(state.Name))
			s.WriteString(" ")
			s.WriteString(StyleError.Render(state.Message))
		}
		s.WriteString("\n")
	}

	return s.String()
}
