package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// PromptModelSelection prompts the user to select a model for the given provider.
// Returns the selected model ID, or empty string if cancelled.
func PromptModelSelection(provider string) (string, error) {
	models := llm.GetModelsForProvider(provider)
	if len(models) == 0 {
		// No models in pricing table for this provider, use default
		return llm.DefaultModelForProvider(provider), nil
	}

	m := modelSelectModel{
		provider: provider,
		models:   models,
		cursor:   0, // Default is sorted first
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running model selection: %w", err)
	}

	result := finalModel.(modelSelectModel)
	if result.quit {
		return "", fmt.Errorf("model selection cancelled")
	}

	return result.selectedID, nil
}

type modelSelectModel struct {
	provider   string
	models     []llm.ModelOption
	cursor     int
	selectedID string
	quit       bool
}

func (m modelSelectModel) Init() tea.Cmd {
	return nil
}

func (m modelSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quit = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.models)-1 {
				m.cursor++
			}
		case "enter":
			m.selectedID = m.models[m.cursor].ID
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m modelSelectModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	defaultBadge := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)

	s := "\n" + titleStyle.Render(fmt.Sprintf("🧠 Select Model for %s", m.provider)) + "\n\n"

	for i, model := range m.models {
		cursor := "  "
		style := normalStyle

		if m.cursor == i {
			cursor = "▶ "
			style = selectedStyle
		}

		line := fmt.Sprintf("%s%s", cursor, style.Render(fmt.Sprintf("%-24s", model.DisplayName)))
		if model.IsDefault {
			line += defaultBadge.Render(" (default)")
		}
		line += dimStyle.Render(" " + model.PriceInfo)
		s += line + "\n"
	}

	s += "\n" + dimStyle.Render("↑/↓ navigate • enter select • esc cancel") + "\n"
	return s
}
