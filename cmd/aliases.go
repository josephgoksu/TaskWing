/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// plansCmd is an alias for "plan list"
var plansCmd = &cobra.Command{
	Use:   "plans",
	Short: "List all plans (alias for 'plan list')",
	Long:  `List all development plans. This is a convenience alias for 'tw plan list'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return planListCmd.RunE(planListCmd, args)
	},
}

// tasksCmd is an alias for "task list"
var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "List all tasks (alias for 'task list')",
	Long:  `List all tasks grouped by plan. This is a convenience alias for 'tw task list'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return taskListCmd.RunE(taskListCmd, args)
	},
}

// nodesCmd is an alias for "list" (knowledge nodes)
var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List knowledge nodes (alias for 'list')",
	Long:  `List all knowledge nodes. This is a convenience alias for 'tw list'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return listCmd.RunE(listCmd, args)
	},
}

// statusCmd shows a quick project status summary
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show project status summary",
	Long:  `Display a quick summary of the project's TaskWing status including node counts, plan counts, and any issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Delegate to memory check for now
		return memoryCheckCmd.RunE(memoryCheckCmd, args)
	},
}

func init() {
	rootCmd.AddCommand(plansCmd)
	rootCmd.AddCommand(tasksCmd)
	rootCmd.AddCommand(nodesCmd)
	rootCmd.AddCommand(statusCmd)

	// Copy flags from parent commands
	tasksCmd.Flags().StringP("plan", "p", "", "Filter by plan ID (prefix match)")
}
