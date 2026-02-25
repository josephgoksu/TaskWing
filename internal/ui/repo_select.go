package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PromptRepoSelection displays a multi-select list for choosing repositories to bootstrap.
// All repos are pre-selected by default (user deselects what they don't want).
// preSelected is optional; if empty, all repos are pre-selected.
func PromptRepoSelection(repos []string, preSelected ...string) ([]string, error) {
	selected := make(map[string]bool)
	if len(preSelected) > 0 {
		for _, r := range preSelected {
			selected[r] = true
		}
	} else {
		// Default: all repos selected
		for _, r := range repos {
			selected[r] = true
		}
	}

	m := repoSelectModel{
		choices:  repos,
		cursor:   0,
		selected: selected,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running repo selection: %w", err)
	}

	result := finalModel.(repoSelectModel)
	if result.quit {
		return nil, nil
	}

	// Build list of selected repos in original order
	var resultSelected []string
	for _, r := range repos {
		if result.selected[r] {
			resultSelected = append(resultSelected, r)
		}
	}
	return resultSelected, nil
}

// repoSelectModel is the Bubble Tea model for repository multi-selection.
type repoSelectModel struct {
	choices  []string
	cursor   int
	selected map[string]bool
	quit     bool
}

func (m repoSelectModel) Init() tea.Cmd {
	return nil
}

func (m repoSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			// Confirm selection
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
			// No selection — treat as cancel
			m.quit = true
			return m, tea.Quit
		case "s":
			// Skip repo selection (use all)
			for _, c := range m.choices {
				m.selected[c] = true
			}
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

func (m repoSelectModel) View() string {
	checkedStyle := lipgloss.NewStyle().Foreground(ColorSelected)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(StyleSelectTitle.Render("Choose repositories to bootstrap:"))
	sb.WriteString("\n\n")

	for i, choice := range m.choices {
		displayName := filepath.Base(choice)

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
		fmt.Fprintf(&sb, "%s%s %s\n", cursor, checkbox, style.Render(displayName))
	}

	// Count selected
	count := 0
	for _, v := range m.selected {
		if v {
			count++
		}
	}

	sb.WriteString("\n")
	sb.WriteString(StyleSelectDim.Render(fmt.Sprintf("%d/%d selected", count, len(m.choices))))
	sb.WriteString("\n")
	sb.WriteString(StyleSelectDim.Render("↑/↓ navigate • space toggle • a all • enter confirm • s skip (all) • esc cancel"))
	sb.WriteString("\n")
	return sb.String()
}
