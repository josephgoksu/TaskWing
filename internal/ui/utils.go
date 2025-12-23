package ui

import "os"

// IsInteractive checks if stdout is a terminal.
// This is useful to avoid prompting when piping output or running in non-interactive environments.
func IsInteractive() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
