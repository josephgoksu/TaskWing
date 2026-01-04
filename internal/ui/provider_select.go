package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

type ProviderOption struct {
	ID          string
	Name        string
	Description string
	HasAPIKey   bool
}

// buildProviderOptions creates provider options from ModelRegistry.
func buildProviderOptions() []ProviderOption {
	providers := llm.GetProviders()
	options := make([]ProviderOption, 0, len(providers))

	for _, p := range providers {
		hasKey := p.IsLocal || config.ResolveAPIKey(llm.Provider(p.ID)) != ""

		var desc string
		if p.IsLocal {
			desc = "Local, private, free"
		} else {
			// Show price range and model count
			desc = fmt.Sprintf("$%.2f-$%.2f/1M • %d models", p.MinPrice, p.MaxPrice, p.ModelCount)
			if !hasKey {
				desc += " • ❌ key not set"
			}
		}

		options = append(options, ProviderOption{
			ID:          p.ID,
			Name:        p.DisplayName,
			Description: desc,
			HasAPIKey:   hasKey,
		})
	}

	return options
}

// PromptLLMProvider prompts the user to select an LLM provider.
func PromptLLMProvider() (string, error) {
	options := buildProviderOptions()
	m := providerSelectModel{
		options: options,
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

// LLMSelection contains the result of provider + model selection.
type LLMSelection struct {
	Provider string
	Model    string
}

// PromptLLMSelection runs an interactive provider then model selection flow.
// Returns the selected provider and model, or an error if cancelled.
func PromptLLMSelection() (*LLMSelection, error) {
	// Step 1: Select provider
	provider, err := PromptLLMProvider()
	if err != nil {
		return nil, err
	}

	// Step 2: Select model for that provider
	model, err := PromptModelSelection(provider)
	if err != nil {
		return nil, err
	}

	return &LLMSelection{
		Provider: provider,
		Model:    model,
	}, nil
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
	s := "\n" + StyleSelectTitle.Render("🤖 Select AI Provider") + "\n\n"

	for i, opt := range m.options {
		cursor := "  "
		style := StyleSelectNormal

		if m.cursor == i {
			cursor = "▶ "
			style = StyleSelectActive
		}

		line := fmt.Sprintf("%s%s", cursor, style.Render(fmt.Sprintf("%-10s", opt.Name)))
		line += StyleSelectDim.Render(" " + opt.Description)
		s += line + "\n"
	}

	s += "\n" + StyleSelectDim.Render("↑/↓ navigate • enter select • esc cancel") + "\n"
	return s
}
