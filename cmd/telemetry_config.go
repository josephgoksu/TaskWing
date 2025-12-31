/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/telemetry"
	"github.com/spf13/cobra"
)

var telemetryCmd = &cobra.Command{
	Use:   "telemetry",
	Short: "Manage telemetry settings",
	Long: `View and manage TaskWing's anonymous telemetry settings.

TaskWing collects anonymous usage statistics to improve the product.
No personal data or code is ever collected.

Use 'taskwing config telemetry status' to see current settings.`,
}

var telemetryStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current telemetry status",
	RunE: func(cmd *cobra.Command, args []string) error {
		consent, err := telemetry.GetConsentStatus()
		if err != nil {
			return fmt.Errorf("failed to read telemetry status: %w", err)
		}

		if consent == nil {
			fmt.Println("📊 Telemetry: not configured yet")
			fmt.Println("   Run any command to be prompted for consent.")
			return nil
		}

		if consent.Enabled {
			fmt.Println("📊 Telemetry: enabled")
			fmt.Printf("   Install ID: %s\n", consent.InstallID)
			fmt.Printf("   Consent given: %s\n", consent.ConsentDate.Format("2006-01-02"))
			fmt.Println()
			fmt.Println("   To disable: taskwing config telemetry disable")
		} else {
			fmt.Println("📊 Telemetry: disabled")
			fmt.Println()
			fmt.Println("   To enable: taskwing config telemetry enable")
		}

		return nil
	},
}

var telemetryEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable anonymous telemetry",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := telemetry.SetConsentStatus(true, GetVersion()); err != nil {
			return fmt.Errorf("failed to enable telemetry: %w", err)
		}
		fmt.Println("✅ Telemetry enabled. Thank you for helping improve TaskWing!")
		return nil
	},
}

var telemetryDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable anonymous telemetry",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := telemetry.SetConsentStatus(false, GetVersion()); err != nil {
			return fmt.Errorf("failed to disable telemetry: %w", err)
		}
		fmt.Println("✅ Telemetry disabled.")
		return nil
	},
}

// configCmd is the parent config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage TaskWing configuration",
	Long:  `View and manage TaskWing configuration settings.`,
}

func init() {
	// Add config command to root
	rootCmd.AddCommand(configCmd)

	// Add telemetry command under config
	configCmd.AddCommand(telemetryCmd)

	// Add subcommands under telemetry
	telemetryCmd.AddCommand(telemetryStatusCmd)
	telemetryCmd.AddCommand(telemetryEnableCmd)
	telemetryCmd.AddCommand(telemetryDisableCmd)
}
