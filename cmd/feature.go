/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// featureCmd represents the feature command
var featureCmd = &cobra.Command{
	Use:   "feature",
	Short: "Manage project features",
	Long: `Manage features in your project memory.

Features are the major components or capabilities in your codebase.
Each feature can have decisions attached explaining why it exists.

Examples:
  taskwing feature add "Auth" --oneliner "User authentication system"
  taskwing feature list
  taskwing feature show "Auth"
  taskwing feature link "Users" --to "Auth"`,
}

// feature add command
var featureAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new feature",
	Long: `Add a new feature to project memory.

Example:
  taskwing feature add "Auth" --oneliner "User authentication and authorization"
  taskwing feature add "Payments" --oneliner "Payment processing" --tags "core,billing"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		oneliner, _ := cmd.Flags().GetString("oneliner")
		tagsStr, _ := cmd.Flags().GetString("tags")
		status, _ := cmd.Flags().GetString("status")

		if oneliner == "" {
			return fmt.Errorf("--oneliner is required")
		}

		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		var tags []string
		if tagsStr != "" {
			tags = strings.Split(tagsStr, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
		}

		if status == "" {
			status = memory.FeatureStatusActive
		}

		f := memory.Feature{
			Name:     name,
			OneLiner: oneliner,
			Tags:     tags,
			Status:   status,
		}

		if err := store.CreateFeature(f); err != nil {
			return fmt.Errorf("create feature: %w", err)
		}

		if viper.GetBool("json") {
			output, _ := json.MarshalIndent(map[string]string{
				"status":  "created",
				"feature": name,
			}, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("✓ Feature '%s' created\n", name)
		}

		return nil
	},
}

// feature list command
var featureListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all features",
	Long:  `List all features in project memory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		features, err := store.ListFeatures()
		if err != nil {
			return fmt.Errorf("list features: %w", err)
		}

		if viper.GetBool("json") {
			output, _ := json.MarshalIndent(features, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		if len(features) == 0 {
			fmt.Println("No features found. Add one with: taskwing feature add \"Name\" --oneliner \"Description\"")
			return nil
		}

		fmt.Printf("Features (%d):\n", len(features))
		for _, f := range features {
			statusIcon := "●"
			switch f.Status {
			case memory.FeatureStatusActive:
				statusIcon = "🟢"
			case memory.FeatureStatusDeprecated:
				statusIcon = "🔴"
			case memory.FeatureStatusPlanned:
				statusIcon = "🟡"
			}
			fmt.Printf("  %s %s - %s", statusIcon, f.Name, f.OneLiner)
			if f.DecisionCount > 0 {
				fmt.Printf(" (%d decisions)", f.DecisionCount)
			}
			fmt.Println()
		}

		return nil
	},
}

