/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

// Basic MCP resources: tasks, config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tasksResourceHandler provides access to all tasks in JSON format
func tasksResourceHandler(taskStore store.TaskStore) mcp.ResourceHandler {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
		// List all tasks
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list tasks: %w", err)
		}

		// Convert to response format
		taskResponses := make([]types.TaskResponse, len(tasks))
		for i, task := range tasks {
			taskResponses[i] = taskToResponse(task)
		}

		// Marshal to JSON
		jsonData, err := json.MarshalIndent(taskResponses, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tasks to JSON: %w", err)
		}

		logInfo(fmt.Sprintf("Provided tasks resource with %d tasks", len(tasks)))

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      params.URI,
					MIMEType: "application/json",
					Text:     string(jsonData),
				},
			},
		}, nil
	}
}

// configResourceHandler provides access to TaskWing configuration
func configResourceHandler() mcp.ResourceHandler {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
		// Get current simple configuration
		config := GetConfig()

		// Use the simple config structure directly
		mcpConfig := config

		// Marshal to JSON
		jsonData, err := json.MarshalIndent(mcpConfig, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config to JSON: %w", err)
		}

		logInfo("Provided config resource")

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      params.URI,
					MIMEType: "application/json",
					Text:     string(jsonData),
				},
			},
		}, nil
	}
}

// Archive and knowledge resources removed in cleanup.
