package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PromptAISelection displays a multi-select list for choosing AI assistants.
// choices: ordered list of IDs (e.g., "claude", "cursor")
// descriptions: map of ID -> Display Name (e.g., "claude" -> "Claude Code")
// preSelected: optional list of IDs to pre-select (e.g., from detected global config)
func PromptAISelection(choices []string, descriptions map[string]string, preSelected ...string) ([]string, error) {
	selected := make(map[string]bool)
	for _, id := range preSelected {
		selected[id] = true
	}

	m := aiSelectModel{
		choices:      choices,
		descriptions: descriptions,
		cursor:       0,
		selected:     selected,
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
	var resultSelected []string
	for _, c := range choices {
		if result.selected[c] {
			resultSelected = append(resultSelected, c)
		}
	}
	return resultSelected, nil
}

// aiSelectModel is the Bubble Tea model for AI multi-selection
type aiSelectModel struct {
	choices      []string
	descriptions map[string]string
	cursor       int
	selected     map[string]bool
	quit         bool
	skipped      bool
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
			// Confirm selection or skip if nothing selected
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
			// No selection = skip
			m.skipped = true
			return m, tea.Quit
		case "s":
			// Skip AI setup
			m.skipped = true
			return m, tea.Quit
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
	checkedStyle := lipgloss.NewStyle().Foreground(ColorSelected)

	s := "\n" + StyleSelectTitle.Render("Choose your AI assistant(s):") + "\n\n"

	for i, choice := range m.choices {
		displayName := m.descriptions[choice]
		// Fallback if description missing
		if displayName == "" {
			displayName = choice
		}

		cursor := "  "
		checkbox := "[ ]"
		style := StyleSelectNormal

		if m.selected[choice] {
			checkbox = checkedStyle.Render("[✓]")
		}

		if m.cursor == i {
			cursor = "▶ "
			style = StyleSelectActive
		}
		s += cursor + checkbox + " " + style.Render(fmt.Sprintf("%-10s", choice)) + StyleSelectDim.Render(" - "+displayName) + "\n"
	}

	s += "\n" + StyleSelectDim.Render("↑/↓ navigate • space toggle • a all • enter confirm • s skip • esc cancel") + "\n"
	return s
}