// feature show command
var featureShowCmd = &cobra.Command{
	Use:   "show <name-or-id>",
	Short: "Show feature details",
	Long:  `Show detailed information about a feature including its decisions.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]

		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		// Try to find by name first
		features, err := store.ListFeatures()
		if err != nil {
			return fmt.Errorf("list features: %w", err)
		}

		var featureID string
		for _, f := range features {
			if strings.EqualFold(f.Name, nameOrID) || f.ID == nameOrID {
				featureID = f.ID
				break
			}
		}

		if featureID == "" {
			return fmt.Errorf("feature not found: %s", nameOrID)
		}

		feature, err := store.GetFeature(featureID)
		if err != nil {
			return fmt.Errorf("get feature: %w", err)
		}

		decisions, err := store.GetDecisions(featureID)
		if err != nil {
			return fmt.Errorf("get decisions: %w", err)
		}

		deps, _ := store.GetDependencies(featureID)
		dependents, _ := store.GetDependents(featureID)

		if viper.GetBool("json") {
			output := map[string]interface{}{
				"feature":    feature,
				"decisions":  decisions,
				"dependsOn":  deps,
				"dependedBy": dependents,
			}
			jsonBytes, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(jsonBytes))
			return nil
		}

		fmt.Printf("# %s\n", feature.Name)
		fmt.Printf("%s\n\n", feature.OneLiner)
		fmt.Printf("ID:     %s\n", feature.ID)
		fmt.Printf("Status: %s\n", feature.Status)
		if len(feature.Tags) > 0 {
			fmt.Printf("Tags:   %s\n", strings.Join(feature.Tags, ", "))
		}

		if len(decisions) > 0 {
			fmt.Printf("\n## Decisions (%d)\n", len(decisions))
			for _, d := range decisions {
				fmt.Printf("  • %s\n", d.Title)
				fmt.Printf("    %s\n", d.Summary)
			}
		}

		if len(deps) > 0 {
			fmt.Printf("\n## Depends On\n")
			for _, dep := range deps {
				fmt.Printf("  → %s\n", dep)
			}
		}

		if len(dependents) > 0 {
			fmt.Printf("\n## Depended By\n")
			for _, dep := range dependents {
				fmt.Printf("  ← %s\n", dep)
			}
		}

		return nil
	},
}

// feature update command
var featureUpdateCmd = &cobra.Command{
	Use:   "update <name-or-id>",
	Short: "Update a feature",
	Long: `Update a feature's metadata in project memory.

Examples:
  taskwing feature update "Auth" --oneliner "OAuth2 + JWT authentication"
  taskwing feature update "Auth" --status deprecated
  taskwing feature update "Auth" --tags "core,security"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]

		oneliner, _ := cmd.Flags().GetString("oneliner")
		tagsStr, _ := cmd.Flags().GetString("tags")
		status, _ := cmd.Flags().GetString("status")

		onelinerSet := cmd.Flags().Changed("oneliner")
		tagsSet := cmd.Flags().Changed("tags")
		statusSet := cmd.Flags().Changed("status")

		if !onelinerSet && !tagsSet && !statusSet {
			return fmt.Errorf("at least one of --oneliner, --tags, or --status is required")
		}
		if onelinerSet && oneliner == "" {
			return fmt.Errorf("--oneliner cannot be empty")
		}
		if statusSet && status == "" {
			return fmt.Errorf("--status cannot be empty")
		}

		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		features, err := store.ListFeatures()
		if err != nil {
			return fmt.Errorf("list features: %w", err)
		}

		var featureID string
		for _, f := range features {
			if strings.EqualFold(f.Name, nameOrID) || f.ID == nameOrID {
				featureID = f.ID
				break
			}
		}
		if featureID == "" {
			return fmt.Errorf("feature not found: %s", nameOrID)
		}

		f, err := store.GetFeature(featureID)
		if err != nil {
			return fmt.Errorf("get feature: %w", err)
		}

		if onelinerSet {
			f.OneLiner = oneliner
		}

		if tagsSet {
			if tagsStr == "" {
				f.Tags = nil
			} else {
				parts := strings.Split(tagsStr, ",")
				tags := make([]string, 0, len(parts))
				for _, p := range parts {
					tag := strings.TrimSpace(p)
					if tag == "" {
						continue
					}
					tags = append(tags, tag)
				}
				f.Tags = tags
			}
		}

		if statusSet {
			switch status {
			case memory.FeatureStatusActive, memory.FeatureStatusDeprecated, memory.FeatureStatusPlanned:
				f.Status = status
			default:
				return fmt.Errorf("invalid --status: %s (expected: active, deprecated, planned)", status)
			}
		}

		if err := store.UpdateFeature(*f); err != nil {
			return fmt.Errorf("update feature: %w", err)
		}

		if viper.GetBool("json") {
			output, _ := json.MarshalIndent(map[string]string{
				"status":  "updated",
				"feature": f.Name,
				"id":      f.ID,
			}, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		fmt.Printf("✓ Feature '%s' updated\n", f.Name)
		return nil
	},
}

