/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/spf13/cobra"
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search tasks by title, description, or ID",
	Long: `Search for tasks using a query string that will be matched against task titles, descriptions, and IDs.
The search is case-insensitive and supports partial matches.

Examples:
  taskwing search "authentication"     # Find tasks containing "authentication"
  taskwing search "urgent API"        # Find tasks containing both "urgent" and "API"
  taskwing search --json "database"   # Output results in JSON format
  taskwing search --status todo "fix" # Search only in todo tasks`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := GetStore()
		if err != nil {
			HandleFatalError("Error: Could not initialize the task store.", err)
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleFatalError("Failed to close task store", err)
			}
		}()

		query := strings.Join(args, " ")
		statusFilter, _ := cmd.Flags().GetString("status")
		priorityFilter, _ := cmd.Flags().GetString("priority")
		jsonOutput, _ := cmd.Flags().GetBool("json")

		// Create search filter
		filterFn := func(task models.Task) bool {
			// Status filter
			if statusFilter != "" && string(task.Status) != statusFilter {
				return false
			}

			// Priority filter
			if priorityFilter != "" && string(task.Priority) != priorityFilter {
				return false
			}

			// Search query
			queryLower := strings.ToLower(query)
			titleLower := strings.ToLower(task.Title)
			descLower := strings.ToLower(task.Description)
			idLower := strings.ToLower(task.ID)

			return strings.Contains(titleLower, queryLower) ||
				strings.Contains(descLower, queryLower) ||
				strings.Contains(idLower, queryLower)
		}

		// Search tasks
		tasks, err := taskStore.ListTasks(filterFn, nil)
		if err != nil {
			HandleFatalError("Failed to search tasks", err)
		}

		if len(tasks) == 0 {
			if jsonOutput {
				fmt.Println("[]")
			} else {
				fmt.Printf("No tasks found matching query: %s\n", query)
			}
			return
		}

		if jsonOutput {
			// Output as JSON
			jsonData, err := json.MarshalIndent(tasks, "", "  ")
			if err != nil {
				HandleFatalError("Failed to marshal search results to JSON", err)
				return
			}
			fmt.Println(string(jsonData))
		} else {
			// Output as formatted text
			fmt.Printf("Found %d task(s) matching query: %s\n\n", len(tasks), query)
			for i, task := range tasks {
				fmt.Printf("%d. %s (ID: %s)\n", i+1, task.Title, truncateUUID(task.ID))
				fmt.Printf("   Status: %s | Priority: %s\n", task.Status, task.Priority)
				if task.Description != "" {
					desc := task.Description
					if len(desc) > 100 {
						desc = desc[:97] + "..."
					}
					fmt.Printf("   Description: %s\n", desc)
				}
				if len(task.Dependencies) > 0 {
					fmt.Printf("   Dependencies: %s\n", truncateUUIDList(task.Dependencies))
				}
				fmt.Println()
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)

	// Optional filtering flags
	searchCmd.Flags().String("status", "", "Filter search results by status")
	searchCmd.Flags().String("priority", "", "Filter search results by priority")
	searchCmd.Flags().Bool("json", false, "Output search results in JSON format")
}
