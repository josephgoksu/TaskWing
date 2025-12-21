package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestRootCmd(t *testing.T) {
	// Reset flags and config
	viper.Reset()

	// Capture output
	b := bytes.NewBufferString("")
	rootCmd.SetOut(b)
	rootCmd.SetErr(b)

	// Test --help to ensure banner/template works
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	assert.NoError(t, err)

	output := b.String()
	assert.Contains(t, output, "TaskWing - Knowledge Graph") // Short desc
	assert.Contains(t, output, "Usage:")
	assert.Contains(t, output, "Commands:")
}

func TestVersion(t *testing.T) {
	// We don't have a specific version command logic exposed as a function in root.go other than GetVersion
	// checking GetVersion
	v := GetVersion()
	assert.Equal(t, "2.0.0", v)
}
