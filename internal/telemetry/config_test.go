package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_NewConfig(t *testing.T) {
	// Use temp directory for test isolation
	tmpDir := t.TempDir()
	SetConfigDir(tmpDir)
	defer SetConfigDir("")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should return defaults
	if cfg.Enabled {
		t.Error("new config should have Enabled = false")
	}
	if cfg.ConsentAsked {
		t.Error("new config should have ConsentAsked = false")
	}
	if cfg.AnonymousID == "" {
		t.Error("new config should have generated AnonymousID")
	}

	// UUID should be valid format (36 chars with hyphens)
	if len(cfg.AnonymousID) != 36 {
		t.Errorf("AnonymousID should be UUID format, got length %d", len(cfg.AnonymousID))
	}
}

func TestSave_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	SetConfigDir(tmpDir)
	defer SetConfigDir("")

	cfg := &Config{
		Enabled:      true,
		ConsentAsked: true,
		AnonymousID:  "test-uuid-1234",
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Verify file permissions
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	// On Unix, check that permissions are 0600
	// Note: Windows doesn't support Unix permissions the same way
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", info.Mode().Perm())
	}

	// Verify content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if loaded.Enabled != cfg.Enabled {
		t.Errorf("Enabled = %v, want %v", loaded.Enabled, cfg.Enabled)
	}
	if loaded.ConsentAsked != cfg.ConsentAsked {
		t.Errorf("ConsentAsked = %v, want %v", loaded.ConsentAsked, cfg.ConsentAsked)
	}
	if loaded.AnonymousID != cfg.AnonymousID {
		t.Errorf("AnonymousID = %v, want %v", loaded.AnonymousID, cfg.AnonymousID)
	}
}

func TestLoad_ExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	SetConfigDir(tmpDir)
	defer SetConfigDir("")

	// Create existing config
	existing := Config{
		Enabled:      true,
		ConsentAsked: true,
		AnonymousID:  "existing-uuid-5678",
	}
	data, _ := json.Marshal(existing)
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load should read existing
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Enabled != existing.Enabled {
		t.Errorf("Enabled = %v, want %v", cfg.Enabled, existing.Enabled)
	}
	if cfg.ConsentAsked != existing.ConsentAsked {
		t.Errorf("ConsentAsked = %v, want %v", cfg.ConsentAsked, existing.ConsentAsked)
	}
	if cfg.AnonymousID != existing.AnonymousID {
		t.Errorf("AnonymousID = %v, want %v", cfg.AnonymousID, existing.AnonymousID)
	}
}

func TestLoad_GeneratesUUID_WhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	SetConfigDir(tmpDir)
	defer SetConfigDir("")

	// Create config without anonymous ID
	existing := Config{
		Enabled:      true,
		ConsentAsked: true,
		AnonymousID:  "", // Missing
	}
	data, _ := json.Marshal(existing)
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have generated a UUID
	if cfg.AnonymousID == "" {
		t.Error("should have generated AnonymousID when missing")
	}
	if len(cfg.AnonymousID) != 36 {
		t.Errorf("AnonymousID should be UUID format, got length %d", len(cfg.AnonymousID))
	}
}

func TestConfig_Enable(t *testing.T) {
	cfg := &Config{
		Enabled:      false,
		ConsentAsked: false,
	}

	cfg.Enable()

	if !cfg.Enabled {
		t.Error("Enable() should set Enabled = true")
	}
	if !cfg.ConsentAsked {
		t.Error("Enable() should set ConsentAsked = true")
	}
}

func TestConfig_Disable(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		ConsentAsked: false,
	}

	cfg.Disable()

	if cfg.Enabled {
		t.Error("Disable() should set Enabled = false")
	}
	if !cfg.ConsentAsked {
		t.Error("Disable() should set ConsentAsked = true")
	}
}

func TestConfig_NeedsConsent(t *testing.T) {
	tests := []struct {
		name         string
		consentAsked bool
		want         bool
	}{
		{"needs consent when not asked", false, true},
		{"no consent needed when already asked", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ConsentAsked: tt.consentAsked}
			if got := cfg.NeedsConsent(); got != tt.want {
				t.Errorf("NeedsConsent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"returns true when enabled", true, true},
		{"returns false when disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Enabled: tt.enabled}
			if got := cfg.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a subdirectory that doesn't exist yet
	nestedDir := filepath.Join(tmpDir, "nested", "config")
	SetConfigDir(nestedDir)
	defer SetConfigDir("")

	cfg := &Config{
		Enabled:      true,
		ConsentAsked: true,
		AnonymousID:  "test-uuid",
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Directory should have been created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Error("Save() should create nested directories")
	}
}

func TestGetConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	SetConfigDir(tmpDir)
	defer SetConfigDir("")

	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath() error = %v", err)
	}

	expected := filepath.Join(tmpDir, ConfigFileName)
	if path != expected {
		t.Errorf("GetConfigPath() = %v, want %v", path, expected)
	}
}

func TestRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	SetConfigDir(tmpDir)
	defer SetConfigDir("")

	// Create and save
	original := &Config{
		Enabled:      true,
		ConsentAsked: true,
		AnonymousID:  "roundtrip-uuid-9999",
	}

	if err := original.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load back
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify all fields match
	if loaded.Enabled != original.Enabled {
		t.Errorf("Enabled = %v, want %v", loaded.Enabled, original.Enabled)
	}
	if loaded.ConsentAsked != original.ConsentAsked {
		t.Errorf("ConsentAsked = %v, want %v", loaded.ConsentAsked, original.ConsentAsked)
	}
	if loaded.AnonymousID != original.AnonymousID {
		t.Errorf("AnonymousID = %v, want %v", loaded.AnonymousID, original.AnonymousID)
	}
}
