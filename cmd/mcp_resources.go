/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/josephgoksu/taskwing.app/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/viper"
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
		taskResponses := make([]TaskResponse, len(tasks))
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
		// Get current configuration
		config := GetConfig()

		// Create a simplified config structure for MCP
		mcpConfig := struct {
			Project struct {
				RootDir       string `json:"rootDir"`
				TasksDir      string `json:"tasksDir"`
				OutputLogPath string `json:"outputLogPath"`
			} `json:"project"`
			Data struct {
				File   string `json:"file"`
				Format string `json:"format"`
			} `json:"data"`
			Debug   bool `json:"debug"`
			Verbose bool `json:"verbose"`
		}{
			Project: struct {
				RootDir       string `json:"rootDir"`
				TasksDir      string `json:"tasksDir"`
				OutputLogPath string `json:"outputLogPath"`
			}{
				RootDir:       config.Project.RootDir,
				TasksDir:      config.Project.TasksDir,
				OutputLogPath: config.Project.OutputLogPath,
			},
			Data: struct {
				File   string `json:"file"`
				Format string `json:"format"`
			}{
				File:   viper.GetString("data.file"),
				Format: viper.GetString("data.format"),
			},
			Debug:   viper.GetBool("debug"),
			Verbose: viper.GetBool("verbose"),
		}

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
