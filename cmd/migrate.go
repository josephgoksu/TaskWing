package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/migration"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate local .taskwing/ data to global store",
	Long: `Migrate project data from a local .taskwing/ directory to the global store at ~/.taskwing/projects/.

This is needed after upgrading to v1.22.6+ which centralizes all storage.
After migration, you can safely delete the local .taskwing/ directory.

The command is idempotent and will not overwrite existing data in the global store
unless --force is used.`,
	RunE: runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().Bool("force", false, "Overwrite existing data in global store")
}

func runMigrate(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := migration.MigrateLocalToGlobal(cwd, force)
	if err != nil {
		return err
	}

	switch {
	case result.AlreadyMigrated:
		fmt.Println("Nothing to migrate. Global store already has data for this project.")
		fmt.Printf("   Store: %s\n", result.StorePath)
	case result.NoLocalDir:
		fmt.Println("No local .taskwing/ directory found. Nothing to migrate.")
	default:
		fmt.Println("Migrated successfully.")
		fmt.Printf("   From: %s\n", filepath.Join(cwd, ".taskwing"))
		fmt.Printf("   To:   %s\n", result.StorePath)
		for _, f := range result.FilesMigrated {
			fmt.Printf("   - %s\n", f)
		}
		fmt.Println()
		fmt.Println("You can now delete the local .taskwing/ directory:")
		fmt.Printf("   rm -rf %s\n", filepath.Join(cwd, ".taskwing"))
	}

	// Also check if global store path is properly registered
	storePath, err := config.GetProjectStorePath(cwd)
	if err == nil && storePath != "" {
		fmt.Printf("\n   Global store: %s\n", storePath)
	}

	return nil
}
