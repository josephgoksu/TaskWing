/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

// findCmd represents the find command
var findCmd = &cobra.Command{
	Use:   "find [query]",
	Short: "Search for code symbols",
	Long: `Search for code symbols using hybrid semantic and lexical matching.

Without arguments, starts an interactive search interface.
With a query argument, performs a one-shot search.

Examples:
  taskwing find                    # Interactive mode
  taskwing find "CreateUser"       # Search for symbols matching "CreateUser"
  taskwing find --kind function    # Filter to functions only
  taskwing find --file user.go     # Filter to specific file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFind,
}

var (
	findKind  string
	findFile  string
	findLimit int
)

func init() {
	rootCmd.AddCommand(findCmd)
	findCmd.Flags().StringVar(&findKind, "kind", "", "Filter by symbol kind (function, struct, interface, method)")
	findCmd.Flags().StringVar(&findFile, "file", "", "Filter by file path")
	findCmd.Flags().IntVar(&findLimit, "limit", 20, "Maximum number of results")
}

func runFind(cmd *cobra.Command, args []string) error {
	// Initialize repository
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	appCtx := app.NewContext(repo)
	codeIntelApp := app.NewCodeIntelApp(appCtx)

	// Check if index exists
	ctx := context.Background()
	stats, err := codeIntelApp.GetStats(ctx)
	if err != nil || stats.SymbolsFound == 0 {
		fmt.Println("⚠️  No symbols indexed. Run 'tw index' first to index your codebase.")
		return nil
	}

	// If query provided, do one-shot search
	if len(args) > 0 {
		return runFindOneShot(ctx, codeIntelApp, args[0])
	}

	// Otherwise, run interactive mode
	if isJSON() {
		return fmt.Errorf("interactive mode not supported with --json flag; provide a query argument")
	}

	return runFindInteractive(ctx, codeIntelApp)
}

func runFindOneShot(ctx context.Context, codeIntelApp *app.CodeIntelApp, query string) error {
	// Render header
	if !isJSON() && !isQuiet() {
		ui.RenderPageHeader("TaskWing Find", fmt.Sprintf("Query: \"%s\"", query))
	}

	if !isQuiet() {
		fmt.Fprint(os.Stderr, "🔍 Searching...")
	}

	result, err := codeIntelApp.SearchCode(ctx, app.SearchCodeOptions{
		Query:    query,
		Limit:    findLimit,
		Kind:     codeintel.SymbolKind(findKind),
		FilePath: findFile,
	})
	if err != nil {
		if !isQuiet() {
			fmt.Fprintln(os.Stderr, " failed")
		}
		return fmt.Errorf("search failed: %v", err)
	}

	if !isQuiet() {
		fmt.Fprintln(os.Stderr, " done")
	}

	if isJSON() {
		return printJSON(result)
	}

	if result.Count == 0 {
		fmt.Println("No symbols found matching your query.")
		fmt.Println("Try a different query or run 'tw index' to refresh the index.")
		return nil
	}

	// Render results
	fmt.Printf("\n📊 Found %d symbols:\n\n", result.Count)

	for i, r := range result.Results {
		sym := r.Symbol
		kindIcon := symbolKindIcon(sym.Kind)
		location := fmt.Sprintf("%s:%d", sym.FilePath, sym.StartLine)

		// Symbol name and kind
		fmt.Printf("%d. %s %s %s\n",
			i+1,
			kindIcon,
			ui.StyleTitle.Render(sym.Name),
			ui.StyleSubtle.Render(fmt.Sprintf("(%s)", sym.Kind)),
		)

		// Signature if available
		if sym.Signature != "" {
			fmt.Printf("   %s\n", ui.StyleSubtle.Render(sym.Signature))
		}

		// Location and score
		fmt.Printf("   📍 %s  ", ui.StyleSubtle.Render(location))
		fmt.Printf("📊 %.2f (%s)\n", r.Score, r.Source)

		// Doc comment if available
		if sym.DocComment != "" && isVerbose() {
			doc := strings.TrimSpace(sym.DocComment)
			if len(doc) > 100 {
				doc = doc[:97] + "..."
			}
			fmt.Printf("   📝 %s\n", ui.StyleSubtle.Render(doc))
		}

		fmt.Println()
	}

	return nil
}

// === Interactive TUI ===

type findModel struct {
	textInput    textinput.Model
	list         list.Model
	codeIntelApp *app.CodeIntelApp
	ctx          context.Context
	query        string
	results      []codeintel.SymbolSearchResult
	err          error
	searching    bool
	selected     *codeintel.Symbol
	width        int
	height       int
}

type symbolItem struct {
	symbol codeintel.SymbolSearchResult
}

func (i symbolItem) Title() string {
	return fmt.Sprintf("%s %s", symbolKindIcon(i.symbol.Symbol.Kind), i.symbol.Symbol.Name)
}

func (i symbolItem) Description() string {
	return fmt.Sprintf("%s:%d (%.2f)", i.symbol.Symbol.FilePath, i.symbol.Symbol.StartLine, i.symbol.Score)
}

func (i symbolItem) FilterValue() string {
	return i.symbol.Symbol.Name
}

type searchResultsMsg struct {
	results []codeintel.SymbolSearchResult
	err     error
}

func runFindInteractive(ctx context.Context, codeIntelApp *app.CodeIntelApp) error {
	ti := textinput.New()
	ti.Placeholder = "Enter search query..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 80, 15)
	l.Title = "Symbol Search"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)

	m := findModel{
		textInput:    ti,
		list:         l,
		codeIntelApp: codeIntelApp,
		ctx:          ctx,
		width:        80,
		height:       24,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Show selected symbol details after exit
	if fm, ok := finalModel.(findModel); ok && fm.selected != nil {
		fmt.Println()
		fmt.Println("Selected symbol:")
		renderSymbolDetails(fm.selected)
	}

	return nil
}

func (m findModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m findModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.textInput.Focused() {
				m.query = m.textInput.Value()
				if m.query != "" {
					m.searching = true
					return m, m.doSearch
				}
			} else {
				// Select item from list
				if item, ok := m.list.SelectedItem().(symbolItem); ok {
					m.selected = &item.symbol.Symbol
					return m, tea.Quit
				}
			}
		case "tab":
			if m.textInput.Focused() && len(m.results) > 0 {
				m.textInput.Blur()
				m.list.SetDelegate(list.NewDefaultDelegate())
			} else {
				m.textInput.Focus()
			}
		case "esc":
			if !m.textInput.Focused() {
				m.textInput.Focus()
			} else {
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-10)

	case searchResultsMsg:
		m.searching = false
		m.err = msg.err
		m.results = msg.results

		// Update list items
		items := make([]list.Item, len(msg.results))
		for i, r := range msg.results {
			items[i] = symbolItem{symbol: r}
		}
		m.list.SetItems(items)
	}

	// Update components
	var cmd tea.Cmd
	if m.textInput.Focused() {
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m findModel) doSearch() tea.Msg {
	result, err := m.codeIntelApp.SearchCode(m.ctx, app.SearchCodeOptions{
		Query: m.query,
		Limit: findLimit,
		Kind:  codeintel.SymbolKind(findKind),
	})
	if err != nil {
		return searchResultsMsg{err: err}
	}
	return searchResultsMsg{results: result.Results}
}

func (m findModel) View() string {
	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("🔍 TaskWing Symbol Search")
	b.WriteString(header)
	b.WriteString("\n\n")

	// Search input
	inputStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(0, 1)
	b.WriteString(inputStyle.Render(m.textInput.View()))
	b.WriteString("\n\n")

	// Status
	if m.searching {
		b.WriteString("⏳ Searching...\n\n")
	} else if m.err != nil {
		b.WriteString(fmt.Sprintf("❌ Error: %v\n\n", m.err))
	} else if len(m.results) > 0 {
		b.WriteString(fmt.Sprintf("📊 Found %d symbols (Tab to navigate, Enter to select)\n\n", len(m.results)))
		b.WriteString(m.list.View())
	} else if m.query != "" {
		b.WriteString("No symbols found.\n")
	}

	// Footer
	b.WriteString("\n")
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"Enter: search/select • Tab: toggle focus • q/Esc: quit",
	)
	b.WriteString(footer)

	return b.String()
}

func symbolKindIcon(kind codeintel.SymbolKind) string {
	switch kind {
	case codeintel.SymbolFunction:
		return "ƒ"
	case codeintel.SymbolMethod:
		return "m"
	case codeintel.SymbolStruct:
		return "S"
	case codeintel.SymbolInterface:
		return "I"
	case codeintel.SymbolType:
		return "T"
	case codeintel.SymbolVariable:
		return "v"
	case codeintel.SymbolConstant:
		return "c"
	case codeintel.SymbolField:
		return "."
	case codeintel.SymbolPackage:
		return "P"
	default:
		return "?"
	}
}

func renderSymbolDetails(sym *codeintel.Symbol) {
	fmt.Printf("  Name:      %s\n", ui.StyleTitle.Render(sym.Name))
	fmt.Printf("  Kind:      %s\n", sym.Kind)
	fmt.Printf("  Location:  %s:%d-%d\n", sym.FilePath, sym.StartLine, sym.EndLine)
	if sym.Signature != "" {
		fmt.Printf("  Signature: %s\n", sym.Signature)
	}
	if sym.ModulePath != "" {
		fmt.Printf("  Module:    %s\n", sym.ModulePath)
	}
	if sym.DocComment != "" {
		doc := strings.TrimSpace(sym.DocComment)
		if len(doc) > 200 {
			doc = doc[:197] + "..."
		}
		fmt.Printf("  Doc:       %s\n", doc)
	}
}
