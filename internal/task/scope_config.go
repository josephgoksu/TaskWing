package task

import (
	"sync"

	"github.com/spf13/viper"
)

// ScopeConfig holds the configurable scope keywords mapping.
// Projects can customize this via .taskwing.yaml or ~/.taskwing/config.yaml:
//
//	task:
//	  scopes:
//	    auth:
//	      - auth
//	      - authentication
//	      - login
//	    custom_domain:
//	      - domain_specific_keyword
//	      - another_keyword
//
// If no custom config is provided, defaultScopeKeywords is used.
type ScopeConfig struct {
	mu         sync.RWMutex
	scopes     map[string][]string
	maxKw      int
	minWordLen int
}

// defaultScopeKeywords provides scope keywords for common software engineering domains.
// These work well for Go, JavaScript, and general backend projects.
var defaultScopeKeywords = map[string][]string{
	"auth":         {"auth", "authentication", "login", "logout", "session", "cookie", "jwt", "token", "password", "credential", "oauth", "sso"},
	"api":          {"api", "endpoint", "handler", "route", "rest", "graphql", "grpc", "request", "response", "middleware"},
	"database":     {"database", "db", "sql", "sqlite", "postgres", "mysql", "migration", "schema", "query", "table", "index"},
	"vectorsearch": {"vector", "embedding", "lancedb", "similarity", "semantic", "search", "rag", "retrieval"},
	"llm":          {"llm", "openai", "claude", "gemini", "ollama", "prompt", "completion", "chat", "model", "inference"},
	"cli":          {"cli", "command", "flag", "cobra", "terminal", "argument", "subcommand"},
	"mcp":          {"mcp", "tool", "protocol", "context", "stdio", "jsonrpc"},
	"bootstrap":    {"bootstrap", "scan", "analyze", "extract", "discover", "pattern"},
	"ui":           {"ui", "tui", "interface", "display", "render", "bubbletea", "lipgloss"},
	"test":         {"test", "testing", "mock", "fixture", "assert", "benchmark", "coverage"},
}

// Default configuration values
const (
	defaultMaxKeywords     = 10
	defaultMinWordLen      = 3
	defaultMinWordLenScope = 2 // Shorter for scope matching (e.g., "db", "ui")
)

var (
	globalScopeConfig *ScopeConfig
	configOnce        sync.Once
)

// GetScopeConfig returns the global scope configuration, loading from viper if available.
// Thread-safe and lazily initialized.
func GetScopeConfig() *ScopeConfig {
	configOnce.Do(func() {
		globalScopeConfig = loadScopeConfig()
	})
	return globalScopeConfig
}

// ResetScopeConfig forces reload of scope config. Only use in tests.
func ResetScopeConfig() {
	configOnce = sync.Once{}
	globalScopeConfig = nil
}

// loadScopeConfig loads scope configuration from viper with fallback to defaults.
func loadScopeConfig() *ScopeConfig {
	cfg := &ScopeConfig{
		scopes:     make(map[string][]string),
		maxKw:      defaultMaxKeywords,
		minWordLen: defaultMinWordLen,
	}

	// Try to load custom scopes from viper config
	// Config path: task.scopes (map[string][]string)
	if customScopes := viper.GetStringMapStringSlice("task.scopes"); len(customScopes) > 0 {
		// Merge custom scopes with defaults (custom takes precedence)
		for scope, keywords := range defaultScopeKeywords {
			cfg.scopes[scope] = keywords
		}
		for scope, keywords := range customScopes {
			cfg.scopes[scope] = keywords
		}
	} else {
		// Use defaults only
		for scope, keywords := range defaultScopeKeywords {
			cfg.scopes[scope] = keywords
		}
	}

	// Load max keywords setting
	if maxKw := viper.GetInt("task.maxKeywords"); maxKw > 0 {
		cfg.maxKw = maxKw
	}

	// Load min word length setting
	if minLen := viper.GetInt("task.minWordLength"); minLen > 0 {
		cfg.minWordLen = minLen
	}

	return cfg
}

// GetScopes returns the scope to keywords mapping.
func (c *ScopeConfig) GetScopes() map[string][]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Return a copy to prevent mutation
	result := make(map[string][]string, len(c.scopes))
	for k, v := range c.scopes {
		result[k] = append([]string{}, v...)
	}
	return result
}

// MaxKeywords returns the maximum number of keywords to extract.
func (c *ScopeConfig) MaxKeywords() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxKw
}

// MinWordLength returns the minimum word length for keyword extraction.
func (c *ScopeConfig) MinWordLength() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.minWordLen
}

// MinWordLengthForScope returns the minimum word length for scope matching.
// This is intentionally shorter (2 chars) to match abbreviations like "db", "ui", "ai".
func (c *ScopeConfig) MinWordLengthForScope() int {
	return defaultMinWordLenScope
}

// InferScope determines the most likely scope for given words.
// Returns "general" if no strong match is found.
//
// Algorithm:
// 1. For each configured scope, count how many of its keywords appear in the text
// 2. The scope with the highest count wins
// 3. If no keywords match, return "general"
func (c *ScopeConfig) InferScope(words map[string]bool) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	scopeScores := make(map[string]int)
	for scope, scopeKws := range c.scopes {
		for _, kw := range scopeKws {
			if words[kw] {
				scopeScores[scope]++
			}
		}
	}

	bestScope := "general"
	bestScore := 0
	for scope, score := range scopeScores {
		if score > bestScore {
			bestScore = score
			bestScope = scope
		}
	}
	return bestScope
}
