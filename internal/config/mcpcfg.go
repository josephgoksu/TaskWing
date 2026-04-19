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

