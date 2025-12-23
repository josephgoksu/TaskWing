package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// GetGlobalConfigDir returns the path to the global configuration directory (~/.taskwing).
// This is the source of truth for where global config lives.
func GetGlobalConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".taskwing"), nil
}

// GetMemoryBasePath returns the path to the memory directory.
// It prioritizes Viper config "memory.path", then XDG_DATA_HOME/taskwing/memory,
// then defaults to ~/.taskwing/memory.
func GetMemoryBasePath() string {
	// 1. Check Viper config (flags/config file)
	if path := viper.GetString("memory.path"); path != "" {
		return path
	}

	// 2. Check XDG_DATA_HOME
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "taskwing", "memory")
	}

	// 3. Fallback to ~/.taskwing/memory
	dir, err := GetGlobalConfigDir()
	if err != nil {
		return "./memory"
	}
	return filepath.Join(dir, "memory")
}
