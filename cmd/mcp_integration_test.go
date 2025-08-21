package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// MCPTestResult represents the result of an MCP tool test
type MCPTestResult struct {
	ToolName    string
	Success     bool
	Error       string
	Duration    time.Duration
	Description string
}

// MCPIntegrationTestSuite manages MCP server integration tests
type MCPIntegrationTestSuite struct {
	tempDir    string
	binaryPath string
	results    []MCPTestResult
}

// SetupMCPIntegrationTest initializes the integration test environment
func SetupMCPIntegrationTest(t *testing.T) *MCPIntegrationTestSuite {
	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "taskwing-mcp-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Build TaskWing binary for testing
	binaryPath := filepath.Join(tempDir, "taskwing")
	// Get the project root directory (go up from cmd to root)
	projectRoot, _ := filepath.Abs("..")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "main.go")
	buildCmd.Dir = projectRoot
	if err := buildCmd.Run(); err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("Failed to build TaskWing binary: %v", err)
	}

	return &MCPIntegrationTestSuite{
		tempDir:    tempDir,
		binaryPath: binaryPath,
		results:    make([]MCPTestResult, 0),
	}
}

// Cleanup removes temporary test files
func (suite *MCPIntegrationTestSuite) Cleanup() {
	if suite.tempDir != "" {
		_ = os.RemoveAll(suite.tempDir)
	}
}

// RunTest executes a single integration test
func (suite *MCPIntegrationTestSuite) RunTest(toolName, description string, testFunc func() error) {
	start := time.Now()

	result := MCPTestResult{
		ToolName:    toolName,
		Description: description,
	}

	err := testFunc()
	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	result.Duration = time.Since(start)
	suite.results = append(suite.results, result)
}

// TestMCPServerHelp tests if MCP server help command works
func TestMCPServerHelp(t *testing.T) {
	suite := SetupMCPIntegrationTest(t)
	defer suite.Cleanup()

	suite.RunTest("mcp-server-help", "MCP server help command should work", func() error {
		cmd := exec.Command(suite.binaryPath, "mcp", "--help")
		cmd.Dir = suite.tempDir
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("MCP help command failed: %v", err)
		}

		if !strings.Contains(string(output), "Start a Model Context Protocol") {
			return fmt.Errorf("MCP help output doesn't contain expected text")
		}

		return nil
	})

	suite.PrintResults(t)
}

// TestTaskWingBinaryBasics tests basic TaskWing functionality
func TestTaskWingBinaryBasics(t *testing.T) {
	suite := SetupMCPIntegrationTest(t)
	defer suite.Cleanup()

	// Test binary execution
	suite.RunTest("binary-help", "TaskWing binary should show help", func() error {
		cmd := exec.Command(suite.binaryPath, "--help")
		cmd.Dir = suite.tempDir
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("Help command failed: %v", err)
		}

		if !strings.Contains(string(output), "TaskWing") {
			return fmt.Errorf("Help output doesn't contain 'TaskWing'")
		}

		return nil
	})

	// Test version command
	suite.RunTest("binary-version", "TaskWing binary should show version", func() error {
		cmd := exec.Command(suite.binaryPath, "version")
		cmd.Dir = suite.tempDir
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("Version command failed: %v", err)
		}

		if len(strings.TrimSpace(string(output))) == 0 {
			return fmt.Errorf("Version output is empty")
		}

		return nil
	})

	suite.PrintResults(t)
}

// TestBasicTaskOperations tests basic task management without MCP
func TestBasicTaskOperations(t *testing.T) {
	suite := SetupMCPIntegrationTest(t)
	defer suite.Cleanup()

	// Initialize TaskWing in the temp directory
	initCmd := exec.Command(suite.binaryPath, "init")
	initCmd.Dir = suite.tempDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("Failed to initialize TaskWing: %v", err)
	}

	// Test add command
	suite.RunTest("add-task", "Should be able to add a task", func() error {
		cmd := exec.Command(suite.binaryPath, "add", "Test Task", "--description", "Test description", "--non-interactive")
		cmd.Dir = suite.tempDir
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("Add task command failed: %v", err)
		}

		if !strings.Contains(string(output), "Task added successfully") && !strings.Contains(string(output), "Test Task") {
			return fmt.Errorf("Add task output doesn't indicate success")
		}

		return nil
	})

	// Test list command
	suite.RunTest("list-tasks", "Should be able to list tasks", func() error {
		cmd := exec.Command(suite.binaryPath, "list")
		cmd.Dir = suite.tempDir
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("List tasks command failed: %v", err)
		}

		if !strings.Contains(string(output), "Test Task") {
			return fmt.Errorf("List doesn't contain the added task")
		}

		return nil
	})

	suite.PrintResults(t)
}

