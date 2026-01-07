package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// GetGlobalConfigDir returns the path to the global configuration directory (~/.taskwing).
// This is the source of truth for where global config lives.
// It's a variable to allow overriding in tests.
var GetGlobalConfigDir = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".taskwing"), nil
}

// GetMemoryBasePath returns the path to the memory directory.
// Resolution order (first match wins):
// 1. Explicit config via "memory.path" (Viper/env/flag)
// 2. Local project directory: .taskwing/memory (if exists)
// 3. XDG_DATA_HOME/taskwing/memory (if XDG_DATA_HOME is set)
// 4. Global fallback: ~/.taskwing/memory
func GetMemoryBasePath() string {
	// 1. Check Viper config (flags/config file/env)
	if path := viper.GetString("memory.path"); path != "" {
		return path
	}

	// 2. Check for local project .taskwing/memory directory
	// This allows per-project isolation when running from within a project
	localMemory := ".taskwing/memory"
	if info, err := os.Stat(localMemory); err == nil && info.IsDir() {
		return localMemory
	}

	// 3. Check XDG_DATA_HOME
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "taskwing", "memory")
	}

	// 4. Fallback to ~/.taskwing/memory (global)
	dir, err := GetGlobalConfigDir()
	if err != nil {
		return "./memory"
	}
	return filepath.Join(dir, "memory")
}
