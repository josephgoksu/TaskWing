// Package telemetry manages anonymous usage telemetry for TaskWing.
// See docs/reference/TELEMETRY_POLICY.md for privacy policy and data practices.
package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// ConfigFileName is the name of the telemetry configuration file.
const ConfigFileName = "telemetry.json"

// Config holds the telemetry state and user preferences.
// Stored at ~/.taskwing/telemetry.json (separate from main config).
type Config struct {
	// Enabled indicates whether telemetry is currently enabled.
	Enabled bool `json:"enabled"`

	// ConsentAsked indicates whether the user has been prompted for consent.
	// Once true, we won't prompt again (user made a choice).
	ConsentAsked bool `json:"consent_asked"`

	// AnonymousID is a random UUID for anonymous user identification.
	// Generated once on first load, never changes.
	// Not tied to any personally identifiable information.
	AnonymousID string `json:"anonymous_id"`
}

// configDirOverride allows tests to override the config directory.
// When empty, uses the default ~/.taskwing path.
var (
	configDirOverride   string
	configDirOverrideMu sync.RWMutex
)

// SetConfigDir sets a custom config directory path (for testing).
// Pass empty string to reset to default behavior.
func SetConfigDir(dir string) {
	configDirOverrideMu.Lock()
	defer configDirOverrideMu.Unlock()
	configDirOverride = dir
}

// getConfigDir returns the telemetry config directory.
// Uses override if set, otherwise ~/.taskwing.
func getConfigDir() (string, error) {
	configDirOverrideMu.RLock()
	override := configDirOverride
	configDirOverrideMu.RUnlock()

	if override != "" {
		return override, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".taskwing"), nil
}

// GetConfigPath returns the full path to the telemetry config file.
func GetConfigPath() (string, error) {
	dir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// Load reads the telemetry configuration from disk.
// If the file doesn't exist, returns a new Config with defaults.
// Generates an anonymous ID if one doesn't exist.
func Load() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("get config path: %w", err)
	}

	cfg := &Config{
		Enabled:      false, // Default: disabled until consent given
		ConsentAsked: false,
		AnonymousID:  "",
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - generate anonymous ID and return defaults
			cfg.AnonymousID = uuid.New().String()
			return cfg, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Ensure we always have an anonymous ID
	if cfg.AnonymousID == "" {
		cfg.AnonymousID = uuid.New().String()
	}

	return cfg, nil
}

// Save writes the telemetry configuration to disk.
// Creates the config directory if it doesn't exist.
// Uses secure file permissions (0600).
func (c *Config) Save() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Write with secure permissions (owner read/write only)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// Enable turns on telemetry and marks consent as given.
func (c *Config) Enable() {
	c.Enabled = true
	c.ConsentAsked = true
}

// Disable turns off telemetry and marks consent as given.
func (c *Config) Disable() {
	c.Enabled = false
	c.ConsentAsked = true
}

// NeedsConsent returns true if the user hasn't been asked about telemetry yet.
func (c *Config) NeedsConsent() bool {
	return !c.ConsentAsked
}

// IsEnabled returns true if telemetry is currently enabled.
func (c *Config) IsEnabled() bool {
	return c.Enabled
}
