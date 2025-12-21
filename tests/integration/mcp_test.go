package integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMCPHandshake verifies that the MCP server starts up correctly
// and responds to initialization without blocking on telemetry prompts.
func TestMCPHandshake(t *testing.T) {
	// 1. Locate the binary
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}

	// Assuming the test is running from the module root or we can find the bin
	// We'll look for the binary in ./bin/taskwing (where `air` builds it)
	// or ./taskwing (where `make build` puts it).
	binPath := filepath.Join(cwd, "../..", "bin", "taskwing")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		// Fallback to local root
		binPath = filepath.Join(cwd, "../..", "taskwing")
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			t.Fatalf("Binary not found at %s or ./taskwing. Run 'make build' first.", binPath)
		}
	}

	// 2. Prepare the command
	cmd := exec.Command(binPath, "mcp")
	cmd.Dir = filepath.Dir(binPath) // Run in project root effectively

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to get stderr: %v", err)
	}

	// 3. Start the process
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
	}()

	// 4. Send Initialize Request
	// Minimal JSON-RPC init
	initReq := `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}},"id":1}`

	go func() {
		defer stdin.Close()
		_, _ = io.WriteString(stdin, initReq+"\n")
	}()

	// 5. Read output with timeout
	type response struct {
		JSONRPC string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   json.RawMessage `json:"error,omitempty"`
		ID      interface{}     `json:"id"`
	}

	done := make(chan error, 1)

	// Monitor stderr for forbidden prompts
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Log stderr for debugging
			t.Logf("[stderr] %s", line)

			if strings.Contains(line, "Do you want to enable anonymous telemetry") {
				done <- fmt.Errorf("FAIL: Telemetry prompt detected in stderr")
				return
			}
		}
	}()

	// Monitor stdout for JSON response
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			t.Logf("[stdout] %s", line)

			var resp response
			if err := json.Unmarshal([]byte(line), &resp); err == nil {
				if resp.ID != nil {
					// We got a response!
					done <- nil
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			done <- fmt.Errorf("Error reading stdout: %v", err)
		}
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Test Failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for MCP initialization response")
	}
}
