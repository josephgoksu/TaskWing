// Package app provides the application layer that orchestrates business logic.
// This layer sits between CLI/MCP handlers and the service layer, ensuring
// a single source of truth for all operations. CLI and MCP become thin adapters.
package app

import (
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// Context holds shared dependencies for all app services.
// It provides a consistent way to access repository and configuration
// across all application operations.
type Context struct {
	Repo   *memory.Repository
	LLMCfg llm.Config
}

// NewContext creates an app context with standard initialization.
// LLM config loading is best-effort - operations continue with empty config
// if loading fails (LLM features will be disabled but basic features work).
// NOTE: For role-specific config (query, bootstrap), use NewContextForRole instead.
func NewContext(repo *memory.Repository) *Context {
	llmCfg, err := config.LoadLLMConfig()
	if err != nil {
		// Non-fatal: continue with empty config
		llmCfg = llm.Config{}
	}
	return &Context{Repo: repo, LLMCfg: llmCfg}
}

// NewContextForRole creates an app context with role-aware LLM config.
// This respects llm.models.<role> configuration (e.g., llm.models.query: "gemini:gemini-3-flash").
// Use this in MCP handlers to match CLI behavior for query vs bootstrap operations.
func NewContextForRole(repo *memory.Repository, role llm.ModelRole) *Context {
	llmCfg, err := config.LoadLLMConfigForRole(role)
	if err != nil {
		// Non-fatal: continue with empty config
		llmCfg = llm.Config{}
	}
	return &Context{Repo: repo, LLMCfg: llmCfg}
}

// NewContextWithConfig creates an app context with explicit LLM config.
// Use this when you already have the config (e.g., from CLI flags).
func NewContextWithConfig(repo *memory.Repository, llmCfg llm.Config) *Context {
	return &Context{Repo: repo, LLMCfg: llmCfg}
}
