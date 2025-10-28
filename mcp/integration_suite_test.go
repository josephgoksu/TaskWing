package mcp

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

		if !strings.Contains(string(output), "Start MCP server") {
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
		cmd := exec.Command(suite.binaryPath, "add", "Test Task", "--non-interactive", "--no-ai")
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

// TestMCPToolsIntegration runs basic integration tests for MCP tools
func TestMCPToolsIntegration(t *testing.T) {
	suite := SetupMCPIntegrationTest(t)
	defer suite.Cleanup()

	// Test basic MCP functionality through binary
	suite.RunTest("mcp-init", "Initialize TaskWing project", func() error {
		cmd := exec.Command(suite.binaryPath, "init")
		cmd.Dir = suite.tempDir
		return cmd.Run()
	})

	suite.RunTest("mcp-basic-task", "Create and manage task via binary", func() error {
		// Create a task
		cmd := exec.Command(suite.binaryPath, "add", "Test Task", "--no-ai")
		cmd.Dir = suite.tempDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create task: %w", err)
		}

		// List tasks to verify
		cmd = exec.Command(suite.binaryPath, "list")
		cmd.Dir = suite.tempDir
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to list tasks: %w", err)
		}

		if !strings.Contains(string(output), "Test Task") {
			return fmt.Errorf("task not found in output: %s", output)
		}

		return nil
	})

	suite.PrintResults(t)
}
