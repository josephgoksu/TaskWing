/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/taskwing.app/store"
	"github.com/josephgoksu/taskwing.app/types"
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

// archiveResourceHandler provides access to archived tasks and project history
func archiveResourceHandler() mcp.ResourceHandler {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
		cfg := GetConfig()
		
		// Parse URI to determine what archive data to return
		// Format: taskwing://archive or taskwing://archive?project=name&date=2025-01-19
		uri := params.URI
		
		var responseData interface{}
		var description string
		
		if strings.Contains(uri, "?") {
			// Query specific archive data
			responseData, description = querySpecificArchive(uri, cfg)
		} else {
			// Return archive index and summary
			responseData, description = getArchiveIndex(cfg)
		}
		
		// Marshal to JSON
		jsonData, err := json.MarshalIndent(responseData, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal archive data to JSON: %w", err)
		}
		
		logInfo(fmt.Sprintf("Provided archive resource: %s", description))
		
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

// knowledgeResourceHandler provides access to knowledge base and patterns
func knowledgeResourceHandler() mcp.ResourceHandler {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
		cfg := GetConfig()
		
		// Parse URI to determine what knowledge data to return
		// Format: taskwing://knowledge or taskwing://knowledge?type=patterns&pattern=doc-consolidation
		uri := params.URI
		
		var responseData interface{}
		var description string
		
		if strings.Contains(uri, "?") {
			// Query specific knowledge data
			responseData, description = querySpecificKnowledge(uri, cfg)
		} else {
			// Return complete knowledge base
			responseData, description = getCompleteKnowledgeBase(cfg)
		}
		
		// Marshal to JSON
		jsonData, err := json.MarshalIndent(responseData, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal knowledge data to JSON: %w", err)
		}
		
		logInfo(fmt.Sprintf("Provided knowledge resource: %s", description))
		
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

// Helper functions for archive resource

func getArchiveIndex(cfg *types.AppConfig) (interface{}, string) {
	indexPath := filepath.Join(cfg.Project.RootDir, "archive", "index.json")
	
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return map[string]interface{}{
			"error": "Archive index not found",
			"archives": []interface{}{},
			"statistics": map[string]interface{}{
				"total_archives": 0,
				"total_tasks_archived": 0,
			},
		}, "empty archive index"
	}
	
	var index map[string]interface{}
	if err := json.Unmarshal(data, &index); err != nil {
		return map[string]interface{}{
			"error": "Failed to parse archive index",
		}, "corrupted archive index"
	}
	
	// Enhance with summary information for AI
	if archives, ok := index["archives"].([]interface{}); ok {
		index["summary"] = map[string]interface{}{
			"total_projects": len(archives),
			"ai_guidance": "Use this data to understand past project patterns, success rates, and common approaches. Reference specific archive files for detailed task breakdowns.",
		}
	}
	
	return index, fmt.Sprintf("archive index with %v projects", index["statistics"])
}

func querySpecificArchive(uri string, cfg *types.AppConfig) (interface{}, string) {
	// Parse query parameters from URI
	// For now, return a specific archive file
	// In full implementation, would parse ?project=name&date=date parameters
	
	// Example: Return the most recent archive
	indexPath := filepath.Join(cfg.Project.RootDir, "archive", "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return map[string]interface{}{"error": "Archive index not found"}, "archive index error"
	}
	
	var index ArchiveIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return map[string]interface{}{"error": "Failed to parse archive index"}, "archive parse error"
	}
	
	if len(index.Archives) == 0 {
		return map[string]interface{}{"error": "No archives found"}, "no archives"
	}
	
	// Return the most recent archive
	recentArchive := index.Archives[len(index.Archives)-1]
	archivePath := filepath.Join(cfg.Project.RootDir, "archive", recentArchive.FilePath)
	
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		return map[string]interface{}{"error": "Archive file not found"}, "archive file error"
	}
	
	var archive map[string]interface{}
	json.Unmarshal(archiveData, &archive)
	
	return archive, fmt.Sprintf("archive for %s", recentArchive.ProjectName)
}

// Helper functions for knowledge resource

func getCompleteKnowledgeBase(cfg *types.AppConfig) (interface{}, string) {
	knowledgeBase := map[string]interface{}{
		"version": "1.0",
		"last_updated": "2025-08-19",
		"description": "TaskWing Knowledge Base - Accumulated wisdom from completed projects",
	}
	
	// Load patterns
	patterns, err := loadPatterns(cfg)
	if err == nil {
		knowledgeBase["patterns"] = patterns
	}
	
	// Load retrospectives summary
	retrospectives, err := loadRetrospectives(cfg)
	if err == nil {
		knowledgeBase["retrospectives"] = retrospectives
	}
	
	// Load decisions
	decisions, err := loadDecisions(cfg)
	if err == nil {
		knowledgeBase["decisions"] = decisions
	}
	
	// Add AI guidance
	knowledgeBase["ai_guidance"] = map[string]interface{}{
		"patterns": "Use patterns for task generation and time estimation",
		"retrospectives": "Reference lessons learned and project outcomes",
		"decisions": "Understand context and rationale for similar situations",
		"best_practices": "Apply proven approaches for similar work types",
	}
	
	return knowledgeBase, "complete knowledge base"
}

func querySpecificKnowledge(uri string, cfg *types.AppConfig) (interface{}, string) {
	// Parse query parameters - for now return patterns specifically
	patterns, err := loadPatterns(cfg)
	if err != nil {
		return map[string]interface{}{"error": "No patterns found"}, "patterns error"
	}
	
	return map[string]interface{}{
		"patterns": patterns,
		"usage_guidance": "Use these patterns for task breakdown and estimation",
	}, "task patterns"
}

func loadPatterns(cfg *types.AppConfig) ([]interface{}, error) {
	patternsDir := filepath.Join(cfg.Project.RootDir, "knowledge", "patterns")
	
	files, err := os.ReadDir(patternsDir)
	if err != nil {
		return nil, err
	}
	
	patterns := []interface{}{}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			filePath := filepath.Join(patternsDir, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			
			var pattern map[string]interface{}
			if err := json.Unmarshal(data, &pattern); err != nil {
				continue
			}
			
			patterns = append(patterns, pattern)
		}
	}
	
	return patterns, nil
}

func loadRetrospectives(cfg *types.AppConfig) ([]interface{}, error) {
	retrospectivesDir := filepath.Join(cfg.Project.RootDir, "knowledge", "retrospectives")
	
	files, err := os.ReadDir(retrospectivesDir)
	if err != nil {
		return nil, err
	}
	
	retrospectives := []interface{}{}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".md") {
			retrospectives = append(retrospectives, map[string]interface{}{
				"file": file.Name(),
				"path": filepath.Join("knowledge", "retrospectives", file.Name()),
				"project": strings.TrimSuffix(file.Name(), ".md"),
			})
		}
	}
	
	return retrospectives, nil
}

func loadDecisions(cfg *types.AppConfig) ([]interface{}, error) {
	decisionsDir := filepath.Join(cfg.Project.RootDir, "knowledge", "decisions")
	
	files, err := os.ReadDir(decisionsDir)
	if err != nil {
		return nil, err
	}
	
	decisions := []interface{}{}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".md") {
			decisions = append(decisions, map[string]interface{}{
				"file": file.Name(),
				"path": filepath.Join("knowledge", "decisions", file.Name()),
				"decision": strings.TrimSuffix(file.Name(), ".md"),
			})
		}
	}
	
	return decisions, nil
}
