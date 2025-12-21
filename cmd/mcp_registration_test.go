package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMCPStub ensures that 'make test-mcp' has at least one test to run.
// Real MCP tests should be implemented here to verify JSON-RPC behavior.
func TestMCP_Stub(t *testing.T) {
	// Verify mcp command is registered
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "mcp" {
			found = true
			break
		}
	}
	assert.True(t, found, "mcp command must be registered on root")
}