// feature delete command
var featureDeleteCmd = &cobra.Command{
	Use:   "delete <name-or-id>",
	Short: "Delete a feature",
	Long: `Delete a feature from project memory.

Note: Cannot delete a feature that has dependents (other features that depend on it).
Delete the dependents first or unlink them.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		// Find feature ID
		features, _ := store.ListFeatures()
		var featureID, featureName string
		for _, f := range features {
			if strings.EqualFold(f.Name, nameOrID) || f.ID == nameOrID {
				featureID = f.ID
				featureName = f.Name
				break
			}
		}

		if featureID == "" {
			return fmt.Errorf("feature not found: %s", nameOrID)
		}

		if !force {
			fmt.Printf("Delete feature '%s'? This cannot be undone. Use --force to confirm.\n", featureName)
			return nil
		}

		if err := store.DeleteFeature(featureID); err != nil {
			return fmt.Errorf("delete feature: %w", err)
		}

		fmt.Printf("✓ Feature '%s' deleted\n", featureName)
		return nil
	},
}

// feature link command
var featureLinkCmd = &cobra.Command{
	Use:   "link <from-feature>",
	Short: "Create a relationship between features",
	Long: `Create a relationship between two features.

Relationship types:
  depends_on - "from" requires "to" to function
  extends    - "from" adds capabilities to "to"
  replaces   - "from" supersedes "to"
  related    - loose association

Example:
  taskwing feature link "Users" --depends-on "Auth"
  taskwing feature link "Users" --to "Auth" --type depends_on`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromName := args[0]
		toName, _ := cmd.Flags().GetString("to")
		relType, _ := cmd.Flags().GetString("type")

		dependsOn, _ := cmd.Flags().GetString("depends-on")
		extends, _ := cmd.Flags().GetString("extends")
		replaces, _ := cmd.Flags().GetString("replaces")
		related, _ := cmd.Flags().GetString("related")

		explicitTargets := []struct {
			value string
			rel   string
		}{
			{value: dependsOn, rel: memory.EdgeTypeDependsOn},
			{value: extends, rel: memory.EdgeTypeExtends},
			{value: replaces, rel: memory.EdgeTypeReplaces},
			{value: related, rel: memory.EdgeTypeRelated},
		}

		used := 0
		for _, t := range explicitTargets {
			if t.value != "" {
				toName = t.value
				relType = t.rel
				used++
			}
		}

		if used > 1 {
			return fmt.Errorf("choose exactly one relationship flag: --depends-on, --extends, --replaces, --related (or use --to with --type)")
		}

		if toName == "" {
			return fmt.Errorf("target feature is required (use --depends-on/--extends/--replaces/--related, or --to)")
		}
		if relType == "" {
			relType = memory.EdgeTypeDependsOn
		}

		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		// Resolve feature names to IDs
		features, _ := store.ListFeatures()
		var fromID, toID string
		for _, f := range features {
			if strings.EqualFold(f.Name, fromName) || f.ID == fromName {
				fromID = f.ID
			}
			if strings.EqualFold(f.Name, toName) || f.ID == toName {
				toID = f.ID
			}
		}

		if fromID == "" {
			return fmt.Errorf("feature not found: %s", fromName)
		}
		if toID == "" {
			return fmt.Errorf("feature not found: %s", toName)
		}

		if err := store.Link(fromID, toID, relType); err != nil {
			return fmt.Errorf("link features: %w", err)
		}

		fmt.Printf("✓ Linked: %s → %s (%s)\n", fromName, toName, relType)
		return nil
	},
}

// feature unlink command
var featureUnlinkCmd = &cobra.Command{
	Use:   "unlink <from-feature>",
	Short: "Remove a relationship between features",
	Long: `Remove a relationship between two features.

Example:
  taskwing feature unlink "Users" --from "Auth"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromName := args[0]
		toName, _ := cmd.Flags().GetString("from")
		relType, _ := cmd.Flags().GetString("type")

		if toName == "" {
			return fmt.Errorf("--from is required")
		}
		if relType == "" {
			relType = memory.EdgeTypeDependsOn
		}

		store, err := memory.NewSQLiteStore(GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory store: %w", err)
		}
		defer store.Close()

		// Resolve feature names to IDs
		features, _ := store.ListFeatures()
		var fromID, toID string
		for _, f := range features {
			if strings.EqualFold(f.Name, fromName) || f.ID == fromName {
				fromID = f.ID
			}
			if strings.EqualFold(f.Name, toName) || f.ID == toName {
				toID = f.ID
			}
		}

		if fromID == "" {
			return fmt.Errorf("feature not found: %s", fromName)
		}
		if toID == "" {
			return fmt.Errorf("feature not found: %s", toName)
		}

		if err := store.Unlink(fromID, toID, relType); err != nil {
			return fmt.Errorf("unlink features: %w", err)
		}

		fmt.Printf("✓ Unlinked: %s → %s\n", fromName, toName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(featureCmd)

	// Add subcommands
	featureCmd.AddCommand(featureAddCmd)
	featureCmd.AddCommand(featureListCmd)
	featureCmd.AddCommand(featureShowCmd)
	featureCmd.AddCommand(featureUpdateCmd)
	featureCmd.AddCommand(featureDeleteCmd)
	featureCmd.AddCommand(featureLinkCmd)
	featureCmd.AddCommand(featureUnlinkCmd)

	// Flags for add
	featureAddCmd.Flags().String("oneliner", "", "Brief description (required)")
	featureAddCmd.Flags().String("tags", "", "Comma-separated tags")
	featureAddCmd.Flags().String("status", "active", "Status: active, deprecated, planned")

	// Flags for update
	featureUpdateCmd.Flags().String("oneliner", "", "Brief description")
	featureUpdateCmd.Flags().String("tags", "", "Comma-separated tags (pass empty string to clear)")
	featureUpdateCmd.Flags().String("status", "", "Status: active, deprecated, planned")

	// Flags for delete
	featureDeleteCmd.Flags().Bool("force", false, "Confirm deletion")

	// Flags for link
	featureLinkCmd.Flags().String("to", "", "Target feature (required)")
	featureLinkCmd.Flags().String("type", "depends_on", "Relationship type: depends_on, extends, replaces, related")
	featureLinkCmd.Flags().String("depends-on", "", "Create a depends_on relationship to target feature")
	featureLinkCmd.Flags().String("extends", "", "Create an extends relationship to target feature")
	featureLinkCmd.Flags().String("replaces", "", "Create a replaces relationship to target feature")
	featureLinkCmd.Flags().String("related", "", "Create a related relationship to target feature")

	// Flags for unlink
	featureUnlinkCmd.Flags().String("from", "", "Target feature to unlink from (required)")
	featureUnlinkCmd.Flags().String("type", "depends_on", "Relationship type")
}