// PrintResults outputs test results in a formatted way
func (suite *MCPIntegrationTestSuite) PrintResults(t *testing.T) {
	totalTests := len(suite.results)
	passed := 0
	failed := 0

	for _, result := range suite.results {
		if result.Success {
			passed++
			t.Logf("✅ %-20s - %s (%.2fms)", result.ToolName, result.Description, float64(result.Duration.Nanoseconds())/1e6)
		} else {
			failed++
			t.Errorf("❌ %-20s - %s (%.2fms) - Error: %s", result.ToolName, result.Description, float64(result.Duration.Nanoseconds())/1e6, result.Error)
		}
	}

	if totalTests > 0 {
		t.Logf("\n=== INTEGRATION TEST SUMMARY ===")
		t.Logf("Total Tests: %d", totalTests)
		t.Logf("Passed: %d", passed)
		t.Logf("Failed: %d", failed)
		t.Logf("Success Rate: %.1f%%", float64(passed)/float64(totalTests)*100)
	}
}

// TestAllMCPToolsComprehensive runs a comprehensive test of all MCP tools
func TestAllMCPToolsComprehensive(t *testing.T) {
	// This is a placeholder for the comprehensive MCP tools test
	// It would use the MCP server binary and test tool functionality

	suite := SetupMCPIntegrationTest(t)
	defer suite.Cleanup()

	// Initialize TaskWing in the temp directory
	initCmd := exec.Command(suite.binaryPath, "init")
	initCmd.Dir = suite.tempDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("Failed to initialize TaskWing: %v", err)
	}

	// Define all MCP tools to test
	mcpTools := map[string]string{
		"task-summary":          "Get project overview and health metrics",
		"add-task":              "Create individual tasks with validation",
		"list-tasks":            "Filter tasks by various criteria",
		"get-task":              "Retrieve task details by ID",
		"update-task":           "Modify task fields",
		"delete-task":           "Remove tasks with dependency checks",
		"mark-done":             "Complete tasks and set timestamps",
		"batch-create-tasks":    "Create multiple tasks with relationships",
		"bulk-tasks":            "Batch complete/delete/prioritize by task IDs",
		"bulk-by-filter":        "Bulk operations with filter expressions",
		"search-tasks":          "Full-text search across task content",
		"find-task":             "Smart task resolution by partial reference",
		"find-task-by-title":    "Fuzzy title matching with scores",
		"set-current-task":      "Set active task for context",
		"get-current-task":      "Retrieve active task context",
		"clear-current-task":    "Remove current task reference",
		"board-snapshot":        "Kanban-style status overview",
		"workflow-status":       "Project phase and completion analysis",
		"query-tasks":           "Natural language and structured queries",
		"extract-task-ids":      "Get task IDs with simple criteria",
		"task-analytics":        "Compute project metrics with grouping",
		"suggest-tasks":         "Context-aware task suggestions",
		"task-autocomplete":     "Title completion suggestions",
		"smart-task-transition": "AI-powered next step recommendations",
		"dependency-health":     "Analyze and validate task relationships",
	}

	// For now, just test that MCP server can start and show help
	suite.RunTest("mcp-comprehensive", "MCP server comprehensive test (basic)", func() error {
		cmd := exec.Command(suite.binaryPath, "mcp", "--help")
		cmd.Dir = suite.tempDir
		_, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("MCP server help failed: %v", err)
		}
		return nil
	})

	// Mark that we've identified all tools for future implementation
	t.Logf("Found %d MCP tools to test: %v", len(mcpTools), getKeys(mcpTools))

	suite.PrintResults(t)
}

// getKeys returns the keys of a map as a slice
func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
