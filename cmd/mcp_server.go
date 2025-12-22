/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/spec"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// mcpCmd represents the mcp command
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI tool integration",
	Long: `Start a Model Context Protocol (MCP) server to enable AI tools like Claude Code,
Cursor, and other AI assistants to interact with TaskWing project memory.

The MCP server provides the project-context tool that gives AI tools access to:
- Knowledge nodes (decisions, features, plans, notes)
- Semantic search across project memory
- Relationships between components

Example usage with Claude Code:
  taskwing mcp

The server will run until the client disconnects.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If arguments are provided but no subcommand was matched by Cobra,
		// it might mean an invalid subcommand or argument.
		// However, to maintain "taskwing mcp" as the way to start the server,
		// we only error if it looks like a subcommand attempt.
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q\nRun '%s --help' for usage", args[0], cmd.CommandPath(), cmd.Root().Name())
		}
		return runMCPServer(cmd.Context())
	},
}

var mcpPort int

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().IntVar(&mcpPort, "port", 0, "Port for SSE transport (0 = stdio)")
}

// ProjectContextParams defines the parameters for the project-context tool
type ProjectContextParams struct {
	Query string `json:"query,omitempty"`
}

func runMCPServer(ctx context.Context) error {
	fmt.Fprintf(os.Stderr, "\n🎯 TaskWing MCP Server Starting...\n")
	fmt.Fprintf(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Fprintf(os.Stderr, "Knowledge Graph for Engineering Teams\n")
	fmt.Fprintf(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Get current working directory
	cwd, _ := os.Getwd()

	// Initialize memory store
	store, err := memory.NewSQLiteStore(config.GetMemoryBasePath())
	if err != nil {
		return fmt.Errorf("failed to initialize memory store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Create MCP server
	impl := &mcpsdk.Implementation{
		Name:    "taskwing",
		Version: version,
	}

	serverOpts := &mcpsdk.ServerOptions{
		InitializedHandler: func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.InitializedParams) {
			fmt.Fprintf(os.Stderr, "✓ MCP connection established\n")
			if viper.GetBool("verbose") {
				fmt.Fprintf(os.Stderr, "[DEBUG] Client initialized\n")
			}
		},
	}

	server := mcpsdk.NewServer(impl, serverOpts)

	// Register project-context tool
	tool := &mcpsdk.Tool{
		Name:        "project-context",
		Description: "Get project knowledge for AI context. Use {\"query\":\"search term\"} for semantic search, or omit for summary.",
	}

	mcpsdk.AddTool(server, tool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[ProjectContextParams]) (*mcpsdk.CallToolResultFor[any], error) {
		query := strings.TrimSpace(params.Arguments.Query)

		// Try new node-based system first
		nodes, err := store.ListNodes("")
		hasNodes := err == nil && len(nodes) > 0

		if hasNodes {
			return handleNodeContext(ctx, store, query, nodes)
		}

		// Fallback to legacy feature system
		result, err := handleLegacyContext(store, query)
		if err != nil && query == "" {
			// If it's a summary query and it failed/is empty, return a helpful message
			return &mcpsdk.CallToolResultFor[any]{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: "Project memory is empty. Run 'taskwing bootstrap' to analyze this repository and generate context."},
				},
			}, nil
		}
		return result, err
	})

	// Register list-specs tool
	specStore, specErr := spec.NewStore(cwd)
	if specErr == nil {
		listSpecsTool := &mcpsdk.Tool{
			Name:        "list-specs",
			Description: "List all feature specifications in this project.",
		}

		mcpsdk.AddTool(server, listSpecsTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[struct{}]) (*mcpsdk.CallToolResultFor[any], error) {
			specs, err := specStore.ListSpecs()
			if err != nil {
				return &mcpsdk.CallToolResultFor[any]{
					Content: []mcpsdk.Content{
						&mcpsdk.TextContent{Text: fmt.Sprintf("Error listing specs: %v", err)},
					},
				}, nil
			}

			if len(specs) == 0 {
				return &mcpsdk.CallToolResultFor[any]{
					Content: []mcpsdk.Content{
						&mcpsdk.TextContent{Text: "No specs found. Create one with: taskwing spec create \"feature description\""},
					},
				}, nil
			}

			jsonBytes, _ := json.MarshalIndent(specs, "", "  ")
			return &mcpsdk.CallToolResultFor[any]{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: string(jsonBytes)},
				},
			}, nil
		})

		// Register list-tasks tool
		type ListTasksParams struct {
			SpecSlug string `json:"spec_slug,omitempty"`
		}

		listTasksTool := &mcpsdk.Tool{
			Name:        "list-tasks",
			Description: "List development tasks. Use {\"spec_slug\":\"slug\"} to filter by spec.",
		}

		mcpsdk.AddTool(server, listTasksTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[ListTasksParams]) (*mcpsdk.CallToolResultFor[any], error) {
			tasks, err := specStore.ListTasks(params.Arguments.SpecSlug)
			if err != nil {
				return &mcpsdk.CallToolResultFor[any]{
					Content: []mcpsdk.Content{
						&mcpsdk.TextContent{Text: fmt.Sprintf("Error listing tasks: %v", err)},
					},
				}, nil
			}

			if len(tasks) == 0 {
				return &mcpsdk.CallToolResultFor[any]{
					Content: []mcpsdk.Content{
						&mcpsdk.TextContent{Text: "No tasks found."},
					},
				}, nil
			}

			jsonBytes, _ := json.MarshalIndent(tasks, "", "  ")
			return &mcpsdk.CallToolResultFor[any]{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: string(jsonBytes)},
				},
			}, nil
		})

		// Register get-task-context tool
		type GetTaskContextParams struct {
			TaskID string `json:"task_id"`
		}

		getTaskTool := &mcpsdk.Tool{
			Name:        "get-task-context",
			Description: "Get full context for a task including spec details and relevant code. Use {\"task_id\":\"task-abc123\"}",
		}

		mcpsdk.AddTool(server, getTaskTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[GetTaskContextParams]) (*mcpsdk.CallToolResultFor[any], error) {
			taskCtx, err := specStore.GetTaskContext(params.Arguments.TaskID)
			if err != nil {
				return &mcpsdk.CallToolResultFor[any]{
					Content: []mcpsdk.Content{
						&mcpsdk.TextContent{Text: fmt.Sprintf("Task not found: %v", err)},
					},
				}, nil
			}

			jsonBytes, _ := json.MarshalIndent(taskCtx, "", "  ")
			return &mcpsdk.CallToolResultFor[any]{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: string(jsonBytes)},
				},
			}, nil
		})
	}

	// Run the server
	if mcpPort > 0 {
		fmt.Fprintf(os.Stderr, "Starting SSE transport on port %d\n", mcpPort)
		return fmt.Errorf("SSE transport not yet implemented")
	}

	if err := server.Run(ctx, mcpsdk.NewStdioTransport()); err != nil {
		return fmt.Errorf("MCP server failed: %w", err)
	}

	return nil
}

// handleNodeContext returns context using the new node-based knowledge graph
func handleNodeContext(ctx context.Context, store *memory.SQLiteStore, query string, nodes []memory.Node) (*mcpsdk.CallToolResultFor[any], error) {
	if query == "" {
		// Summary response - group by type
		byType := make(map[string][]memory.Node)
		for _, n := range nodes {
			t := n.Type
			if t == "" {
				t = "unknown"
			}
			byType[t] = append(byType[t], n)
		}

		result := struct {
			Nodes map[string][]memory.Node `json:"nodes"`
			Total int                      `json:"total"`
		}{
			Nodes: byType,
			Total: len(nodes),
		}

		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		return &mcpsdk.CallToolResultFor[any]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(jsonBytes)},
			},
		}, nil
	}

	// Semantic search
	apiKey := viper.GetString("llm.apiKey")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	var results []memory.Node

	if apiKey != "" {
		// Use embeddings for semantic search
		llmCfg := llm.Config{APIKey: apiKey}
		queryEmb, err := knowledge.GenerateEmbedding(ctx, query, llmCfg)
		if err == nil {
			type scored struct {
				Node  memory.Node
				Score float32
			}
			var scoredNodes []scored

			for _, n := range nodes {
				fullNode, err := store.GetNode(n.ID)
				if err != nil || len(fullNode.Embedding) == 0 {
					continue
				}
				score := knowledge.CosineSimilarity(queryEmb, fullNode.Embedding)
				scoredNodes = append(scoredNodes, scored{Node: *fullNode, Score: score})
			}

			sort.Slice(scoredNodes, func(i, j int) bool {
				return scoredNodes[i].Score > scoredNodes[j].Score
			})

			limit := 5
			if len(scoredNodes) < limit {
				limit = len(scoredNodes)
			}
			for i := 0; i < limit; i++ {
				results = append(results, scoredNodes[i].Node)
			}
		}
	}

	// Fallback to keyword matching if semantic search didn't work
	if len(results) == 0 {
		queryLower := strings.ToLower(query)
		for _, n := range nodes {
			if strings.Contains(strings.ToLower(n.Content), queryLower) ||
				strings.Contains(strings.ToLower(n.Summary), queryLower) {
				results = append(results, n)
				if len(results) >= 5 {
					break
				}
			}
		}
	}

	result := struct {
		Query   string        `json:"query"`
		Results []memory.Node `json:"results"`
		Total   int           `json:"total"`
	}{
		Query:   query,
		Results: results,
		Total:   len(results),
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return &mcpsdk.CallToolResultFor[any]{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: string(jsonBytes)},
		},
	}, nil
}

// handleLegacyContext returns context using the old feature/decision system
func handleLegacyContext(store *memory.SQLiteStore, query string) (*mcpsdk.CallToolResultFor[any], error) {
	index, err := store.GetIndex()
	if err != nil {
		return nil, fmt.Errorf("get index: %w", err)
	}

	if query == "" {
		result := struct {
			Features []memory.FeatureSummary `json:"features"`
			Total    int                     `json:"total"`
		}{
			Features: index.Features,
			Total:    len(index.Features),
		}

		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		return &mcpsdk.CallToolResultFor[any]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(jsonBytes)},
			},
		}, nil
	}

	// Feature query (legacy)
	featureIDByName := make(map[string]string, len(index.Features))
	featureNameByID := make(map[string]string, len(index.Features))
	for _, f := range index.Features {
		featureNameByID[f.ID] = f.Name
		featureIDByName[strings.ToLower(strings.TrimSpace(f.Name))] = f.ID
	}

	queryKey := strings.ToLower(query)
	seedID := featureIDByName[queryKey]
	if seedID == "" {
		for _, f := range index.Features {
			if strings.Contains(strings.ToLower(f.Name), queryKey) {
				seedID = f.ID
				break
			}
		}
	}
	if seedID == "" {
		return nil, fmt.Errorf("no feature matches query: %q", query)
	}

	feature, _ := store.GetFeature(seedID)
	decisions, _ := store.GetDecisions(seedID)

	result := struct {
		Query     string            `json:"query"`
		Feature   *memory.Feature   `json:"feature"`
		Decisions []memory.Decision `json:"decisions"`
	}{
		Query:     query,
		Feature:   feature,
		Decisions: decisions,
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return &mcpsdk.CallToolResultFor[any]{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: string(jsonBytes)},
		},
	}, nil
}
