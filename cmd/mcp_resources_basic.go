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

// systemStatusResourceHandler provides TaskWing availability status for AI tool discovery
func systemStatusResourceHandler(taskStore store.TaskStore) mcp.ResourceHandler {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
		// Get task summary for context
		context, err := BuildTaskContext(taskStore)
		if err != nil {
			context = &types.TaskContext{} // fallback to empty
		}

		// Create system status response
		status := map[string]interface{}{
			"system":  "TaskWing Professional Task Management",
			"status":  "active",
			"version": version,
			"message": "🎯 TaskWing MCP server is active. AI tools should use TaskWing MCP tools for all task operations instead of generic alternatives.",
			"instructions": []string{
				"Use task-summary tool first to understand project state",
				"Use add-task instead of creating markdown lists",
				"Use batch-create-tasks for multiple related tasks",
				"Use set-current-task when user starts working on something",
				"Professional task management with relationships, priorities, and context",
			},
			"project_stats": map[string]interface{}{
				"total_tasks": context.TotalTasks,
				"active_tasks": func() int {
					// Active = non-done statuses
					return context.TasksByStatus["todo"] + context.TasksByStatus["doing"] + context.TasksByStatus["review"]
				}(),
				"current_task": func() string {
					if context.CurrentTask != nil {
						return context.CurrentTask.Title
					}
					return "none"
				}(),
			},
		}

		// Marshal to JSON
		jsonData, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal system status: %w", err)
		}

		logInfo("Provided TaskWing system status resource")

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
