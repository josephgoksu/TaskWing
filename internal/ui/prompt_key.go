package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PromptAPIKey prompts the user to enter their LLM API key.
// Returns the entered key or an error.
func PromptAPIKey() (string, error) {
	ti := textinput.New()
	ti.Placeholder = "api-key"
	ti.Focus()
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 256
	ti.Width = 50

	m := apiKeyModel{
		textInput: ti,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running prompt: %w", err)
	}

	result := finalModel.(apiKeyModel)
	if result.quit {
		return "", fmt.Errorf("api key input cancelled")
	}

	return result.value, nil
}

type apiKeyModel struct {
	textInput textinput.Model
	value     string
	quit      bool
}

func (m apiKeyModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m apiKeyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.value = m.textInput.Value()
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quit = true
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m apiKeyModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	s := "\n" + titleStyle.Render("🔑 API Key required") + "\n"
	s += dimStyle.Render("It will be stored locally in ~/.taskwing/config.yaml") + "\n\n"
	s += m.textInput.View() + "\n\n"
	s += dimStyle.Render("Press Enter to confirm • Esc to cancel") + "\n"

	return s
}
