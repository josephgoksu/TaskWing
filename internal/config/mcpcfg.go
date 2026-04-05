// Package config - MCP server configuration and crash logging stubs.
package config

import "strings"

// CanonicalServerName is the canonical MCP server name used in AI tool configs.
const CanonicalServerName = "taskwing"

// legacyServerNames contains server names from previous versions that should be migrated.
var legacyServerNames = map[string]bool{
	"tw":              true,
	"taskwing-server": true,
	"taskwing-mcp":    true,
}

// IsLegacyServerName returns true if the name is a deprecated MCP server name.
func IsLegacyServerName(name string) bool {
	return legacyServerNames[name]
}

// ContainsCanonicalServerName checks if text contains the canonical server name.
func ContainsCanonicalServerName(text string) bool {
	return strings.Contains(text, CanonicalServerName)
}

// SetLastInput, SetVersion, HandlePanic are lightweight stubs
// for the removed crash-logging subsystem (internal/logger).

var lastInput string
var appVersion string
var basePath string
var lastCommand string

// SetLastInput records the last user input for diagnostics.
func SetLastInput(input string) { lastInput = input }

// SetVersion stores the app version for diagnostics.
func SetVersion(v string) { appVersion = v }

// SetBasePath records the project base path for diagnostics.
func SetBasePath(p string) { basePath = p }

// SetCommand records the current command for diagnostics.
func SetCommand(cmd string) { lastCommand = cmd }

// HandlePanic is a no-op recovery handler. The crash-logging subsystem was removed.
func HandlePanic() {
	// Intentionally empty. Previously wrote crash logs to disk.
	// Removed to reduce complexity. Panics propagate to the Go runtime.
}
