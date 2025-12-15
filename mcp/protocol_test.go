package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"testing"
	"time"
)

// jsonrpcRequest is a minimal JSON-RPC 2.0 request shape
type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonrpcResponse is a minimal JSON-RPC 2.0 response shape
type jsonrpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int              `json:"id,omitempty"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data,omitempty"`
	} `json:"error,omitempty"`
}

// writeFrame writes an MCP/JSON-RPC message with Content-Length headers
func writeFrame(w io.Writer, payload []byte) error {
	// MCP go-sdk here expects newline-delimited JSON on stdio
	if _, err := w.Write(payload); err != nil {
		return err
	}
	_, err := w.Write([]byte("\n"))
	return err
}

// readFrame reads a single framed MCP/JSON-RPC message (Content-Length based)
func readFrame(r *bufio.Reader, max int) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) > max {
		return nil, fmt.Errorf("frame too large: %d", len(line))
	}
	return bytes.TrimRight(line, "\r\n"), nil
}

// sendRPC sends a JSON-RPC request and returns the parsed response
func sendRPC(t *testing.T, w io.Writer, r *bufio.Reader, id int, method string, params interface{}) (*jsonrpcResponse, []byte, error) {
	t.Helper()
	req := jsonrpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request failed: %w", err)
	}
	if err := writeFrame(w, payload); err != nil {
		return nil, nil, fmt.Errorf("write frame failed: %w", err)
	}
	// Read frames until we get a response with matching ID
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readFrame(r, 1<<20)
		if err != nil {
			return nil, nil, fmt.Errorf("read frame failed: %w", err)
		}
		// Try parse as response; skip notifications
		var resp jsonrpcResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			return nil, msg, fmt.Errorf("unmarshal response failed: %w (msg=%s)", err, string(msg))
		}
		if resp.ID == id {
			return &resp, msg, nil
		}
		// Not our response; continue loop (notification or other response)
	}
	return nil, nil, fmt.Errorf("timeout waiting for response to id=%d", id)
}

// sendNotification sends a JSON-RPC notification (no id)
func sendNotification(t *testing.T, w io.Writer, method string, params interface{}) {
	t.Helper()
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal notification failed: %v", err)
	}
	if err := writeFrame(w, payload); err != nil {
		t.Fatalf("write notification failed: %v", err)
	}
}

// TestMCPProtocolStdio starts the MCP server and exercises a minimal JSON-RPC flow
func TestMCPProtocolStdio(t *testing.T) {
	suite := SetupMCPIntegrationTest(t)
	defer suite.Cleanup()

	// Ensure project-scoped config exists so Viper prefers it over $HOME
	cfgDir := suite.tempDir + "/.taskwing"
	_ = exec.Command("mkdir", "-p", cfgDir).Run()
	_ = exec.Command("bash", "-lc", ": > '"+cfgDir+"/.taskwing.yaml' ").Run()

	// Start MCP server process
	cmd := exec.Command(suite.binaryPath, "mcp")
	cmd.Dir = suite.tempDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start mcp server: %v", err)
	}

	reader := bufio.NewReader(stdout)

	// Give the server a brief moment to start
	time.Sleep(150 * time.Millisecond)

	// 1) initialize
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "taskwing-mcp-test",
			"version": "0.0.1",
		},
	}
	resp, raw, err := sendRPC(t, stdin, reader, 1, "initialize", initParams)
	if err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatalf("initialize failed: %v; stderr=%s", err, stderr.String())
	}
	if resp.Error != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatalf("initialize error: %v (%s); stderr=%s", resp.Error.Message, string(raw), stderr.String())
	}

	// 2) initialized notification
	sendNotification(t, stdin, "initialized", map[string]interface{}{})

	// 3) tools/list
	listResp, listRaw, err := sendRPC(t, stdin, reader, 2, "tools/list", map[string]interface{}{})
	if err != nil || listResp.Error != nil || listResp.Result == nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatalf("tools/list error: %v %+v (%s); stderr=%s", err, listResp.Error, string(listRaw), stderr.String())
	}
	// Verify at least one known tool exists (e.g., add-task)
	var list struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(*listResp.Result, &list); err != nil {
		t.Fatalf("parse tools/list result: %v (%s)", err, string(listRaw))
	}
	foundAdd := false
	for _, tl := range list.Tools {
		if tl.Name == "add-task" {
			foundAdd = true
			break
		}
	}
	if !foundAdd {
		t.Fatalf("expected 'add-task' in tools list, got %v", list.Tools)
	}

	// 4) tools/call add-task
	callParams := map[string]interface{}{
		"name": "add-task",
		"arguments": map[string]interface{}{
			"title": "MCP stdio test task",
		},
	}
	callResp, callRaw, err := sendRPC(t, stdin, reader, 3, "tools/call", callParams)
	if err != nil || callResp.Error != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatalf("tools/call(add-task) error: %v %+v (%s); stderr=%s", err, callResp.Error, string(callRaw), stderr.String())
	}

	// 5) tools/call list-tasks (smoke)
	listCallParams := map[string]interface{}{
		"name":      "list-tasks",
		"arguments": map[string]interface{}{},
	}
	listCallResp, listCallRaw, err := sendRPC(t, stdin, reader, 4, "tools/call", listCallParams)
	if err != nil || listCallResp.Error != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatalf("tools/call(list-tasks) error: %v %+v (%s); stderr=%s", err, listCallResp.Error, string(listCallRaw), stderr.String())
	}

	// Shutdown server by closing stdin and waiting
	_ = stdin.Close()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("mcp server did not exit in time; stderr=%s", stderr.String())
	case err := <-done:
		if err != nil {
			t.Fatalf("mcp server exit with error: %v; stderr=%s", err, stderr.String())
		}
	}
}

