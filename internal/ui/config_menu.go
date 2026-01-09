package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/viper"
)

// ConfigItem represents a configurable setting
type ConfigItem struct {
	ID          string // internal key
	Label       string // display name
	Value       string // current value (provider/model)
	Status      string // key status emoji
	Description string // help text
}

// ConfigMenuResult contains the user's selection
type ConfigMenuResult struct {
	Action   string // "edit", "quit"
	Selected string // ID of selected item
}

// RunConfigMenu displays the unified configuration menu
func RunConfigMenu() (*ConfigMenuResult, error) {
	items := buildConfigItems()

	m := configMenuModel{
		items:  items,
		cursor: 0,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("config menu error: %w", err)
	}

	result := finalModel.(configMenuModel)
	if result.quit {
		return &ConfigMenuResult{Action: "quit"}, nil
	}

	return &ConfigMenuResult{
		Action:   "edit",
		Selected: result.items[result.cursor].ID,
	}, nil
}

func buildConfigItems() []ConfigItem {
	cfg, err := config.LoadLLMConfig()
	if err != nil {
		// Log error but continue with defaults - don't crash the menu
		fmt.Fprintf(os.Stderr, "Warning: failed to load LLM config: %v\n", err)
	}

	// Bootstrap model (complex tasks)
	bootstrapSpec := viper.GetString("llm.models.bootstrap")
	bootstrapValue := fmt.Sprintf("%s/%s", cfg.Provider, cfg.Model)
	bootstrapStatus := getKeyStatus(string(cfg.Provider))
	if bootstrapSpec != "" {
		// Normalize to slash format for consistent display
		normalized := strings.Replace(bootstrapSpec, ":", "/", 1)
		parts := strings.SplitN(normalized, "/", 2)
		if len(parts) == 2 {
			bootstrapValue = normalized // Use normalized (slash) format
			bootstrapStatus = getKeyStatus(parts[0])
		}
	}

	// Query model (fast)
	querySpec := viper.GetString("llm.models.query")
	queryValue := "(uses default)"
	queryStatus := ""
	if querySpec != "" {
		queryValue = strings.Replace(querySpec, ":", "/", 1)
		parts := strings.SplitN(queryValue, "/", 2)
		if len(parts) >= 1 {
			queryStatus = getKeyStatus(parts[0])
		}
	}

	// Embeddings - read directly from viper to catch recent changes
	embProviderStr := viper.GetString("llm.embedding_provider")
	embModel := viper.GetString("llm.embedding_model")
	if embModel == "" {
		embModel = cfg.EmbeddingModel // Fallback to loaded config
	}
	embValue := "(not configured)"
	embStatus := ""
	if embProviderStr != "" {
		embValue = fmt.Sprintf("%s/%s", embProviderStr, embModel)
		embStatus = getKeyStatus(embProviderStr)
	} else if cfg.Provider != "" {
		embValue = fmt.Sprintf("(uses %s)", cfg.Provider)
		embStatus = getKeyStatus(string(cfg.Provider))
	}

	// Reranking
	rerankEnabled := viper.GetBool("retrieval.reranking.enabled")
	rerankValue := "disabled"
	rerankStatus := ""
	if rerankEnabled {
		rerankURL := viper.GetString("retrieval.reranking.base_url")
		if rerankURL != "" {
			rerankValue = rerankURL
			rerankStatus = "🔌"
		}
	}

	return []ConfigItem{
		{
			ID:          "bootstrap",
			Label:       "Complex Tasks",
			Value:       bootstrapValue,
			Status:      bootstrapStatus,
			Description: "Used for bootstrap, planning (expensive, capable)",
		},
		{
			ID:          "query",
			Label:       "Fast Queries",
			Value:       queryValue,
			Status:      queryStatus,
			Description: "Used for context lookups (cheap, fast)",
		},
		{
			ID:          "embedding",
			Label:       "Embeddings",
			Value:       embValue,
			Status:      embStatus,
			Description: "Used for semantic search in knowledge base",
		},
		{
			ID:          "reranking",
			Label:       "Reranking",
			Value:       rerankValue,
			Status:      rerankStatus,
			Description: "Optional: improves search result quality",
		},
	}
}

func getKeyStatus(provider string) string {
	provider = strings.ToLower(provider)
	switch provider {
	case "ollama", "tei":
		return "🏠" // local
	default:
		if config.ResolveAPIKey(llm.Provider(provider)) != "" {
			return "✅" // key set
		}
		return "❌" // key missing
	}
}

// --- Config Menu Model ---

type configMenuModel struct {
	items  []ConfigItem
	cursor int
	quit   bool
}

func (m configMenuModel) Init() tea.Cmd { return nil }

func (m configMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

var (
	configTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))

	configActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86"))

	configDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	configValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229"))
)

func (m configMenuModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(configTitleStyle.Render("TaskWing Configuration"))
	b.WriteString("\n")
	b.WriteString(configDimStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	b.WriteString("\n\n")

	for i, item := range m.items {
		cursor := "  "
		labelStyle := configDimStyle
		valueStyle := configDimStyle

		if m.cursor == i {
			cursor = "▶ "
			labelStyle = configActiveStyle
			valueStyle = configValueStyle
		}

		// Format: cursor Label          value    status
		label := fmt.Sprintf("%-14s", item.Label)
		value := fmt.Sprintf("%-35s", item.Value)

		line := fmt.Sprintf("%s%s %s %s",
			cursor,
			labelStyle.Render(label),
			valueStyle.Render(value),
			item.Status,
		)
		b.WriteString(line)
		b.WriteString("\n")

		// Show description for selected item
		if m.cursor == i {
			b.WriteString(configDimStyle.Render(fmt.Sprintf("     %s", item.Description)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(configDimStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	b.WriteString("\n")
	b.WriteString(configDimStyle.Render("↑/↓ navigate • enter configure • q quit"))
	b.WriteString("\n")

	return b.String()
}
