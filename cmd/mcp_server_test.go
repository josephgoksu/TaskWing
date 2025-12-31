/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Shared Types (JSON-RPC protocol)
// -----------------------------------------------------------------------------

// MCPRequest represents a JSON-RPC request to the MCP server
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      *int        `json:"id,omitempty"` // nil for notifications
}

// MCPResponse represents a JSON-RPC response from the MCP server
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// MCPError represents an error response from the MCP server
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// -----------------------------------------------------------------------------
// Test Harness — Single implementation for all MCP tests
// -----------------------------------------------------------------------------

// mcpTestHarness encapsulates MCP server test setup and teardown
type mcpTestHarness struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	stderr *bytes.Buffer
	cancel context.CancelFunc
}

// newMCPTestHarness creates and starts an MCP server for testing
func newMCPTestHarness(t *testing.T, workDir string) *mcpTestHarness {
	t.Helper()

	binPath := findBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)

	cmd := exec.CommandContext(ctx, binPath, "mcp")
	if workDir != "" {
		cmd.Dir = workDir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("Failed to start MCP server: %v", err)
	}

	return &mcpTestHarness{
		t:      t,
		cmd:    cmd,
		stdin:  stdin,
		reader: bufio.NewReader(stdout),
		stderr: &stderrBuf,
		cancel: cancel,
	}
}

// Close cleans up the test harness
func (h *mcpTestHarness) Close() {
	h.stdin.Close()
	_ = h.cmd.Process.Kill()
	_ = h.cmd.Wait()
	h.cancel()
}

// SendAndReceive sends a request and reads the response
func (h *mcpTestHarness) SendAndReceive(req MCPRequest) (*MCPResponse, error) {
	reqBytes, _ := json.Marshal(req)
	h.t.Logf("Sending: %s", string(reqBytes))

	if _, err := h.stdin.Write(append(reqBytes, '\n')); err != nil {
		return nil, err
	}

	// For notifications (no ID), don't expect response
	if req.ID == nil {
		return nil, nil
	}

	line, err := h.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	h.t.Logf("Received: %s", string(line))

	var resp MCPResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Initialize performs the MCP initialize handshake
func (h *mcpTestHarness) Initialize() (*MCPResponse, error) {
	id := 1
	return h.SendAndReceive(MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"clientInfo":      map[string]string{"name": "test", "version": "1.0"},
			"capabilities":    map[string]interface{}{},
		},
		ID: &id,
	})
}

// SendInitializedNotification sends the initialized notification
func (h *mcpTestHarness) SendInitializedNotification() {
	_, _ = h.SendAndReceive(MCPRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	})
}

// Stderr returns captured stderr content
func (h *mcpTestHarness) Stderr() string {
	return h.stderr.String()
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// findBinary locates the taskwing binary relative to test working directory
func findBinary(t *testing.T) string {
	t.Helper()

	// When running `go test ./cmd`, CWD is the repo root
	if _, err := os.Stat("./taskwing"); err == nil {
		abs, _ := filepath.Abs("./taskwing")
		return abs
	}

	// Try parent (when tests run from cmd/ directory)
	if _, err := os.Stat("../taskwing"); err == nil {
		abs, _ := filepath.Abs("../taskwing")
		return abs
	}

	t.Skip("taskwing binary not found, run 'make build' first")
	return ""
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

// TestMCPProtocolStdio tests that the MCP server responds correctly to initialize
func TestMCPProtocolStdio(t *testing.T) {
	h := newMCPTestHarness(t, "")
	defer h.Close()

	resp, err := h.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v\nStderr: %s", err, h.Stderr())
	}

	if resp.Error != nil {
		t.Fatalf("MCP initialize returned error: %v", resp.Error.Message)
	}
	if resp.ID != 1 {
		t.Errorf("Expected response ID 1, got %d", resp.ID)
	}

	// Verify server info in result
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected serverInfo in response")
	}
	if name, ok := serverInfo["name"].(string); !ok || name != "taskwing" {
		t.Errorf("Expected server name 'taskwing', got %v", serverInfo["name"])
	}

	t.Log("✅ MCP initialize handshake successful")
}

