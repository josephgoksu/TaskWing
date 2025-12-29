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
	"github.com/josephgoksu/TaskWing/internal/ui"
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
	ui.RenderPageHeader("TaskWing MCP Server", "Knowledge Graph for Engineering Teams")
	fmt.Fprintf(os.Stderr, "\n")

	// Get current working directory
	// Initialize memory repository
	repo, err := memory.NewDefaultRepository(config.GetMemoryBasePath())
	if err != nil {
		return fmt.Errorf("failed to initialize memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

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

		// Node-based system only
		nodes, err := repo.ListNodes("")
		if err != nil {
			return nil, fmt.Errorf("list nodes: %w", err)
		}
		if len(nodes) == 0 {
			return &mcpsdk.CallToolResultFor[any]{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: "Project memory is empty. Run 'taskwing bootstrap' to analyze this repository and generate context."},
				},
			}, nil
		}
		return handleNodeContext(ctx, repo, query, nodes)
	})

	// Spec tools removed as part of cleanup

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
func handleNodeContext(ctx context.Context, repo *memory.Repository, query string, nodes []memory.Node) (*mcpsdk.CallToolResultFor[any], error) {
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

	var results []memory.Node
	// Semantic search
	if llmCfg, cfgErr := config.LoadLLMConfig(); cfgErr == nil {
		if llmCfg.Provider != llm.ProviderAnthropic && (llmCfg.APIKey != "" || llmCfg.Provider == llm.ProviderOllama) {
			queryEmb, err := knowledge.GenerateEmbedding(ctx, query, llmCfg)
			if err == nil {
				type scored struct {
					Node  memory.Node
					Score float32
				}
				var scoredNodes []scored

				// Fix N+1 query: get all nodes with embeddings in a single query
				nodesWithEmbeddings, err := repo.ListNodesWithEmbeddings()
				if err == nil {
					// Build map for O(1) lookup by ID
					nodeMap := make(map[string]*memory.Node)
					for i := range nodesWithEmbeddings {
						nodeMap[nodesWithEmbeddings[i].ID] = &nodesWithEmbeddings[i]
					}

					// Score each node from the original list
					for _, n := range nodes {
						fullNode, ok := nodeMap[n.ID]
						if !ok || len(fullNode.Embedding) == 0 {
							continue
						}
						score := knowledge.CosineSimilarity(queryEmb, fullNode.Embedding)
						scoredNodes = append(scoredNodes, scored{Node: *fullNode, Score: score})
					}
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
