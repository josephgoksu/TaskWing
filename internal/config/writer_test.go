package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// setupTestConfigDir overrides GetGlobalConfigDir to use a temp directory.
// Returns a cleanup function that restores the original.
func setupTestConfigDir(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	original := GetGlobalConfigDir

	GetGlobalConfigDir = func() (string, error) {
		return tmpDir, nil
	}

	return tmpDir, func() {
		GetGlobalConfigDir = original
	}
}

func TestSaveAPIKeyForProvider_Success(t *testing.T) {
	tmpDir, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Save a key
	err := SaveAPIKeyForProvider("openai", "sk-test-key-12345")
	if err != nil {
		t.Fatalf("SaveAPIKeyForProvider() error = %v", err)
	}

	// Verify the config file was created
	configPath := filepath.Join(tmpDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Read the config and verify the key is stored correctly
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	savedKey := v.GetString("llm.apiKeys.openai")
	if savedKey != "sk-test-key-12345" {
		t.Errorf("saved key = %q, want %q", savedKey, "sk-test-key-12345")
	}
}

func TestSaveAPIKeyForProvider_EmptyProvider(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	err := SaveAPIKeyForProvider("", "some-key")
	if err == nil {
		t.Fatal("expected error for empty provider")
	}
	if !strings.Contains(err.Error(), "provider cannot be empty") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "provider cannot be empty")
	}
}

func TestSaveAPIKeyForProvider_EmptyKey(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	err := SaveAPIKeyForProvider("openai", "")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !strings.Contains(err.Error(), "API key cannot be empty") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "API key cannot be empty")
	}
}

func TestSaveAPIKeyForProvider_MultipleProviders(t *testing.T) {
	tmpDir, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Save keys for multiple providers
	providers := map[string]string{
		"openai":    "sk-openai-key",
		"anthropic": "sk-anthropic-key",
		"gemini":    "gemini-api-key",
	}

	for provider, key := range providers {
		if err := SaveAPIKeyForProvider(provider, key); err != nil {
			t.Fatalf("SaveAPIKeyForProvider(%q) error = %v", provider, err)
		}
	}

	// Verify all keys are stored
	v := viper.New()
	v.SetConfigFile(filepath.Join(tmpDir, "config.yaml"))
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	for provider, expectedKey := range providers {
		savedKey := v.GetString("llm.apiKeys." + provider)
		if savedKey != expectedKey {
			t.Errorf("key for %q = %q, want %q", provider, savedKey, expectedKey)
		}
	}
}

func TestSaveAPIKeyForProvider_UpdatesExistingKey(t *testing.T) {
	tmpDir, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Save initial key
	if err := SaveAPIKeyForProvider("openai", "old-key"); err != nil {
		t.Fatalf("initial save error = %v", err)
	}

	// Update with new key
	if err := SaveAPIKeyForProvider("openai", "new-key"); err != nil {
		t.Fatalf("update save error = %v", err)
	}

	// Verify the key was updated
	v := viper.New()
	v.SetConfigFile(filepath.Join(tmpDir, "config.yaml"))
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	savedKey := v.GetString("llm.apiKeys.openai")
	if savedKey != "new-key" {
		t.Errorf("saved key = %q, want %q", savedKey, "new-key")
	}
}

func TestSaveAPIKeyForProvider_PreservesOtherConfig(t *testing.T) {
	tmpDir, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Create existing config with other settings
	configPath := filepath.Join(tmpDir, "config.yaml")
	existingConfig := `version: "1"
llm:
  provider: gemini
  model: gemini-flash
verbose: true
`
	if err := os.WriteFile(configPath, []byte(existingConfig), 0600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// Save API key
	if err := SaveAPIKeyForProvider("openai", "sk-test"); err != nil {
		t.Fatalf("SaveAPIKeyForProvider() error = %v", err)
	}

	// Verify other settings were preserved
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if v.GetString("llm.provider") != "gemini" {
		t.Errorf("provider was modified, got %q", v.GetString("llm.provider"))
	}
	if v.GetString("llm.model") != "gemini-flash" {
		t.Errorf("model was modified, got %q", v.GetString("llm.model"))
	}
	if !v.GetBool("verbose") {
		t.Error("verbose setting was not preserved")
	}
}

func TestDeleteAPIKeyForProvider_Success(t *testing.T) {
	tmpDir, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// First save a key
	if err := SaveAPIKeyForProvider("openai", "sk-to-delete"); err != nil {
		t.Fatalf("SaveAPIKeyForProvider() error = %v", err)
	}

	// Delete it
	err := DeleteAPIKeyForProvider("openai")
	if err != nil {
		t.Fatalf("DeleteAPIKeyForProvider() error = %v", err)
	}

	// Verify the key is gone
	v := viper.New()
	v.SetConfigFile(filepath.Join(tmpDir, "config.yaml"))
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if v.IsSet("llm.apiKeys.openai") {
		t.Error("key should have been deleted but still exists")
	}
}

func TestDeleteAPIKeyForProvider_EmptyProvider(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	err := DeleteAPIKeyForProvider("")
	if err == nil {
		t.Fatal("expected error for empty provider")
	}
	if !strings.Contains(err.Error(), "provider cannot be empty") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "provider cannot be empty")
	}
}

