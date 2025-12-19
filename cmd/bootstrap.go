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
	"strings"

	"github.com/google/uuid"
	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
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
  taskwing bootstrap                        # Generate with LLM analysis
  taskwing bootstrap --basic --preview      # Preview heuristic scan (no LLM calls)
  taskwing bootstrap --basic                # Save heuristic scan`,
	RunE: func(cmd *cobra.Command, args []string) error {
		preview, _ := cmd.Flags().GetBool("preview")
		basic, _ := cmd.Flags().GetBool("basic")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current directory: %w", err)
		}

		if basic {
			return runBasicBootstrap(cwd, preview)
		}

		apiKey := viper.GetString("llm.apiKey")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			if viper.GetBool("json") {
				fmt.Fprintln(os.Stderr, "OPENAI_API_KEY (or TASKWING_LLM_APIKEY) not set; running heuristic scan (--basic).")
			} else {
				fmt.Println("⚠️  OPENAI_API_KEY not set; running heuristic scan (use --basic to silence this).")
			}
			return runBasicBootstrap(cwd, preview)
		}

		return runLLMBootstrap(cmd.Context(), cwd, preview, apiKey)
	},
}

func runLLMBootstrap(ctx context.Context, cwd string, preview bool, apiKey string) error {
	model := viper.GetString("llm.modelName")
	if model == "" {
		model = DefaultLLMModel
	}

	providerStr := viper.GetString("llm.provider")
	if providerStr == "" {
		providerStr = "openai"
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
	nodesCreated := 0
	embeddingsGenerated := 0
	nodesSkipped := 0

	// Semantic deduplication - tracks embeddings for cosine similarity comparison
	// If a new node's embedding is > 0.92 similar to an existing one, skip it
	seenNodeSummaries := make(map[string]struct{})
	var seenEmbeddings [][]float32
	const similarityThreshold = 0.92

	// Prepare embedding config
	embeddingCfg := llm.Config{APIKey: apiKey}

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
			nodesCreated++
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
				nodesCreated++
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
		fmt.Fprintf(os.Stderr, "  Knowledge nodes created: %d (skipped %d duplicates)\n", nodesCreated, nodesSkipped)
	} else {
		fmt.Printf("\n✓ Bootstrap complete:\n")
		fmt.Printf("  • Features created: %d\n", featuresCreated)
		fmt.Printf("  • Decisions created: %d\n", decisionsCreated)
		fmt.Printf("  • Relationships created: %d\n", edgesCreated)
		fmt.Printf("  • Knowledge nodes created: %d (skipped %d duplicates)\n", nodesCreated, nodesSkipped)
	}

	return nil
}

func runBasicBootstrap(cwd string, preview bool) error {
	if viper.GetBool("json") {
		fmt.Fprintln(os.Stderr, "Scanning repository...")
	} else {
		fmt.Println("🔍 Scanning repository...")
	}

	scanner := bootstrap.NewScanner(cwd)
	result, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	if viper.GetBool("json") {
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal scan result: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		result.PrintPreview()
	}

	if preview || viper.GetBool("preview") {
		if viper.GetBool("json") {
			fmt.Fprintln(os.Stderr, "\nThis was a preview. Run 'taskwing bootstrap' to save to memory.")
			fmt.Fprintln(os.Stderr, "Tip: Run without --basic for LLM-powered analysis.")
		} else {
			fmt.Println("\n💡 This was a preview. Run 'taskwing bootstrap' to save to memory.")
			fmt.Println("   Tip: Run without --basic for LLM-powered analysis.")
		}
		return nil
	}

	if len(result.Features) == 0 && len(result.Decisions) == 0 {
		if viper.GetBool("json") {
			fmt.Fprintln(os.Stderr, "\nNothing to save. No features or decisions detected.")
			fmt.Fprintln(os.Stderr, "Tip: Run without --basic for LLM-powered analysis.")
		} else {
			fmt.Println("\n⚠️  Nothing to save. No features or decisions detected.")
			fmt.Println("   Tip: Run without --basic for LLM-powered analysis.")
		}
		return nil
	}

	memoryPath := GetMemoryBasePath()
	store, err := memory.NewSQLiteStore(memoryPath)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()
	ensureGitignore(memoryPath)

	featuresCreated := 0
	for _, f := range result.Features {
		err := store.CreateFeature(memory.Feature{
			Name:     f.Name,
			OneLiner: f.OneLiner,
			Status:   memory.FeatureStatusActive,
		})
		if err != nil {
			continue
		}
		featuresCreated++
	}

	allFeatures, err := store.ListFeatures()
	if err != nil {
		return fmt.Errorf("list features: %w", err)
	}
	featureIDByName := make(map[string]string, len(allFeatures))
	for _, f := range allFeatures {
		key := strings.ToLower(strings.TrimSpace(f.Name))
		if key == "" {
			continue
		}
		featureIDByName[key] = f.ID
	}

	decisionsCreated := 0
	for _, d := range result.Decisions {
		featureKey := strings.ToLower(strings.TrimSpace(d.Feature))
		featureID := featureIDByName[featureKey]
		if featureID == "" {
			continue
		}

		err := store.AddDecision(featureID, memory.Decision{
			Title:     d.Title,
			Summary:   d.Reasoning,
			Reasoning: d.Reasoning,
		})
		if err == nil {
			decisionsCreated++
		}
	}

	if viper.GetBool("json") {
		fmt.Fprintf(os.Stderr, "\nBootstrap complete:\n")
		fmt.Fprintf(os.Stderr, "  Features created: %d\n", featuresCreated)
		fmt.Fprintf(os.Stderr, "  Decisions created: %d\n", decisionsCreated)
	} else {
		fmt.Printf("\n✓ Bootstrap complete:\n")
		fmt.Printf("  • Features created: %d\n", featuresCreated)
		fmt.Printf("  • Decisions created: %d\n", decisionsCreated)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
	bootstrapCmd.Flags().Bool("preview", false, "Preview what would be generated without saving")
	bootstrapCmd.Flags().Bool("basic", false, "Use heuristic scan only (no LLM calls)")
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
