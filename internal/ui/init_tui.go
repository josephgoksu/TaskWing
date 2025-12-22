package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PromptAISelection displays a multi-select list for choosing AI assistants.
// choices: ordered list of IDs (e.g., "claude", "cursor")
// descriptions: map of ID -> Display Name (e.g., "claude" -> "Claude Code")
func PromptAISelection(choices []string, descriptions map[string]string) ([]string, error) {
	m := aiSelectModel{
		choices:      choices,
		descriptions: descriptions,
		cursor:       0,
		selected:     make(map[string]bool),
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running selection: %w", err)
	}

	result := finalModel.(aiSelectModel)
	if result.quit {
		return nil, nil
	}

	// Build list of selected in order
	var selected []string
	for _, c := range choices {
		if result.selected[c] {
			selected = append(selected, c)
		}
	}
	return selected, nil
}

// aiSelectModel is the Bubble Tea model for AI multi-selection
type aiSelectModel struct {
	choices      []string
	descriptions map[string]string
	cursor       int
	selected     map[string]bool
	quit         bool
}

func (m aiSelectModel) Init() tea.Cmd {
	return nil
}

func (m aiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case " ":
			// Toggle selection
			choice := m.choices[m.cursor]
			m.selected[choice] = !m.selected[choice]
		case "enter":
			// Confirm selection (must have at least one selected)
			hasSelection := false
			for _, v := range m.selected {
				if v {
					hasSelection = true
					break
				}
			}
			if hasSelection {
				return m, tea.Quit
			}
		case "a":
			// Select all
			for _, c := range m.choices {
				m.selected[c] = true
			}
		}
	}
	return m, nil
}

func (m aiSelectModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	checkedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	s := "\n" + titleStyle.Render("Choose your AI assistant(s):") + "\n\n"

	for i, choice := range m.choices {
		displayName := m.descriptions[choice]
		// Fallback if description missing
		if displayName == "" {
			displayName = choice
		}

		cursor := "  "
		checkbox := "[ ]"
		style := normalStyle

		if m.selected[choice] {
			checkbox = checkedStyle.Render("[✓]")
		}

		if m.cursor == i {
			cursor = "▶ "
			style = selectedStyle
		}
		s += cursor + checkbox + " " + style.Render(fmt.Sprintf("%-10s", choice)) + dimStyle.Render(" - "+displayName) + "\n"
	}

	s += "\n" + dimStyle.Render("↑/↓ navigate • space toggle • a all • enter confirm • esc cancel") + "\n"
	return s
}
