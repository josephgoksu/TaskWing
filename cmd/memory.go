/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// memoryCmd represents the memory command
var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage project memory integrity",
	Long: `Manage the integrity of your project memory database.

Commands for checking, repairing, and rebuilding the memory store.

Examples:
  taskwing memory check      # Check for integrity issues
  taskwing memory repair     # Fix integrity issues
  taskwing memory rebuild    # Rebuild the index cache`,
}

// memory check command
var memoryCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check memory integrity",
	Long: `Validate the integrity of the project memory.

Checks for:
  • Missing markdown files
  • Orphan edges (relationships to non-existent features)
  • Index cache staleness`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		issues, err := store.Check()
		if err != nil {
			return fmt.Errorf("check integrity: %w", err)
		}

		if viper.GetBool("json") {
			output, _ := json.MarshalIndent(map[string]interface{}{
				"issues": issues,
				"count":  len(issues),
			}, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		if len(issues) == 0 {
			fmt.Println("✓ No integrity issues found")
			return nil
		}

		fmt.Printf("Found %d issues:\n\n", len(issues))
		for i, issue := range issues {
			fmt.Printf("%d. [%s] %s\n", i+1, issue.Type, issue.Message)
			if issue.FeatureID != "" {
				fmt.Printf("   Feature: %s\n", issue.FeatureID)
			}
		}

		fmt.Println("\nRun 'taskwing memory repair' to fix these issues.")
		return nil
	},
}

// memory repair command
var memoryRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair integrity issues",
	Long: `Attempt to fix integrity issues in project memory.

Actions:
  • Regenerate missing markdown files from SQLite data
  • Remove orphan edges
  • Rebuild the index cache`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		// First check what needs repair
		issues, _ := store.Check()
		if len(issues) == 0 {
			fmt.Println("✓ No issues to repair")
			return nil
		}

		fmt.Printf("Repairing %d issues...\n", len(issues))

		if err := store.Repair(); err != nil {
			return fmt.Errorf("repair: %w", err)
		}

		// Verify repair
		remaining, _ := store.Check()
		if len(remaining) == 0 {
			fmt.Println("✓ All issues repaired")
		} else {
			fmt.Printf("⚠ %d issues remain after repair\n", len(remaining))
		}

		return nil
	},
}

// memory rebuild command
var memoryRebuildCmd = &cobra.Command{
	Use:     "rebuild-index",
	Aliases: []string{"rebuild"},
	Short:   "Rebuild the index cache",
	Long: `Regenerate the index.json cache from SQLite data.

This is useful if the cache is out of sync with the database.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		if err := store.RebuildIndex(); err != nil {
			return fmt.Errorf("rebuild index: %w", err)
		}

		index, _ := store.GetIndex()
		fmt.Printf("✓ Index rebuilt with %d features\n", len(index.Features))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(memoryCmd)

	// Add subcommands
	memoryCmd.AddCommand(memoryCheckCmd)
	memoryCmd.AddCommand(memoryRepairCmd)
	memoryCmd.AddCommand(memoryRebuildCmd)
}
