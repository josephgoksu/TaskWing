package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RunnerJobStatus represents the state of a runner analysis job.
type RunnerJobStatus int

const (
	RunnerJobRunning RunnerJobStatus = iota
	RunnerJobDone
	RunnerJobError
)

// RunnerJobState holds the display state for one analysis job.
type RunnerJobState struct {
	ID       string
	Status   RunnerJobStatus
	Message  string
	Spinner  spinner.Model
	StartAt  time.Time
	Duration time.Duration
}

// RunnerJobDoneMsg signals that a runner job has completed.
type RunnerJobDoneMsg struct {
	ID       string
	Findings int
	Rels     int
	ErrMsg   string
	Duration time.Duration
}

// RunnerProgressModel is a lightweight Bubble Tea model for showing
// spinner progress during parallel runner analysis.
type RunnerProgressModel struct {
	Jobs []*RunnerJobState
	done int
}

// NewRunnerProgressModel creates a new progress model for the given job IDs.
func NewRunnerProgressModel(jobIDs []string) RunnerProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	jobs := make([]*RunnerJobState, len(jobIDs))
	for i, id := range jobIDs {
		jobs[i] = &RunnerJobState{
			ID:      id,
			Status:  RunnerJobRunning,
			Message: "analyzing...",
			Spinner: s,
			StartAt: time.Now(),
		}
	}

	return RunnerProgressModel{Jobs: jobs}
}

func (m RunnerProgressModel) Init() tea.Cmd {
	if len(m.Jobs) == 0 {
		return tea.Quit
	}
	return m.Jobs[0].Spinner.Tick
}

func (m RunnerProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmds []tea.Cmd
		for _, job := range m.Jobs {
			if job.Status == RunnerJobRunning {
				var cmd tea.Cmd
				job.Spinner, cmd = job.Spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case RunnerJobDoneMsg:
		for _, job := range m.Jobs {
			if job.ID == msg.ID {
				job.Duration = msg.Duration
				if msg.ErrMsg != "" {
					job.Status = RunnerJobError
					job.Message = msg.ErrMsg
				} else {
					job.Status = RunnerJobDone
					job.Message = fmt.Sprintf("%d findings, %d relationships", msg.Findings, msg.Rels)
				}
				m.done++
				break
			}
		}

		if m.done >= len(m.Jobs) {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m RunnerProgressModel) View() string {
	var s strings.Builder

	for _, job := range m.Jobs {
		s.WriteString("        ")
		switch job.Status {
		case RunnerJobRunning:
			s.WriteString(job.Spinner.View())
			s.WriteString(" ")
			s.WriteString(StyleTitle.Render(fmt.Sprintf("%-14s", job.ID)))
			s.WriteString(" ")
			s.WriteString(StyleSubtle.Render(job.Message))
			elapsed := time.Since(job.StartAt).Round(time.Second)
			s.WriteString(StyleSubtle.Render(fmt.Sprintf(" %s", elapsed)))
		case RunnerJobDone:
			icon := lipgloss.NewStyle().Foreground(ColorSuccess).Render(IconOK.Emoji)
			durStyle := lipgloss.NewStyle().Foreground(ColorDim)
			s.WriteString(icon)
			s.WriteString(" ")
			s.WriteString(fmt.Sprintf("%-14s", job.ID))
			s.WriteString(" ")
			s.WriteString(StyleSuccess.Render(job.Message))
			s.WriteString(" ")
			s.WriteString(durStyle.Render(FormatDuration(job.Duration)))
		case RunnerJobError:
			icon := lipgloss.NewStyle().Foreground(ColorWarning).Render(IconWarn.Emoji)
			s.WriteString(icon)
			s.WriteString(" ")
			s.WriteString(fmt.Sprintf("%-14s", job.ID))
			s.WriteString(" ")
			s.WriteString(StyleError.Render(job.Message))
		}
		s.WriteString("\n")
	}

	return s.String()
}
