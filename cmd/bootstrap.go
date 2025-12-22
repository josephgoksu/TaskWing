/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/josephgoksu/TaskWing/internal/agents"
	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// bootstrapCmd represents the bootstrap command
var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Auto-generate project memory from existing repo",
	Long: `Scan your repository and automatically generate features and decisions.

The bootstrap command analyzes:
  • Directory structure → Detects features
  • Git history → Extracts decisions from conventional commits
  • LLM inference → Understands WHY decisions were made (default)

Examples:
  taskwing bootstrap --preview              # Preview with LLM analysis (requires OPENAI_API_KEY)
  taskwing bootstrap                        # Generate with parallel agent analysis
  taskwing bootstrap --legacy               # Use legacy single-LLM mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		preview, _ := cmd.Flags().GetBool("preview")
		legacy, _ := cmd.Flags().GetBool("legacy")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current directory: %w", err)
		}

		apiKey := viper.GetString("llm.apiKey")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return fmt.Errorf("OPENAI_API_KEY (or TASKWING_LLM_APIKEY) not set")
		}

		// Use legacy single-LLM mode if --legacy flag is set
		if legacy {
			return runLLMBootstrap(cmd.Context(), cwd, preview, apiKey)
		}

		// Default: use parallel agent architecture
		return runAgentBootstrap(cmd.Context(), cwd, preview, apiKey)
	},
}

// runAgentBootstrap uses the parallel agent architecture for analysis
func runAgentBootstrap(ctx context.Context, cwd string, preview bool, apiKey string) error {
	model := viper.GetString("llm.model")
	if model == "" {
		model = config.DefaultOpenAIModel
	}

	providerStr := viper.GetString("llm.provider")
	if providerStr == "" {
		providerStr = config.DefaultProvider
	}

	provider, err := llm.ValidateProvider(providerStr)
	if err != nil {
		return fmt.Errorf("invalid LLM provider: %w", err)
	}

	fmt.Println("")
	fmt.Println("┌──────────────────────────────────────────────────────────────┐")
	fmt.Println("│  🤖 TaskWing Agent Bootstrap (Experimental)                  │")
	fmt.Println("└──────────────────────────────────────────────────────────────┘")
	fmt.Println("")
	fmt.Printf("  ⚡ Using: %s (%s) with parallel agents\n", model, provider)

	llmCfg := llm.Config{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
		BaseURL:  viper.GetString("llm.baseURL"),
	}

	projectName := filepath.Base(cwd)

	// Create agents
	docAgent := agents.NewDocAgent(llmCfg)
	codeAgent := agents.NewReactCodeAgent(llmCfg, cwd)
	gitAgent := agents.NewGitAgent(llmCfg)
	depsAgent := agents.NewDepsAgent(llmCfg)

	agentsList := []agents.Agent{docAgent, codeAgent, gitAgent, depsAgent}

	// Prepare input
	input := agents.Input{
		BasePath:    cwd,
		ProjectName: projectName,
		Mode:        agents.ModeBootstrap,
		Verbose:     true, // Will be suppressed in TUI
	}

	// Run TUI
	tuiModel := ui.NewBootstrapModel(ctx, input, agentsList)
	p := tea.NewProgram(tuiModel)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	bootstrapModel, ok := finalModel.(ui.BootstrapModel)
	if !ok {
		return fmt.Errorf("internal error: invalid model type")
	}

	if bootstrapModel.Quitting && len(bootstrapModel.Results) < len(agentsList) {
		fmt.Println("\n⚠️  Bootstrap cancelled.")
		return nil
	}

	// Aggregate findings
	allFindings := agents.AggregateFindings(bootstrapModel.Results)

	// Render the dashboard summary instead of raw list
	renderBootstrapDashboard(allFindings)

	if preview || viper.GetBool("preview") {
		fmt.Println("\n💡 This was a preview. Run 'taskwing bootstrap' to save to memory.")
		return nil
	}

	// Prepare embedding config
	embeddingCfg := llm.Config{
		Provider: provider,
		APIKey:   apiKey,
		BaseURL:  viper.GetString("llm.baseURL"),
	}

	// Save to memory using refactored function
	return saveToMemory(ctx, allFindings, GetMemoryBasePath(), embeddingCfg)
}

// saveToMemory persists findings to the SQLite database
func saveToMemory(ctx context.Context, allFindings []agents.Finding, memoryPath string, embeddingCfg llm.Config) error {
	if len(allFindings) == 0 {
		fmt.Println("\n⚠️  No findings to save.")
		return nil
	}

	store, err := memory.NewSQLiteStore(memoryPath)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()
	ensureGitignore(memoryPath)

	if !viper.GetBool("quiet") {
		fmt.Print("  Generating embeddings")
	}

	nodesCreated := 0
	for _, f := range allFindings {
		content := f.Title + "\n" + f.Description
		if f.Why != "" {
			content += "\n\nWhy: " + f.Why
		}
		if f.Tradeoffs != "" {
			content += "\nTradeoffs: " + f.Tradeoffs
		}

		node := memory.Node{
			ID:          uuid.New().String(),
			Type:        string(f.Type),
			Summary:     f.Title,
			Content:     f.Description,
			SourceAgent: f.SourceAgent,
		}

		if f.Why != "" {
			node.Content += "\n\nWhy: " + f.Why
		}
		if f.Tradeoffs != "" {
			node.Content += "\nTradeoffs: " + f.Tradeoffs
		}

		// Generate embedding for semantic search
		// Only if API key is present
		if embeddingCfg.APIKey != "" {
			if embedding, err := knowledge.GenerateEmbedding(ctx, content, embeddingCfg); err == nil {
				node.Embedding = embedding
				if !viper.GetBool("quiet") {
					fmt.Print(".")
				}
			}
		}

		if err := store.CreateNode(node); err != nil {
			continue
		}
		nodesCreated++
	}

	if !viper.GetBool("quiet") {
		fmt.Println(" done")
	}

	// === STRUCTURED STORAGE (V2 Schema) ===
	if !viper.GetBool("quiet") {
		fmt.Print("  Saving structured data")
	}

	featuresCreated := 0
	decisionsCreated := 0
	patternsCreated := 0

	// 1. Process Features (from DocAgent)
	featureIDByName := make(map[string]string)

	// Load existing features to prevent duplicates
	existingFeatures, err := store.ListFeatures()
	if err == nil {
		for _, f := range existingFeatures {
			featureIDByName[strings.ToLower(f.Name)] = f.ID
		}
	}

	for _, f := range allFindings {
		if f.Type != agents.FindingTypeFeature {
			continue
		}

		name := strings.TrimSpace(f.Title)
		if name == "" {
			continue
		}

		key := strings.ToLower(name)
		if _, exists := featureIDByName[key]; exists {
			continue // Already exists
		}

		desc := f.Description
		if len(desc) > 200 {
			desc = desc[:197] + "..."
		}

		newID := "f-" + uuid.New().String()[:8]
		err := store.CreateFeature(memory.Feature{
			ID:        newID,
			Name:      name,
			OneLiner:  desc,
			Status:    memory.FeatureStatusActive,
			CreatedAt: time.Now(),
		})
		if err == nil {
			featureIDByName[key] = newID
			featuresCreated++
		}
	}

	// 2. Process Patterns (from ReactCodeAgent)
	for _, f := range allFindings {
		if f.Type != agents.FindingTypePattern {
			continue
		}

		name := strings.TrimSpace(f.Title)
		if name == "" {
			continue
		}

		// Extract fields from metadata
		context, _ := f.Metadata["context"].(string)
		solution, _ := f.Metadata["solution"].(string)
		consequences, _ := f.Metadata["consequences"].(string)

		err := store.CreatePattern(memory.Pattern{
			Name:         name,
			Context:      context,
			Solution:     solution,
			Consequences: consequences,
		})
		if err == nil {
			patternsCreated++
		}
	}

	// 3. Process Decisions (Smart Linking)
	for _, f := range allFindings {
		if f.Type != agents.FindingTypeDecision {
			continue
		}

		title := strings.TrimSpace(f.Title)
		if title == "" {
			continue
		}

		// Get component context from metadata
		componentName, _ := f.Metadata["component"].(string)
		componentName = strings.TrimSpace(componentName)

		// Smart Fallback: If no explicit component, infer from source agent
		if componentName == "" {
			switch f.SourceAgent {
			case "git":
				componentName = "Project Evolution"
			case "deps":
				componentName = "Technology Stack"
			case "doc_agent", "doc":
				componentName = "Documentation"
			default:
				componentName = "Core Architecture"
			}
		}

		// Find or Create Feature for Component
		featKey := strings.ToLower(componentName)
		featID := featureIDByName[featKey]

		if featID == "" {
			// Auto-create feature for discovered component
			featID = "f-" + uuid.New().String()[:8]
			err := store.CreateFeature(memory.Feature{
				ID:        featID,
				Name:      componentName,
				OneLiner:  "Auto-detected component from decision analysis",
				Status:    memory.FeatureStatusActive,
				CreatedAt: time.Now(),
			})
			if err != nil {
				continue
			}
			featureIDByName[featKey] = featID
			featuresCreated++
		}

		err := store.AddDecision(featID, memory.Decision{
			Title:     title,
			Summary:   f.Description,
			Reasoning: f.Why,
			Tradeoffs: f.Tradeoffs,
		})
		if err == nil {
			decisionsCreated++
		}
	}

	if !viper.GetBool("quiet") {
		fmt.Println(" done")
	}

	fmt.Printf("\n✅ Saved %d knowledge nodes, %d features, %d patterns, %d decisions to memory.\n",
		nodesCreated, featuresCreated, patternsCreated, decisionsCreated)
	return nil
}

func renderBootstrapDashboard(findings []agents.Finding) {
	var (
		headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		cardStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).Padding(0, 1).BorderForeground(lipgloss.Color("63")).MarginLeft(1)
		keyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // Pink
		valStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // White
		subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")) // Gray
	)

	// 1. Extract Stack
	var stack []string
	for _, f := range findings {
		if f.Type == agents.FindingTypeDependency {
			stack = append(stack, f.Title)
		}
	}
	// Limit stack items
	if len(stack) > 5 {
		stack = stack[:5]
	}
	stackStr := strings.Join(stack, " • ")
	if stackStr == "" {
		stackStr = "Detecting..."
	}

	// 2. Counts
	grouped := agents.GroupFindingsByType(findings)
	counts := fmt.Sprintf("🎯 %d Decisions • 📦 %d Features • 🧩 %d Patterns",
		len(grouped[agents.FindingTypeDecision]),
		len(grouped[agents.FindingTypeFeature]),
		len(grouped[agents.FindingTypePattern]))

	// Render "DNA" Summary
	fmt.Println()
	fmt.Println(headerStyle.Render(" 🧬 Project DNA"))
	dnaContent := fmt.Sprintf("%s\n%s",
		keyStyle.Render("Stack: ")+valStyle.Render(stackStr),
		keyStyle.Render("Scope: ")+valStyle.Render(counts),
	)
	fmt.Println(cardStyle.Render(dnaContent))
	fmt.Println()

	// 3. Highlights (Top 3 Decisions)
	var highlights []agents.Finding
	for _, f := range findings {
		if f.Type == agents.FindingTypeDecision && f.Why != "" {
			highlights = append(highlights, f)
		}
	}
	if len(highlights) > 3 {
		highlights = highlights[:3]
	}

	if len(highlights) > 0 {
		fmt.Println(headerStyle.Render(" 💡 Highlights"))

		for i, h := range highlights {
			title := h.Title
			why := h.Why
			if len(why) > 70 {
				why = why[:70] + "..."
			}

			fmt.Printf(" %d. %s\n", i+1, valStyle.Render(title))
			fmt.Printf("    %s\n", subtleStyle.Render(why))
		}
		fmt.Println()
	}
}

func runLLMBootstrap(ctx context.Context, cwd string, preview bool, apiKey string) error {
	model := viper.GetString("llm.model")
	if model == "" {
		model = config.DefaultOpenAIModel
	}

	providerStr := viper.GetString("llm.provider")
	if providerStr == "" {
		providerStr = config.DefaultProvider
	}

	provider, err := llm.ValidateProvider(providerStr)
	if err != nil {
		return fmt.Errorf("invalid LLM provider: %w", err)
	}

	if viper.GetBool("json") {
		fmt.Fprintln(os.Stderr, "Analyzing repository with LLM...")
		fmt.Fprintf(os.Stderr, "Provider: %s, Model: %s\n\n", provider, model)
	} else {
		// Value-focused header: tell users WHAT they'll get
		fmt.Println("")
		fmt.Println("┌──────────────────────────────────────────────────────────────┐")
		fmt.Println("│  🧠 TaskWing Bootstrap — Architectural Intelligence          │")
		fmt.Println("└──────────────────────────────────────────────────────────────┘")
		fmt.Println("")
		fmt.Println("  This will analyze your codebase and generate:")
		fmt.Println("  • Features with WHY they exist (not just what)")
		fmt.Println("  • Key decisions with trade-offs and reasoning")
		fmt.Println("  • Relationships between components")
		fmt.Println("")
		fmt.Printf("  ⚡ Using: %s (%s)\n", model, provider)
		fmt.Println("")
		fmt.Println("  Gathering context...")
	}

	llmCfg := llm.Config{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
		BaseURL:  viper.GetString("llm.baseURL"),
	}

	analyzer, err := bootstrap.NewLLMAnalyzer(ctx, cwd, llmCfg)
	if err != nil {
		return fmt.Errorf("create LLM analyzer: %w", err)
	}

	result, err := analyzer.Analyze(ctx)
	if err != nil {
		return fmt.Errorf("LLM analysis: %w", err)
	}

	if viper.GetBool("json") {
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal LLM analysis: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		result.PrintAnalysis()
	}

	if preview || viper.GetBool("preview") {
		if viper.GetBool("json") {
			fmt.Fprintln(os.Stderr, "\nThis was a preview. Run 'taskwing bootstrap' to save to memory.")
		} else {
			fmt.Println("\n💡 This was a preview. Run 'taskwing bootstrap' to save to memory.")
		}
		return nil
	}

	if len(result.Features) == 0 {
		if viper.GetBool("json") {
			fmt.Fprintln(os.Stderr, "\nNo features detected by LLM.")
		} else {
			fmt.Println("\n⚠️  No features detected by LLM.")
		}
		return nil
	}

	// Save to memory
	memoryPath := GetMemoryBasePath()
	store, err := memory.NewSQLiteStore(memoryPath)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()
	ensureGitignore(memoryPath)

	featuresCreated := 0
	decisionsCreated := 0

	existingFeatures, err := store.ListFeatures()
	if err != nil {
		return fmt.Errorf("list features: %w", err)
	}
	featureIDByName := make(map[string]string, len(existingFeatures))
	for _, f := range existingFeatures {
		key := strings.ToLower(strings.TrimSpace(f.Name))
		if key == "" {
			continue
		}
		featureIDByName[key] = f.ID
	}

	for _, f := range result.Features {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			continue
		}

		featureKey := strings.ToLower(name)
		featureID := featureIDByName[featureKey]

		if featureID == "" {
			oneLiner := strings.TrimSpace(f.Description)
			if oneLiner == "" {
				oneLiner = strings.TrimSpace(f.Purpose)
			}
			if oneLiner == "" {
				oneLiner = "Auto-generated feature"
			}

			featureID = "f-" + uuid.New().String()[:8]
			createErr := store.CreateFeature(memory.Feature{
				ID:       featureID,
				Name:     name,
				OneLiner: oneLiner,
				Status:   memory.FeatureStatusActive,
			})
			if createErr != nil {
				// Refresh feature map once; if the feature already exists, attach decisions to it.
				features, listErr := store.ListFeatures()
				if listErr != nil {
					continue
				}
				featureIDByName = make(map[string]string, len(features))
				for _, stored := range features {
					k := strings.ToLower(strings.TrimSpace(stored.Name))
					if k == "" {
						continue
					}
					featureIDByName[k] = stored.ID
				}
				featureID = featureIDByName[featureKey]
				if featureID == "" {
					continue
				}
			} else {
				featuresCreated++
				featureIDByName[featureKey] = featureID
			}
		}

		existingDecisionTitles := make(map[string]struct{})
		if existingDecisions, err := store.GetDecisions(featureID); err == nil {
			for _, d := range existingDecisions {
				titleKey := strings.ToLower(strings.TrimSpace(d.Title))
				if titleKey == "" {
					continue
				}
				existingDecisionTitles[titleKey] = struct{}{}
			}
		}

		// Add decisions
		for _, d := range f.Decisions {
			title := strings.TrimSpace(d.Title)
			if title == "" {
				continue
			}
			titleKey := strings.ToLower(title)
			if _, ok := existingDecisionTitles[titleKey]; ok {
				continue
			}

			summary := strings.TrimSpace(d.Why)
			if summary == "" {
				summary = title
			}

			err := store.AddDecision(featureID, memory.Decision{
				Title:     title,
				Summary:   summary,
				Reasoning: summary,
				Tradeoffs: d.Tradeoffs,
			})
			if err == nil {
				decisionsCreated++
				existingDecisionTitles[titleKey] = struct{}{}
			}
		}
	}

	// Second pass: create edges from LLM-inferred relationships
	// (Must happen after all features are created so we can resolve names to IDs)
	edgesCreated := 0
	for _, f := range result.Features {
		fromName := strings.TrimSpace(f.Name)
		if fromName == "" {
			continue
		}
		fromID := featureIDByName[strings.ToLower(fromName)]
		if fromID == "" {
			continue
		}

		// Process depends_on relationships
		for _, dep := range f.DependsOn {
			toID := featureIDByName[strings.ToLower(strings.TrimSpace(dep))]
			if toID != "" && toID != fromID {
				if err := store.Link(fromID, toID, memory.EdgeTypeDependsOn); err == nil {
					edgesCreated++
				}
			}
		}

		// Process extends relationships
		for _, ext := range f.Extends {
			toID := featureIDByName[strings.ToLower(strings.TrimSpace(ext))]
			if toID != "" && toID != fromID {
				if err := store.Link(fromID, toID, memory.EdgeTypeExtends); err == nil {
					edgesCreated++
				}
			}
		}

		// Process related_to relationships
		for _, rel := range f.RelatedTo {
			toID := featureIDByName[strings.ToLower(strings.TrimSpace(rel))]
			if toID != "" && toID != fromID {
				if err := store.Link(fromID, toID, memory.EdgeTypeRelated); err == nil {
					edgesCreated++
				}
			}
		}
	}

	// === DUAL-WRITE: Create nodes for new knowledge graph ===
	// This runs alongside legacy feature/decision creation during migration phase
	nodesCreatedLines := 0
	embeddingsGenerated := 0
	nodesSkipped := 0

	// Semantic deduplication - tracks embeddings for cosine similarity comparison
	// If a new node's embedding is > 0.92 similar to an existing one, skip it
	seenNodeSummaries := make(map[string]struct{})
	var seenEmbeddings [][]float32
	const similarityThreshold = 0.92

	// Prepare embedding config
	embeddingCfg := llm.Config{
		Provider: provider,
		APIKey:   apiKey,
		BaseURL:  viper.GetString("llm.baseURL"),
	}

	if !viper.GetBool("quiet") && !viper.GetBool("json") {
		fmt.Print("  Generating knowledge nodes")
	}

	for _, f := range result.Features {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			continue
		}

		// Create feature node
		oneLiner := strings.TrimSpace(f.Description)
		if oneLiner == "" {
			oneLiner = strings.TrimSpace(f.Purpose)
		}
		content := fmt.Sprintf("%s: %s", name, oneLiner)

		// String-based dedup as first pass (fast)
		summaryKey := strings.ToLower(name)
		if _, seen := seenNodeSummaries[summaryKey]; seen {
			nodesSkipped++
			continue
		}
		seenNodeSummaries[summaryKey] = struct{}{}

		node := memory.Node{
			Content: content,
			Type:    memory.NodeTypeFeature,
			Summary: name,
		}

		// Generate embedding
		if apiKey != "" {
			if embedding, err := knowledge.GenerateEmbedding(ctx, content, embeddingCfg); err == nil {
				node.Embedding = embedding
				embeddingsGenerated++

				// Semantic dedup: skip if embedding is too similar to an existing one
				if isSemanticallyDuplicate(embedding, seenEmbeddings, similarityThreshold) {
					nodesSkipped++
					continue
				}
				seenEmbeddings = append(seenEmbeddings, embedding)
			}
		}

		if err := store.CreateNode(node); err == nil {
			nodesCreatedLines++
			if !viper.GetBool("quiet") && !viper.GetBool("json") {
				fmt.Print(".")
			}
		}

		// Create decision nodes
		for _, d := range f.Decisions {
			title := strings.TrimSpace(d.Title)
			if title == "" {
				continue
			}

			why := strings.TrimSpace(d.Why)
			tradeoffs := strings.TrimSpace(d.Tradeoffs)
			decisionContent := title
			if why != "" {
				decisionContent += ". Why: " + why
			}
			if tradeoffs != "" {
				decisionContent += ". Trade-offs: " + tradeoffs
			}

			// Dedup: skip if we've seen this decision title (string-based first pass)
			decisionKey := strings.ToLower(title)
			if _, seen := seenNodeSummaries[decisionKey]; seen {
				nodesSkipped++
				continue
			}
			seenNodeSummaries[decisionKey] = struct{}{}

			decisionNode := memory.Node{
				Content: decisionContent,
				Type:    memory.NodeTypeDecision,
				Summary: title,
			}

			// Generate embedding
			if apiKey != "" {
				if embedding, err := knowledge.GenerateEmbedding(ctx, decisionContent, embeddingCfg); err == nil {
					decisionNode.Embedding = embedding
					embeddingsGenerated++

					// Semantic dedup: skip if embedding is too similar to an existing one
					if isSemanticallyDuplicate(embedding, seenEmbeddings, similarityThreshold) {
						nodesSkipped++
						continue
					}
					seenEmbeddings = append(seenEmbeddings, embedding)
				}
			}

			if err := store.CreateNode(decisionNode); err == nil {
				nodesCreatedLines++
				if !viper.GetBool("quiet") && !viper.GetBool("json") {
					fmt.Print(".")
				}
			}
		}
	}

	if !viper.GetBool("quiet") && !viper.GetBool("json") {
		fmt.Println(" done")
	}

	if viper.GetBool("json") {
		fmt.Fprintf(os.Stderr, "\nBootstrap complete:\n")
		fmt.Fprintf(os.Stderr, "  Features created: %d\n", featuresCreated)
		fmt.Fprintf(os.Stderr, "  Decisions created: %d\n", decisionsCreated)
		fmt.Fprintf(os.Stderr, "  Relationships created: %d\n", edgesCreated)
		fmt.Fprintf(os.Stderr, "  Knowledge nodes created: %d (skipped %d duplicates)\n", nodesCreatedLines, nodesSkipped)
	} else {
		fmt.Printf("\n✓ Bootstrap complete:\n")
		fmt.Printf("  • Features created: %d\n", featuresCreated)
		fmt.Printf("  • Decisions created: %d\n", decisionsCreated)
		fmt.Printf("  • Relationships created: %d\n", edgesCreated)
		fmt.Printf("  • Knowledge nodes created: %d (skipped %d duplicates)\n", nodesCreatedLines, nodesSkipped)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
	bootstrapCmd.Flags().Bool("preview", false, "Preview what would be generated without saving")
	bootstrapCmd.Flags().Bool("legacy", false, "Use legacy single-LLM bootstrap (default: parallel agents)")
}

// ensureGitignore creates .gitignore in the memory directory if it doesn't exist
func ensureGitignore(memoryPath string) {
	gitignorePath := memoryPath + "/.gitignore"
	if _, err := os.Stat(gitignorePath); err == nil {
		return // already exists
	}
	gitignoreContent := `# TaskWing generated/cache files
memory.db-journal
memory.db-wal
memory.db-shm
index.json
`
	os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
}

// cosineSimilarity computes cosine similarity between two embedding vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (sqrt32(normA) * sqrt32(normB))
}

// sqrt32 returns the square root of a float32
func sqrt32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}

// isSemanticallyDuplicate checks if an embedding is too similar to any existing embeddings
func isSemanticallyDuplicate(embedding []float32, existing [][]float32, threshold float32) bool {
	for _, e := range existing {
		if cosineSimilarity(embedding, e) > threshold {
			return true
		}
	}
	return false
}
