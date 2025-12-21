package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/josephgoksu/TaskWing/internal/agents"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/viper"
)

// RunAPI runs the bootstrap process for the API with optional agent filtering.
// It avoids TUI and uses internal logic directly.
func RunAPI(ctx context.Context, cwd string, preview bool, apiKey string, agentFilter []string, memoryPath string) error {
	fmt.Printf("[API Bootstrap] Running with agent filter: %v\n", agentFilter)

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

	llmCfg := llm.Config{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
		BaseURL:  viper.GetString("llm.baseURL"),
	}

	projectName := filepath.Base(cwd)

	// Create all agents
	allAgents := map[string]agents.Agent{
		"doc":  agents.NewDocAgent(llmCfg),
		"code": agents.NewReactCodeAgent(llmCfg, cwd),
		"git":  agents.NewGitAgent(llmCfg),
		"deps": agents.NewDepsAgent(llmCfg),
	}

	// Filter agents if specified
	var agentsList []agents.Agent
	if len(agentFilter) > 0 {
		for _, id := range agentFilter {
			if agent, ok := allAgents[id]; ok {
				agentsList = append(agentsList, agent)
			}
		}
	} else {
		// Run all agents if no filter
		agentsList = []agents.Agent{allAgents["doc"], allAgents["code"], allAgents["git"], allAgents["deps"]}
	}

	if len(agentsList) == 0 {
		return fmt.Errorf("no valid agents specified")
	}

	// Prepare input
	input := agents.Input{
		BasePath:    cwd,
		ProjectName: projectName,
		Mode:        agents.ModeBootstrap,
		Verbose:     false,
	}

	// Run orchestrator directly (no TUI)
	orchestrator := agents.NewOrchestrator(agentsList, false)
	results, err := orchestrator.RunAll(ctx, input)
	if err != nil {
		return fmt.Errorf("orchestrator error: %w", err)
	}

	// Aggregate findings
	allFindings := agents.AggregateFindings(results)

	if preview {
		return nil
	}

	// Save to memory
	if len(allFindings) == 0 {
		return nil
	}

	store, err := memory.NewSQLiteStore(memoryPath)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()
	ensureGitignore(memoryPath)

	// Delete existing nodes for filtered agents (agent-level replace strategy)
	for _, agent := range agentsList {
		if err := store.DeleteNodesByAgent(agent.Name()); err != nil {
			fmt.Printf("   ⚠️ Warning: failed to delete old nodes for %s: %v\n", agent.Name(), err)
		}
	}

	// Prepare embedding config
	embeddingCfg := llm.Config{
		Provider: provider,
		APIKey:   apiKey,
		BaseURL:  viper.GetString("llm.baseURL"),
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
		if embedding, err := knowledge.GenerateEmbedding(ctx, content, embeddingCfg); err == nil {
			node.Embedding = embedding
		}

		if err := store.CreateNode(node); err != nil {
			continue
		}
		nodesCreated++
	}

	return nil
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
