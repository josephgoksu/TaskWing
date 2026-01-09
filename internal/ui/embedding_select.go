package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// EmbeddingSelection contains the result of embedding provider + model selection.
type EmbeddingSelection struct {
	Provider string
	Model    string
	BaseURL  string // For TEI/Ollama
}

// PromptEmbeddingSelection runs an interactive embedding provider then model selection.
func PromptEmbeddingSelection() (*EmbeddingSelection, error) {
	// Step 1: Select provider
	provider, err := PromptEmbeddingProvider()
	if err != nil {
		return nil, err
	}

	// Step 2: Select model for that provider
	model, err := PromptEmbeddingModel(provider)
	if err != nil {
		return nil, err
	}

	return &EmbeddingSelection{
		Provider: provider,
		Model:    model,
	}, nil
}

// PromptEmbeddingProvider prompts the user to select an embedding provider.
func PromptEmbeddingProvider() (string, error) {
	providers := llm.GetEmbeddingProviders()
	options := make([]embeddingProviderOption, 0, len(providers))

	for _, p := range providers {
		hasKey := p.IsLocal || config.ResolveAPIKey(llm.Provider(p.ID)) != ""

		var desc string
		switch {
		case p.ID == llm.ProviderOllama:
			desc = fmt.Sprintf("Local • %d models • free", p.ModelCount)
		case p.ID == llm.ProviderTEI:
			desc = "Self-hosted TEI server"
		case p.IsFree:
			desc = fmt.Sprintf("%d models • free", p.ModelCount)
		default:
			desc = fmt.Sprintf("%d models • paid", p.ModelCount)
		}

		if !hasKey && !p.IsLocal {
			desc += " • API key needed"
		}

		options = append(options, embeddingProviderOption{
			ID:          p.ID,
			Name:        p.DisplayName,
			Description: desc,
		})
	}

	m := embeddingProviderSelectModel{
		options: options,
		cursor:  0,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running provider selection: %w", err)
	}

	result := finalModel.(embeddingProviderSelectModel)
	if result.quit {
		return "", fmt.Errorf("selection cancelled")
	}

	return result.selectedID, nil
}

// PromptEmbeddingModel prompts the user to select an embedding model for a provider.
func PromptEmbeddingModel(provider string) (string, error) {
	models := llm.GetEmbeddingModelsForProvider(provider)
	if len(models) == 0 {
		// No models defined, use "custom" for TEI or empty
		if provider == llm.ProviderTEI {
			return "custom", nil
		}
		return "", nil
	}

	m := embeddingModelSelectModel{
		provider: provider,
		models:   models,
		cursor:   0,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running model selection: %w", err)
	}

	result := finalModel.(embeddingModelSelectModel)
	if result.quit {
		return "", fmt.Errorf("model selection cancelled")
	}

	return result.selectedID, nil
}

// --- Provider Selection Model ---

type embeddingProviderOption struct {
	ID          string
	Name        string
	Description string
}

type embeddingProviderSelectModel struct {
	options    []embeddingProviderOption
	cursor     int
	selectedID string
	quit       bool
}

func (m embeddingProviderSelectModel) Init() tea.Cmd { return nil }

func (m embeddingProviderSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m embeddingProviderSelectModel) View() string {
	s := "\n" + StyleSelectTitle.Render("📐 Select Embedding Provider") + "\n\n"

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

// --- Model Selection Model ---

type embeddingModelSelectModel struct {
	provider   string
	models     []llm.EmbeddingModelOption
	cursor     int
	selectedID string
	quit       bool
}

func (m embeddingModelSelectModel) Init() tea.Cmd { return nil }

func (m embeddingModelSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m embeddingModelSelectModel) View() string {
	s := "\n" + StyleSelectTitle.Render(fmt.Sprintf("📐 Select Embedding Model (%s)", m.provider)) + "\n\n"

	for i, model := range m.models {
		cursor := "  "
		style := StyleSelectNormal

		if m.cursor == i {
			cursor = "▶ "
			style = StyleSelectActive
		}

		line := fmt.Sprintf("%s%s", cursor, style.Render(fmt.Sprintf("%-24s", model.DisplayName)))
		if model.IsDefault {
			line += StyleSelectBadge.Render(" (default)")
		}
		line += StyleSelectDim.Render(" " + model.Info)
		s += line + "\n"
	}

	s += "\n" + StyleSelectDim.Render("↑/↓ navigate • enter select • esc cancel") + "\n"
	return s
}