func TestDeleteAPIKeyForProvider_NoConfigFile(t *testing.T) {
	_, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// No config file exists - should be a no-op, not an error
	err := DeleteAPIKeyForProvider("openai")
	if err != nil {
		t.Fatalf("DeleteAPIKeyForProvider() should not error when no config exists, got: %v", err)
	}
}

func TestDeleteAPIKeyForProvider_KeyNotExists(t *testing.T) {
	tmpDir, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Create config without the key we'll try to delete
	configPath := filepath.Join(tmpDir, "config.yaml")
	existingConfig := `version: "1"
llm:
  provider: gemini
`
	if err := os.WriteFile(configPath, []byte(existingConfig), 0600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// Delete non-existent key - should be a no-op
	err := DeleteAPIKeyForProvider("openai")
	if err != nil {
		t.Fatalf("DeleteAPIKeyForProvider() should not error for non-existent key, got: %v", err)
	}
}

func TestDeleteAPIKeyForProvider_PreservesOtherKeys(t *testing.T) {
	tmpDir, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Save multiple keys
	if err := SaveAPIKeyForProvider("openai", "openai-key"); err != nil {
		t.Fatalf("save openai error = %v", err)
	}
	if err := SaveAPIKeyForProvider("anthropic", "anthropic-key"); err != nil {
		t.Fatalf("save anthropic error = %v", err)
	}

	// Delete only one
	if err := DeleteAPIKeyForProvider("openai"); err != nil {
		t.Fatalf("DeleteAPIKeyForProvider() error = %v", err)
	}

	// Verify the other key is still there
	v := viper.New()
	v.SetConfigFile(filepath.Join(tmpDir, "config.yaml"))
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if v.IsSet("llm.apiKeys.openai") {
		t.Error("openai key should have been deleted")
	}
	if !v.IsSet("llm.apiKeys.anthropic") {
		t.Error("anthropic key should still exist")
	}
	if v.GetString("llm.apiKeys.anthropic") != "anthropic-key" {
		t.Errorf("anthropic key = %q, want %q", v.GetString("llm.apiKeys.anthropic"), "anthropic-key")
	}
}

func TestDeleteAPIKeyForProvider_PreservesOtherConfig(t *testing.T) {
	tmpDir, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Create config with other settings and a key
	configPath := filepath.Join(tmpDir, "config.yaml")
	existingConfig := `version: "1"
llm:
  provider: openai
  model: gpt-4
  apiKeys:
    openai: sk-to-delete
verbose: true
`
	if err := os.WriteFile(configPath, []byte(existingConfig), 0600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// Delete the key
	if err := DeleteAPIKeyForProvider("openai"); err != nil {
		t.Fatalf("DeleteAPIKeyForProvider() error = %v", err)
	}

	// Verify other settings were preserved
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if v.GetString("llm.provider") != "openai" {
		t.Errorf("provider was modified, got %q", v.GetString("llm.provider"))
	}
	if v.GetString("llm.model") != "gpt-4" {
		t.Errorf("model was modified, got %q", v.GetString("llm.model"))
	}
	if !v.GetBool("verbose") {
		t.Error("verbose setting was not preserved")
	}
}

// TestAPIKeyNotInMemoryDB verifies that API key functions don't touch memory.db.
// This is a security constraint - API keys must ONLY be stored in user config files.
func TestAPIKeyNotInMemoryDB(t *testing.T) {
	tmpDir, cleanup := setupTestConfigDir(t)
	defer cleanup()

	// Create a fake memory.db file to verify it's not touched
	memoryDir := filepath.Join(tmpDir, ".taskwing", "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatalf("failed to create memory dir: %v", err)
	}

	memoryDBPath := filepath.Join(memoryDir, "memory.db")
	originalContent := []byte("fake db content - should not be modified")
	if err := os.WriteFile(memoryDBPath, originalContent, 0600); err != nil {
		t.Fatalf("failed to create fake memory.db: %v", err)
	}

	// Perform API key operations
	if err := SaveAPIKeyForProvider("openai", "test-key"); err != nil {
		t.Fatalf("SaveAPIKeyForProvider() error = %v", err)
	}
	if err := DeleteAPIKeyForProvider("openai"); err != nil {
		t.Fatalf("DeleteAPIKeyForProvider() error = %v", err)
	}

	// Verify memory.db was not modified
	content, err := os.ReadFile(memoryDBPath)
	if err != nil {
		t.Fatalf("failed to read memory.db: %v", err)
	}

	if string(content) != string(originalContent) {
		t.Error("memory.db was modified by API key operations - SECURITY VIOLATION")
	}
}
