package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ProviderOption struct {
	ID          string
	Name        string
	Description string
}

var providerOptions = []ProviderOption{
	{
		ID:          "openai",
		Name:        "OpenAI",
		Description: "Cloud-based, requires API Key",
	},
	{
		ID:          "anthropic",
		Name:        "Claude",
		Description: "Anthropic, requires API Key",
	},
	{
		ID:          "gemini",
		Name:        "Gemini",
		Description: "Google, requires API Key",
	},
	{
		ID:          "ollama",
		Name:        "Ollama",
		Description: "Local, private, free (requires Ollama running)",
	},
}

// PromptLLMProvider prompts the user to select an LLM provider.
func PromptLLMProvider() (string, error) {
	m := providerSelectModel{
		options: providerOptions,
		cursor:  0,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running provider selection: %w", err)
	}

	result := finalModel.(providerSelectModel)
	if result.quit {
		return "", fmt.Errorf("provider selection cancelled")
	}

	return result.selectedID, nil
}

type providerSelectModel struct {
	options    []ProviderOption
	cursor     int
	selectedID string
	quit       bool
}

func (m providerSelectModel) Init() tea.Cmd {
	return nil
}

func (m providerSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.selectedID = m.options[m.cursor].ID
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m providerSelectModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	s := "\n" + titleStyle.Render("🤖 Select AI Provider") + "\n\n"

	for i, opt := range m.options {
		cursor := "  "
		style := normalStyle

		if m.cursor == i {
			cursor = "▶ "
			style = selectedStyle
		}

		line := fmt.Sprintf("%s%s", cursor, style.Render(fmt.Sprintf("%-10s", opt.Name)))
		line += dimStyle.Render(" " + opt.Description)
		s += line + "\n"
	}

	s += "\n" + dimStyle.Render("↑/↓ navigate • enter select • esc cancel") + "\n"
	return s
}
