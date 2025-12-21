package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestListCmd_Empty(t *testing.T) {
	// Setup generic temp dir for DB
	tmpDir, err := os.MkdirTemp("", "taskwing-test-list")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Configure viper to use this path
	viper.Set("memory.path", filepath.Join(tmpDir, ".taskwing/memory"))
	viper.Set("telemetry.disabled", true) // Prevent panic in test environment

	// Capture output
	b := bytes.NewBufferString("")
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	// Execute via Root to simulate real CLI usage
	// valid subcommand "list" should match listCmd and skip rootCmd.Run (os.Exit)
	rootCmd.SetArgs([]string{"list"})
	err = rootCmd.Execute()

	assert.NoError(t, err)
	output := b.String()

	// Assert functionality
	assert.Contains(t, output, "No knowledge nodes found")
	assert.Contains(t, output, "Add one with")
}
