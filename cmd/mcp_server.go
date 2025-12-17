/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/memory"
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
- Features and their relationships
- Architectural decisions and rationale
- Project structure and dependencies

Example usage with Claude Code:
  taskwing mcp

The server will run until the client disconnects.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
	fmt.Fprintf(os.Stderr, "Institutional Knowledge Layer for Engineering Teams\n")
	fmt.Fprintf(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Initialize memory store
	store, err := memory.NewSQLiteStore(GetMemoryBasePath())
	if err != nil {
		return fmt.Errorf("failed to initialize memory store: %w", err)
	}
	defer store.Close()

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
		Description: "Get project memory for AI context. Use {\"query\":\"FeatureName\"} to fetch detailed context for a feature and its related features.",
	}

	mcpsdk.AddTool(server, tool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[ProjectContextParams]) (*mcpsdk.CallToolResultFor[any], error) {
		index, err := store.GetIndex()
		if err != nil {
			return nil, fmt.Errorf("get index: %w", err)
		}

		featureNameByID := make(map[string]string, len(index.Features))
		featureIDByName := make(map[string]string, len(index.Features))
		for _, f := range index.Features {
			featureNameByID[f.ID] = f.Name
			featureIDByName[strings.ToLower(strings.TrimSpace(f.Name))] = f.ID
		}

		query := strings.TrimSpace(params.Arguments.Query)
		if query == "" {
			// Summary-only response (fast and bounded).
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

		queryKey := strings.ToLower(query)
		seedID := featureIDByName[queryKey]
		if seedID == "" {
			// Fuzzy match: first feature name containing the query substring.
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

		relatedIDs, err := store.GetRelated(seedID, 2)
		if err != nil {
			return nil, fmt.Errorf("get related: %w", err)
		}

		selectedIDs := make([]string, 0, 1+len(relatedIDs))
		selectedIDs = append(selectedIDs, seedID)
		selectedIDs = append(selectedIDs, relatedIDs...)

		type featureDetail struct {
			Feature    *memory.Feature   `json:"feature"`
			Decisions  []memory.Decision `json:"decisions"`
			DependsOn  []string          `json:"dependsOn"`
			DependedBy []string          `json:"dependedBy"`
		}

		details := make([]featureDetail, 0, len(selectedIDs))
		for _, id := range selectedIDs {
			feature, err := store.GetFeature(id)
			if err != nil {
				continue
			}

			decisions, _ := store.GetDecisions(id)

			depIDs, _ := store.GetDependencies(id)
			deps := make([]string, 0, len(depIDs))
			for _, depID := range depIDs {
				name := featureNameByID[depID]
				if name == "" {
					name = depID
				}
				deps = append(deps, name)
			}

			dependentIDs, _ := store.GetDependents(id)
			dependents := make([]string, 0, len(dependentIDs))
			for _, depID := range dependentIDs {
				name := featureNameByID[depID]
				if name == "" {
					name = depID
				}
				dependents = append(dependents, name)
			}

			details = append(details, featureDetail{
				Feature:    feature,
				Decisions:  decisions,
				DependsOn:  deps,
				DependedBy: dependents,
			})
		}

		result := struct {
			Query    string          `json:"query"`
			Seed     string          `json:"seed"`
			Features []featureDetail `json:"features"`
			Total    int             `json:"total"`
		}{
			Query:    query,
			Seed:     featureNameByID[seedID],
			Features: details,
			Total:    len(details),
		}

		// Format response as JSON text
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")

		return &mcpsdk.CallToolResultFor[any]{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(jsonBytes)},
			},
		}, nil
	})

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
