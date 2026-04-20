package migration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/josephgoksu/TaskWing/internal/config"
)

// MigrateResult describes what happened during migration.
type MigrateResult struct {
	StorePath       string
	FilesMigrated   []string
	NoLocalDir      bool
	AlreadyMigrated bool
}

// MigrateLocalToGlobal moves data from {projectDir}/.taskwing/ to the global store.
// It copies memory.db, ARCHITECTURE.md, config.yaml, and version files.
// Does not overwrite existing files in the global store unless force is true.
func MigrateLocalToGlobal(projectDir string, force bool) (*MigrateResult, error) {
	localDir := filepath.Join(projectDir, ".taskwing")

	// Check if local .taskwing/ exists
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		return &MigrateResult{NoLocalDir: true}, nil
	}

	// Resolve global store path
	storePath, err := config.GetProjectStorePath(projectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve global store: %w", err)
	}

	// Files to migrate (source relative to .taskwing/, dest relative to store)
	migrations := []struct {
		src string // relative to localDir
		dst string // relative to storePath
	}{
		{"memory/memory.db", "memory.db"},
		{"memory/memory.db-wal", "memory.db-wal"},
		{"memory/memory.db-shm", "memory.db-shm"},
		{"memory/ARCHITECTURE.md", "ARCHITECTURE.md"},
		{"config.yaml", "config.yaml"},
		{"version", "version"},
	}

	var migrated []string
	skippedExisting := false

	for _, m := range migrations {
		srcPath := filepath.Join(localDir, m.src)
		dstPath := filepath.Join(storePath, m.dst)

		// Skip if source doesn't exist
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}

		// Check if destination already exists
		if _, err := os.Stat(dstPath); err == nil {
			if !force {
				skippedExisting = true
				continue
			}
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return nil, fmt.Errorf("copy %s: %w", m.src, err)
		}
		migrated = append(migrated, m.dst)
	}

	// Also migrate policies directory if it exists
	localPolicies := filepath.Join(localDir, "policies")
	if info, err := os.Stat(localPolicies); err == nil && info.IsDir() {
		dstPolicies := filepath.Join(storePath, "policies")
		if err := copyDir(localPolicies, dstPolicies, force); err != nil {
			return nil, fmt.Errorf("copy policies: %w", err)
		}
		migrated = append(migrated, "policies/")
	}

	if len(migrated) == 0 && skippedExisting {
		return &MigrateResult{StorePath: storePath, AlreadyMigrated: true}, nil
	}

	return &MigrateResult{
		StorePath:     storePath,
		FilesMigrated: migrated,
	}, nil
}

// AutoMigrateIfNeeded silently migrates local .taskwing/ data on version upgrade.
// Returns true if migration occurred.
func AutoMigrateIfNeeded(projectDir string) bool {
	localDB := filepath.Join(projectDir, ".taskwing", "memory", "memory.db")
	if _, err := os.Stat(localDB); os.IsNotExist(err) {
		return false
	}

	storePath, err := config.GetProjectStorePath(projectDir)
	if err != nil {
		return false
	}

	globalDB := filepath.Join(storePath, "memory.db")
	if _, err := os.Stat(globalDB); err == nil {
		return false // Already has data, don't overwrite
	}

	if err := copyFile(localDB, globalDB); err != nil {
		fmt.Fprintf(os.Stderr, "taskwing: auto-migration of local knowledge failed: %v\n", err)
		return false
	}

	// Also copy ARCHITECTURE.md if present
	localArch := filepath.Join(projectDir, ".taskwing", "memory", "ARCHITECTURE.md")
	globalArch := filepath.Join(storePath, "ARCHITECTURE.md")
	if _, err := os.Stat(localArch); err == nil {
		_ = copyFile(localArch, globalArch)
	}

	fmt.Fprintf(os.Stderr, "taskwing: migrated local knowledge to %s\n", storePath)
	return true
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDir(src, dst string, force bool) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0700)
		}

		if _, err := os.Stat(dstPath); err == nil && !force {
			return nil // Skip existing
		}

		return copyFile(path, dstPath)
	})
}