// TestMCPAllToolsSmoke enumerates tools and calls each with minimal valid params
func TestMCPAllToolsSmoke(t *testing.T) {
	suite := SetupMCPIntegrationTest(t)
	defer suite.Cleanup()

	// Ensure project-scoped config exists
	cfgDir := suite.tempDir + "/.taskwing"
	_ = exec.Command("mkdir", "-p", cfgDir).Run()
	_ = exec.Command("bash", "-lc", ": > '"+cfgDir+"/.taskwing.yaml' ").Run()

	// Start MCP server
	cmd := exec.Command(suite.binaryPath, "mcp")
	cmd.Dir = suite.tempDir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mcp: %v", err)
	}
	reader := bufio.NewReader(stdout)

	// Init
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]interface{}{"name": "smoke", "version": "0"},
	}
	if resp, _, err := sendRPC(t, stdin, reader, 1, "initialize", initParams); err != nil || resp.Error != nil {
		t.Fatalf("initialize failed: %v %+v; stderr=%s", err, resp.Error, stderr.String())
	}
	sendNotification(t, stdin, "initialized", map[string]interface{}{})

	// Seed tasks so tools have data to operate on
	add := func(title string) {
		params := map[string]interface{}{"name": "add-task", "arguments": map[string]interface{}{"title": title, "description": "seed"}}
		if resp, raw, err := sendRPC(t, stdin, reader, 2, "tools/call", params); err != nil || (resp.Error != nil) {
			t.Fatalf("seed add-task failed: %v %+v (%s)", err, resp.Error, string(raw))
		}
	}
	add("Smoke Task One")
	add("Smoke Task Two")

	// Local struct to parse structured list
	type typesTask struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	type typesTaskListResponse struct {
		Tasks []typesTask `json:"tasks"`
		Count int         `json:"count"`
	}
	// Helper to list tasks and return first ID
	getFirstTaskID := func() string {
		params := map[string]interface{}{"name": "list-tasks", "arguments": map[string]interface{}{}}
		resp, raw, err := sendRPC(t, stdin, reader, 3, "tools/call", params)
		if err != nil || resp.Error != nil || resp.Result == nil {
			t.Fatalf("list-tasks failed: %v %+v (%s)", err, resp.Error, string(raw))
		}
		var decoded struct {
			Content    []interface{}   `json:"content"`
			Structured json.RawMessage `json:"structuredContent"`
		}
		if err := json.Unmarshal(*resp.Result, &decoded); err != nil {
			t.Fatalf("unmarshal list result: %v", err)
		}
		var list typesTaskListResponse
		if len(decoded.Structured) == 0 {
			return ""
		}
		if err := json.Unmarshal(decoded.Structured, &list); err != nil {
			t.Fatalf("unmarshal structured list: %v (%s)", err, string(decoded.Structured))
		}
		if list.Count > 0 && len(list.Tasks) > 0 {
			return list.Tasks[0].ID
		}
		return ""
	}

	firstID := getFirstTaskID()
	if firstID == "" {
		t.Fatalf("no tasks found after seeding")
	}

	// Fetch tools list
	listResp, listRaw, err := sendRPC(t, stdin, reader, 4, "tools/list", map[string]interface{}{})
	if err != nil || listResp.Error != nil || listResp.Result == nil {
		t.Fatalf("tools/list failed: %v %+v (%s)", err, listResp.Error, string(listRaw))
	}
	var tools struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(*listResp.Result, &tools); err != nil {
		t.Fatalf("parse tools: %v", err)
	}

	// Minimal argument mapping per tool
	args := map[string]map[string]interface{}{
		"add-task":          {"title": "Smoke Extra Task"},
		"list-tasks":        {},
		"get-task":          {"reference": "Smoke Task One"},
		"update-task":       {"reference": "Smoke Task One", "priority": "high"},
		"delete-task":       {"reference": "Smoke Task Two"},
		"mark-done":         {"reference": "Smoke Task One"},
		"set-current-task":  {"id": firstID},
		"task-summary":      {},
	}

	// Call each tool found with mapped arguments when available
	failed := 0
	for _, tl := range tools.Tools {
		name := tl.Name
		a, ok := args[name]
		if !ok {
			// Not covered yet; skip but log for visibility
			t.Logf("[skip] no smoke args for tool: %s", name)
			continue
		}
		params := map[string]interface{}{"name": name, "arguments": a}
		resp, raw, callErr := sendRPC(t, stdin, reader, 1000+failed, "tools/call", params)
		if callErr != nil || (resp != nil && resp.Error != nil) {
			failed++
			t.Errorf("tool %s failed: err=%v rpcErr=%+v raw=%s", name, callErr, resp.Error, string(raw))
		} else {
			t.Logf("[ok] %s", name)
		}
	}

	// Shutdown
	_ = stdin.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("mcp server did not exit in time; stderr=%s", stderr.String())
	case err := <-done:
		if err != nil {
			t.Fatalf("mcp server exit err: %v; stderr=%s", err, stderr.String())
		}
	}

	if failed > 0 {
		t.Fatalf("%d MCP tools failed smoke test", failed)
	}
}
