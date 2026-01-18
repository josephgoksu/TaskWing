package task

import (
	"testing"

	"github.com/spf13/viper"
)

func TestGetScopeConfig_Defaults(t *testing.T) {
	// Reset for clean test state
	ResetScopeConfig()
	viper.Reset()

	cfg := GetScopeConfig()

	// Verify defaults are loaded
	scopes := cfg.GetScopes()
	if len(scopes) == 0 {
		t.Error("Expected default scopes to be loaded")
	}

	// Check a known default scope
	authKeywords, ok := scopes["auth"]
	if !ok {
		t.Error("Expected 'auth' scope in defaults")
	}
	if len(authKeywords) == 0 {
		t.Error("Expected auth scope to have keywords")
	}

	// Verify default limits
	if cfg.MaxKeywords() != defaultMaxKeywords {
		t.Errorf("Expected maxKeywords=%d, got %d", defaultMaxKeywords, cfg.MaxKeywords())
	}
	if cfg.MinWordLength() != defaultMinWordLen {
		t.Errorf("Expected minWordLength=%d, got %d", defaultMinWordLen, cfg.MinWordLength())
	}
}

func TestGetScopeConfig_CustomScopes(t *testing.T) {
	// Reset for clean test state
	ResetScopeConfig()
	viper.Reset()

	// Configure custom scopes
	viper.Set("task.scopes", map[string][]string{
		"custom_domain": {"keyword1", "keyword2", "keyword3"},
		"auth":          {"custom_auth_keyword"}, // Override default
	})

	cfg := GetScopeConfig()
	scopes := cfg.GetScopes()

	// Custom scope should exist
	customKw, ok := scopes["custom_domain"]
	if !ok {
		t.Error("Expected 'custom_domain' scope to be loaded from config")
	}
	if len(customKw) != 3 {
		t.Errorf("Expected 3 keywords in custom_domain, got %d", len(customKw))
	}

	// Auth scope should be overridden
	authKw := scopes["auth"]
	if len(authKw) != 1 || authKw[0] != "custom_auth_keyword" {
		t.Errorf("Expected auth scope to be overridden, got %v", authKw)
	}

	// Other default scopes should still exist (merged)
	if _, ok := scopes["api"]; !ok {
		t.Error("Expected default 'api' scope to still exist after merge")
	}
}

func TestGetScopeConfig_CustomLimits(t *testing.T) {
	// Reset for clean test state
	ResetScopeConfig()
	viper.Reset()

	// Configure custom limits
	viper.Set("task.maxKeywords", 20)
	viper.Set("task.minWordLength", 4)

	cfg := GetScopeConfig()

	if cfg.MaxKeywords() != 20 {
		t.Errorf("Expected maxKeywords=20, got %d", cfg.MaxKeywords())
	}
	if cfg.MinWordLength() != 4 {
		t.Errorf("Expected minWordLength=4, got %d", cfg.MinWordLength())
	}
}

func TestScopeConfig_InferScope(t *testing.T) {
	// Reset for clean test state
	ResetScopeConfig()
	viper.Reset()

	cfg := GetScopeConfig()

	tests := []struct {
		name     string
		words    map[string]bool
		expected string
	}{
		{
			name:     "auth scope detection",
			words:    map[string]bool{"login": true, "password": true, "jwt": true},
			expected: "auth",
		},
		{
			name:     "database scope detection",
			words:    map[string]bool{"db": true, "sql": true, "migration": true},
			expected: "database",
		},
		{
			name:     "api scope detection",
			words:    map[string]bool{"endpoint": true, "handler": true, "rest": true},
			expected: "api",
		},
		{
			name:     "general fallback",
			words:    map[string]bool{"random": true, "words": true, "here": true},
			expected: "general",
		},
		{
			name:     "short abbreviation matching",
			words:    map[string]bool{"ui": true, "tui": true},
			expected: "ui",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := cfg.InferScope(tc.words)
			if result != tc.expected {
				t.Errorf("Expected scope %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestEnrichAIFields_UsesConfigurableScopes(t *testing.T) {
	// Reset for clean test state
	ResetScopeConfig()
	viper.Reset()

	// Add custom scope
	viper.Set("task.scopes", map[string][]string{
		"payments": {"payment", "stripe", "checkout", "invoice"},
	})

	// Force reload
	ResetScopeConfig()

	task := &Task{
		Title:       "Implement Stripe payment integration",
		Description: "Add checkout flow with invoice generation using the Stripe API",
	}
	task.EnrichAIFields()

	if task.Scope != "payments" {
		t.Errorf("Expected scope 'payments' for payment-related task, got %q", task.Scope)
	}

	// Verify keywords were extracted
	if len(task.Keywords) == 0 {
		t.Error("Expected keywords to be extracted")
	}

	// Verify recall queries were generated
	if len(task.SuggestedRecallQueries) == 0 {
		t.Error("Expected recall queries to be generated")
	}
	// First query should include the inferred scope
	if task.SuggestedRecallQueries[0] != "payments patterns constraints decisions" {
		t.Errorf("Expected first query to include 'payments' scope, got %q", task.SuggestedRecallQueries[0])
	}
}