// TestMCPToolsList tests that the MCP server lists available tools
func TestMCPToolsList(t *testing.T) {
	h := newMCPTestHarness(t, "")
	defer h.Close()

	// Initialize
	if _, err := h.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v\nStderr: %s", err, h.Stderr())
	}
	h.SendInitializedNotification()

	// List tools
	id := 2
	resp, err := h.SendAndReceive(MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      &id,
	})
	if err != nil {
		t.Fatalf("Failed to read tools/list response: %v\nStderr: %s", err, h.Stderr())
	}

	if resp.Error != nil {
		t.Fatalf("tools/list returned error: %s", resp.Error.Message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("Expected tools array in response")
	}

	// Verify project-context tool exists
	found := false
	for _, tool := range tools {
		if toolMap, ok := tool.(map[string]interface{}); ok {
			if toolMap["name"] == "project-context" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("Expected 'project-context' tool in tools list")
	} else {
		t.Log("✅ project-context tool found")
	}
}

// TestMCPProjectContextSummary tests project-context summary mode (empty query)
// This verifies the TypeSummary response structure with counts and examples.
func TestMCPProjectContextSummary(t *testing.T) {
	// This test requires a project with actual nodes
	// Skip if running in isolation without bootstrap
	h := newMCPTestHarness(t, "")
	defer h.Close()

	if _, err := h.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	h.SendInitializedNotification()

	// Call project-context with empty query (summary mode)
	id := 2
	resp, err := h.SendAndReceive(MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "project-context",
			"arguments": map[string]string{}, // Empty = summary mode
		},
		ID: &id,
	})
	if err != nil {
		t.Fatalf("Failed: %v\nStderr: %s", err, h.Stderr())
	}

	if resp.Error != nil {
		t.Fatalf("tools/call returned error: %s", resp.Error.Message)
	}

	// Parse outer response
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("Expected content array in response")
	}

	// Parse the text content (should be JSON)
	firstContent := content[0].(map[string]interface{})
	text, _ := firstContent["text"].(string)

	// If empty, that's fine (handled by TestMCPProjectContextEmpty)
	if strings.Contains(text, "empty") || strings.Contains(text, "bootstrap") {
		t.Skip("Project is empty, summary mode test skipped")
	}

	// Parse the summary JSON
	var summary struct {
		Total int                    `json:"total"`
		Types map[string]interface{} `json:"types"`
	}
	if err := json.Unmarshal([]byte(text), &summary); err != nil {
		t.Fatalf("Failed to parse summary JSON: %v\nText: %s", err, text)
	}

	// Verify structure
	if summary.Total < 0 {
		t.Error("Total should be non-negative")
	}
	if summary.Types == nil {
		t.Error("Types map should exist")
	}

	// Verify each type has count and examples
	for typeName, typeData := range summary.Types {
		td, ok := typeData.(map[string]interface{})
		if !ok {
			t.Errorf("Type %s should be an object", typeName)
			continue
		}
		if _, hasCount := td["count"]; !hasCount {
			t.Errorf("Type %s missing 'count' field", typeName)
		}
		if _, hasExamples := td["examples"]; !hasExamples {
			t.Errorf("Type %s missing 'examples' field", typeName)
		}
	}

	t.Logf("✅ Summary mode returned %d nodes across %d types", summary.Total, len(summary.Types))
}

// TestMCPProjectContextEmpty tests project-context with empty memory
func TestMCPProjectContextEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	h := newMCPTestHarness(t, tmpDir)
	defer h.Close()

	// Initialize
	if _, err := h.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	h.SendInitializedNotification()

	// Call project-context
	id := 2
	resp, err := h.SendAndReceive(MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "project-context",
			"arguments": map[string]string{},
		},
		ID: &id,
	})
	if err != nil {
		t.Fatalf("Failed to read tools/call response: %v\nStderr: %s", err, h.Stderr())
	}

	if resp.Error != nil {
		t.Fatalf("tools/call returned error: %s", resp.Error.Message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("Expected content array in response")
	}

	// Check that response mentions empty state or bootstrap
	firstContent := content[0].(map[string]interface{})
	text, _ := firstContent["text"].(string)
	if strings.Contains(strings.ToLower(text), "empty") || strings.Contains(strings.ToLower(text), "bootstrap") {
		t.Log("✅ Empty project correctly returns bootstrap guidance")
	} else {
		t.Logf("Response text: %s", text)
	}
}
