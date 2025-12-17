/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize TaskWing in the current directory",
	Long: `Initialize TaskWing project memory in the current directory.

This creates the .taskwing/memory directory with:
  • memory.db - SQLite database for features and decisions
  • features/ - Markdown files for human-readable documentation
  • index.json - Cache for fast MCP context loading

Run this in your project root before using other TaskWing commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current directory: %w", err)
		}

		memoryPath := GetMemoryBasePath()

		// Check if already initialized
		if _, err := os.Stat(filepath.Join(memoryPath, "memory.db")); err == nil {
			fmt.Println("✓ TaskWing already initialized in this directory")
			return nil
		}

		// Create memory store (this initializes the directory and database)
		store, err := memory.NewSQLiteStore(memoryPath)
		if err != nil {
			return fmt.Errorf("initialize memory store: %w", err)
		}
		defer store.Close()

		// Build initial index
		if err := store.RebuildIndex(); err != nil {
			return fmt.Errorf("build index: %w", err)
		}

		// Create .gitignore for memory.db
		gitignorePath := filepath.Join(memoryPath, ".gitignore")
		gitignoreContent := `# TaskWing generated/cache files
memory.db-journal
memory.db-wal
memory.db-shm
index.json
`
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not create .gitignore: %v\n", err)
		}

		// Warn if project .gitignore ignores .taskwing entirely (common but breaks sharing).
		if data, err := os.ReadFile(filepath.Join(cwd, ".gitignore")); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == ".taskwing" || trimmed == ".taskwing/" {
					fmt.Println("⚠️  Your project .gitignore ignores '.taskwing/'. TaskWing memory will not be committed.")
					fmt.Println("   Fix: remove that rule, and rely on '.taskwing/memory/.gitignore' to ignore cache files.")
					break
				}
			}
		}

		fmt.Println("✓ TaskWing initialized")
		fmt.Println("")
		fmt.Println("Created:")
		fmt.Printf("  • %s/memory.db\n", memoryPath)
		fmt.Printf("  • %s/features/\n", memoryPath)
		fmt.Printf("  • %s/index.json\n", memoryPath)
		fmt.Println("")
		fmt.Println("Next steps:")
		fmt.Println("  taskwing feature add \"Auth\" --oneliner \"Authentication system\"")
		fmt.Println("  taskwing decision add \"Auth\" \"Use JWT\" --reason \"Stateless scaling\"")
		fmt.Println("  taskwing bootstrap --preview")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
