/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json" // For pretty printing task output for now
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/google/uuid"                  // For generating final IDs
	"github.com/josephgoksu/taskwing.app/llm" // Import the new llm package
	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/prompts"
	"github.com/josephgoksu/taskwing.app/store" // For TaskStore interface
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	// For direct ENV var check as fallback
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate TaskWing artifacts.",
	Long:  `The generate command has subcommands to generate various TaskWing artifacts, such as tasks from a document.`,
	// Run: func(cmd *cobra.Command, args []string) { ... }, // Base command does nothing itself
}

var generateTasksCmd = &cobra.Command{
	Use:   "tasks --file <path_to_document>",
	Short: "Generate tasks from a document (e.g., PRD).",
	Long: `Parses a document (e.g., a Product Requirements Document) using an AI model
and generates a list of tasks and subtasks based on its content.

The supported document formats are plain text (.txt) and Markdown (.md).
The system will prompt for confirmation before creating any tasks.

Requires LLM to be configured in .taskwing/.taskwing.yaml or via environment variables.
Example configuration in .taskwing/.taskwing.yaml:
llm:
  provider: "openai" # or "google"
  modelName: "gpt-4o-mini" # Example: "gpt-4o-mini", "gpt-4o"
  # apiKey: "YOUR_OPENAI_API_KEY" # Set via TASKWING_LLM_APIKEY or OPENAI_API_KEY
  # projectId: "your-gcp-project-id" # For Google, if provider is "google"
  # maxOutputTokens: 2048
  # temperature: 0.7
`,
	Args: cobra.NoArgs, // Path will be a flag
	Run: func(cmd *cobra.Command, args []string) {
		// Graceful shutdown context listening for SIGINT (Ctrl+C)
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()

		docPath, _ := cmd.Flags().GetString("file")

		if docPath == "" {
			HandleError("Error: The --file flag is required with the path to your document.", nil)
		}

		// --- PRE-GENERATION CHECKS ---
		// 1. Check for existing tasks and ask for overwrite confirmation BEFORE any expensive operations.
		taskStore, err := GetStore()
		if err != nil {
			HandleError("Error: Could not initialize the task store.", err)
		}

		existingTasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			HandleError("Error: Could not check for existing tasks.", err)
		}

		if len(existingTasks) > 0 {
			numExisting := len(existingTasks)
			fmt.Printf("Found %d existing task(s).\n", numExisting)
			overwritePrompt := promptui.Prompt{
				Label:     prompts.GenerateTasksOverwriteConfirmation,
				IsConfirm: true,
			}
			_, err := overwritePrompt.Run()
			if err != nil {
				if err == promptui.ErrAbort {
					fmt.Println("Task generation cancelled.")
					taskStore.Close()
					return
				}
				taskStore.Close()
				HandleError("Error: Could not get confirmation for overwriting tasks.", err)
			}

			// User confirmed overwrite. Delete existing tasks now.
			fmt.Println("\nDeleting existing tasks...")
			if err := taskStore.DeleteAllTasks(); err != nil {
				taskStore.Close()
				HandleError("Error: Could not delete the existing tasks.", err)
			}
			fmt.Printf("Successfully deleted %d task(s).\n\n", numExisting)
		}

		// We are done with pre-checks, we can close the store connection for now.
		// It will be re-opened later for creation. This avoids holding the lock.
		taskStore.Close()

		// --- LLM TASK GENERATION ---
		appCfg := GetConfig()

		// 2. Read PRD file content.
		prdContentBytes, err := os.ReadFile(docPath)
		if err != nil {
			HandleError(fmt.Sprintf("Error: Could not read document file '%s'.", docPath), err)
		}
		prdContent := string(prdContentBytes)

		// 3. Load LLM configuration from Viper.
		cmdLLMCfg := appCfg.LLM // This is cmd.LLMConfig

		// Prepare llm.LLMConfig from cmd.LLMConfig, resolving API keys from ENV if necessary.
		resolvedLLMConfig := llm.LLMConfig{
			Provider:                   cmdLLMCfg.Provider,
			ModelName:                  cmdLLMCfg.ModelName,
			APIKey:                     cmdLLMCfg.APIKey,    // Viper already handles ENV overlay for this field from cmd.LLMConfig
			ProjectID:                  cmdLLMCfg.ProjectID, // Viper already handles ENV overlay
			MaxOutputTokens:            cmdLLMCfg.MaxOutputTokens,
			Temperature:                cmdLLMCfg.Temperature,
			EstimationTemperature:      cmdLLMCfg.EstimationTemperature,
			EstimationMaxOutputTokens:  cmdLLMCfg.EstimationMaxOutputTokens,
			ImprovementTemperature:     cmdLLMCfg.ImprovementTemperature,     // Added
			ImprovementMaxOutputTokens: cmdLLMCfg.ImprovementMaxOutputTokens, // Added
		}

		// Explicitly check/resolve APIKey from specific ENV vars if still empty after Viper's load
		if resolvedLLMConfig.APIKey == "" {
			switch resolvedLLMConfig.Provider {
			case "openai":
				apiKeyEnv := os.Getenv("OPENAI_API_KEY")
				if apiKeyEnv == "" {
					apiKeyEnv = os.Getenv(envPrefix + "_LLM_APIKEY")
				}
				resolvedLLMConfig.APIKey = apiKeyEnv
			case "google":
				apiKeyEnv := os.Getenv("GOOGLE_API_KEY")
				if apiKeyEnv == "" {
					apiKeyEnv = os.Getenv(envPrefix + "_LLM_APIKEY")
				}
				resolvedLLMConfig.APIKey = apiKeyEnv
			}
		}
		// Resolve ProjectID for Google if still empty
		if resolvedLLMConfig.Provider == "google" && resolvedLLMConfig.ProjectID == "" {
			resolvedLLMConfig.ProjectID = os.Getenv(envPrefix + "_LLM_PROJECTID")
		}

		// Validate essential LLM config after attempting ENV var fallbacks
		if resolvedLLMConfig.Provider == "" {
			HandleError("Error: LLM provider is not configured. Set 'llm.provider' in your config or use the TASKWING_LLM_PROVIDER environment variable.", nil)
		}
		if resolvedLLMConfig.ModelName == "" {
			HandleError("Error: LLM model name is not configured. Set 'llm.modelName' in your config or use the TASKWING_LLM_MODELNAME environment variable.", nil)
		}
		if resolvedLLMConfig.Provider == "openai" && resolvedLLMConfig.APIKey == "" {
			HandleError("Error: OpenAI API key is not configured. Set 'llm.apiKey' in your config or use the TASKWING_LLM_APIKEY or OPENAI_API_KEY environment variables.", nil)
		}
		if resolvedLLMConfig.Provider == "google" && resolvedLLMConfig.ProjectID == "" {
			HandleError("Error: Google Cloud ProjectID is not configured. Set 'llm.projectId' in your config or use the TASKWING_LLM_PROJECTID environment variable.", nil)
		}

		// 4. Instantiate LLM Provider.
		provider, err := llm.NewProvider(&resolvedLLMConfig)
		if err != nil {
			HandleError("Error: Could not create the LLM provider.", err)
		}

		// --- OPTIONAL PRD IMPROVEMENT ---
		// Construct the absolute path to the templates directory for the prompt loader.
		absoluteTemplatesDir := filepath.Join(appCfg.Project.RootDir, appCfg.Project.TemplatesDir)

		improvePrompt := promptui.Prompt{
			Label:     prompts.GenerateTasksImprovementConfirmation,
			IsConfirm: true,
			Default:   "y",
		}
		_, err = improvePrompt.Run()
		if err != nil && err != promptui.ErrAbort {
			HandleError("Error: Could not get confirmation for PRD improvement.", err)
			return // Unreachable
		}

		if err == nil { // User confirmed "yes"
			s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
			s.Suffix = " Improving PRD with LLM... (This may take a moment)"
			s.Start()

			improveSystemPrompt, promptErr := prompts.GetPrompt(prompts.KeyImprovePRD, absoluteTemplatesDir)
			if promptErr != nil {
				s.Stop()
				HandleError("Error loading PRD improvement prompt.", promptErr)
			}
			improvedContent, improveErr := provider.ImprovePRD(
				ctx, // Pass the cancellable context
				improveSystemPrompt,
				prdContent,
				resolvedLLMConfig.ModelName,
				resolvedLLMConfig.APIKey,
				resolvedLLMConfig.ProjectID,
				resolvedLLMConfig.ImprovementMaxOutputTokens,
				resolvedLLMConfig.ImprovementTemperature,
			)
			s.Stop()
			fmt.Println() // Newline after spinner stops

			if improveErr != nil {
				if errors.Is(improveErr, context.Canceled) {
					fmt.Println("\nOperation cancelled by user.")
					os.Exit(130) // Standard exit code for Ctrl+C
				}
				fmt.Fprintf(os.Stderr, "Warning: Failed to improve PRD: %v. Proceeding with original content.\n", improveErr)
			} else {
				prdContent = improvedContent // Use the improved content for subsequent steps

				// Save the improved PRD for auditing
				generatedPRDDir := filepath.Join(appCfg.Project.RootDir, "generated_prds")
				if err := os.MkdirAll(generatedPRDDir, 0755); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Could not create directory for improved PRD at '%s': %v\n", generatedPRDDir, err)
				} else {
					baseName := strings.TrimSuffix(filepath.Base(docPath), filepath.Ext(docPath))
					improvedPRDPath := filepath.Join(generatedPRDDir, fmt.Sprintf("%s-improved-%s.md", baseName, time.Now().Format("20060102-150405")))
					if err := os.WriteFile(improvedPRDPath, []byte(prdContent), 0644); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: Failed to save improved PRD to '%s': %v\n", improvedPRDPath, err)
					} else {
						fmt.Printf("Improved PRD saved for review at: %s\n", improvedPRDPath)
					}
				}
			}
		} else {
			fmt.Println("Skipping PRD improvement. Proceeding with original content.")
		}

		// Attempt to estimate task parameters to dynamically set maxOutputTokens
		s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
		s.Suffix = fmt.Sprintf(" Estimating task parameters from document using %s provider and model %s...", resolvedLLMConfig.Provider, resolvedLLMConfig.ModelName)
		s.Start()

		estimateSystemPrompt, promptErr := prompts.GetPrompt(prompts.KeyEstimateTasks, absoluteTemplatesDir)
		if promptErr != nil {
			s.Stop()
			HandleError("Error loading task estimation prompt.", promptErr)
		}
		estimationOutput, estimationErr := provider.EstimateTaskParameters(
			ctx, // Pass the cancellable context
			estimateSystemPrompt,
			prdContent,
			resolvedLLMConfig.ModelName,
			resolvedLLMConfig.APIKey,
			resolvedLLMConfig.ProjectID,
			resolvedLLMConfig.EstimationMaxOutputTokens, // Use configured estimation tokens
			resolvedLLMConfig.EstimationTemperature,     // Use configured estimation temperature
		)
		s.Stop()
		fmt.Println() // Newline after spinner stops

		currentMaxOutputTokens := resolvedLLMConfig.MaxOutputTokens // Fallback to configured value
		const minDynamicTokens = 4096                               // Absolute minimum if we calculate dynamically below this.
		const maxSensibleDynamicTokens = 32768                      // Cap for dynamically calculated tokens

		if estimationErr != nil {
			if errors.Is(estimationErr, context.Canceled) {
				fmt.Println("\nOperation cancelled by user.")
				os.Exit(130)
			}
			fmt.Fprintf(os.Stderr, "Warning: Failed to estimate task parameters, will use configured maxOutputTokens (%d). Error: %v\n", currentMaxOutputTokens, estimationErr)
		} else {
			fmt.Printf("LLM Estimation - Estimated Task Count: %d, Complexity: %s\n", estimationOutput.EstimatedTaskCount, estimationOutput.EstimatedComplexity)
			if estimationOutput.EstimatedTaskCount > 0 {
				calculatedTokens := (estimationOutput.EstimatedTaskCount * 200) + 2048 // Heuristic: 200 tokens/task + 2048 buffer

				// Ensure dynamic tokens are not excessively low or high
				if calculatedTokens < minDynamicTokens {
					dynamicMaxOutputTokens := minDynamicTokens
					fmt.Printf("Calculated dynamic tokens (%d) is below minimum (%d), using minimum.\n", calculatedTokens, minDynamicTokens)
					currentMaxOutputTokens = dynamicMaxOutputTokens
				} else if calculatedTokens > maxSensibleDynamicTokens {
					dynamicMaxOutputTokens := maxSensibleDynamicTokens
					fmt.Printf("Calculated dynamic tokens (%d) exceeds sensible cap (%d), using cap.\n", calculatedTokens, maxSensibleDynamicTokens)
					currentMaxOutputTokens = dynamicMaxOutputTokens
				} else {
					currentMaxOutputTokens = calculatedTokens
				}
				fmt.Printf("Using dynamically determined maxOutputTokens: %d\n", currentMaxOutputTokens)
			} else {
				fmt.Printf("LLM estimated 0 tasks. Using configured maxOutputTokens: %d\n", currentMaxOutputTokens)
			}
		}

		genSpinner := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
		genSpinner.Suffix = fmt.Sprintf(" Generating tasks from '%s' (max output tokens: %d) using %s provider and model %s...", docPath, currentMaxOutputTokens, resolvedLLMConfig.Provider, resolvedLLMConfig.ModelName)
		genSpinner.Start()

		// 5. Call LLM service to generate tasks with the determined maxOutputTokens.
		generateSystemPrompt, promptErr := prompts.GetPrompt(prompts.KeyGenerateTasks, absoluteTemplatesDir)
		if promptErr != nil {
			genSpinner.Stop()
			HandleError("Error loading task generation prompt.", promptErr)
		}
		llmTaskOutputs, err := provider.GenerateTasks(
			ctx, // Pass the cancellable context
			generateSystemPrompt,
			prdContent,
			resolvedLLMConfig.ModelName,
			resolvedLLMConfig.APIKey,
			resolvedLLMConfig.ProjectID,
			currentMaxOutputTokens,
			resolvedLLMConfig.Temperature,
		)
		genSpinner.Stop()
		fmt.Println() // Newline after spinner stops

		if err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Println("\nOperation cancelled by user.")
				os.Exit(130)
			}
			HandleError("Error: The AI model failed to generate tasks.", err)
		}

		if len(llmTaskOutputs) == 0 {
			fmt.Println("LLM did not return any tasks based on the document.")
			return
		}

		fmt.Printf("LLM returned %d potential top-level task(s).\n", len(llmTaskOutputs))

		// 6. Parse LLM JSON response (already parsed into llmTaskOutputs by the provider method).

		// 7. Resolve parent/child and dependency relationships by title.
		fmt.Println("\n--- Raw LLM Task Outputs (for debugging) ---")
		rawOutputBytes, _ := json.MarshalIndent(llmTaskOutputs, "", "  ")
		fmt.Println(string(rawOutputBytes))
		fmt.Println("--- End Raw LLM Task Outputs ---")

		taskCandidates, relationshipMap, err := resolveAndBuildTaskCandidates(llmTaskOutputs)
		if err != nil {
			HandleError("Error: Could not process the AI model's output.", err)
		}

		if len(taskCandidates) == 0 {
			fmt.Println("No valid task candidates could be formed from the LLM output.")
			return
		}

		// --- POST-GENERATION CONFIRMATION ---
		// 8. Display tasks and ask for final confirmation to create them.
		fmt.Printf("\n--- Proposed Tasks to Create (%d) ---\n", len(taskCandidates))
		displayTaskCandidates(taskCandidates, relationshipMap)

		confirmPrompt := promptui.Prompt{
			Label:     fmt.Sprintf("Do you want to create these %d tasks?", len(taskCandidates)),
			IsConfirm: true,
		}
		_, confirmErr := confirmPrompt.Run()
		if confirmErr != nil {
			if confirmErr == promptui.ErrAbort {
				fmt.Println("Task creation cancelled.")
				return
			}
			HandleError("Error: Could not get confirmation to create tasks.", confirmErr)
		}

		// 9. If confirmed, get a fresh store connection and create tasks.
		fmt.Println("\nCreating tasks...")
		finalTaskStore, finalStoreErr := GetStore()
		if finalStoreErr != nil {
			HandleError("Error: Could not initialize task store for the final step.", finalStoreErr)
		}
		defer finalTaskStore.Close()

		createdCount, creationErrors := createTasksInStore(finalTaskStore, taskCandidates, relationshipMap)
		fmt.Printf("Successfully created %d tasks.\n", createdCount)
		if len(creationErrors) > 0 {
			fmt.Fprintf(os.Stderr, "Encountered %d errors during task creation:\n", len(creationErrors))
			for i, e := range creationErrors {
				fmt.Fprintf(os.Stderr, "  %d: %v\n", i+1, e)
			}
		}
	},
}

// tempIDs for relationship mapping during candidate resolution
const tempIDPrefix = "temp_task_id_"

type taskRelationshipMap struct {
	tempParentToChildren map[string][]string    // tempParentID -> []tempChildID
	tempChildToParent    map[string]string      // tempChildID -> tempParentID
	tempTaskToDeps       map[string][]string    // tempTaskID -> []tempDependencyID (where dependency is also a tempID)
	flattenedTasks       map[string]models.Task // tempID -> models.Task (without final ID, ParentID, Dependencies)
	titleToTempID        map[string]string      // title -> tempID (for resolving deps by title)
	tempIDToInputID      map[int]string         // input tempId (int) -> internal tempID (string)
	taskOrder            []string               // tempIDs in a stable order for processing and display
}

// resolveAndBuildTaskCandidates processes LLM outputs into a flat list of models.Task candidates
// and a map representing their relationships using temporary IDs.
func resolveAndBuildTaskCandidates(llmOutputs []llm.TaskOutput) ([]models.Task, taskRelationshipMap, error) {
	relMap := taskRelationshipMap{
		tempParentToChildren: make(map[string][]string),
		tempChildToParent:    make(map[string]string),
		tempTaskToDeps:       make(map[string][]string),
		flattenedTasks:       make(map[string]models.Task),
		tempIDToInputID:      make(map[int]string),
		taskOrder:            make([]string, 0),
	}
	tempIDCounter := 0

	// Recursive function to flatten tasks and initial relationships
	var flatten func(outputs []llm.TaskOutput, parentTempID string) error
	flatten = func(outputs []llm.TaskOutput, parentTempID string) error {
		for _, llmTask := range outputs {
			tempIDCounter++
			currentTempID := fmt.Sprintf("%s%d", tempIDPrefix, tempIDCounter)

			if llmTask.Title == "" {
				fmt.Fprintf(os.Stderr, "Warning: LLM returned a task with an empty title. Skipping this task.\n")
				continue
			}
			if llmTask.TempID == 0 {
				return fmt.Errorf("LLM returned a task with a missing or zero tempId: '%s'. Aborting.", llmTask.Title)
			}

			if _, idExists := relMap.tempIDToInputID[llmTask.TempID]; idExists {
				return fmt.Errorf("duplicate tempId %d found from LLM output. Each task must have a unique tempId.", llmTask.TempID)
			}
			relMap.tempIDToInputID[llmTask.TempID] = currentTempID
			relMap.taskOrder = append(relMap.taskOrder, currentTempID)

			candidateTask := models.Task{
				Title:              llmTask.Title,
				Description:        llmTask.Description,
				AcceptanceCriteria: llmTask.AcceptanceCriteria,
				Priority:           mapLLMPriority(llmTask.Priority),
				Status:             models.StatusPending,
			}
			relMap.flattenedTasks[currentTempID] = candidateTask

			if parentTempID != "" {
				relMap.tempParentToChildren[parentTempID] = append(relMap.tempParentToChildren[parentTempID], currentTempID)
				relMap.tempChildToParent[currentTempID] = parentTempID
			}

			// Store dependencies by their given integer tempIDs for later resolution
			if len(llmTask.DependsOnIDs) > 0 {
				depStrs := make([]string, len(llmTask.DependsOnIDs))
				for i, id := range llmTask.DependsOnIDs {
					depStrs[i] = fmt.Sprintf("%d", id)
				}
				relMap.tempTaskToDeps[currentTempID] = depStrs
			}

			if len(llmTask.Subtasks) > 0 {
				if err := flatten(llmTask.Subtasks, currentTempID); err != nil {
					return err // Propagate error up if any
				}
			}
		}
		return nil
	}

	if err := flatten(llmOutputs, ""); err != nil {
		return nil, relMap, err
	}

	// Second pass: Resolve DependsOnIDs from integer tempIDs to internal string tempIDs
	for taskTempID, depIntIDs := range relMap.tempTaskToDeps {
		var depTempIDs []string
		for _, depIntIDStr := range depIntIDs {
			var depIntID int
			fmt.Sscanf(depIntIDStr, "%d", &depIntID) // Convert string back to int for lookup

			if depTargetTempID, exists := relMap.tempIDToInputID[depIntID]; exists {
				if depTargetTempID == taskTempID {
					// This check is against the internal string tempID, which is correct
					return nil, relMap, fmt.Errorf("task '%s' (tempId %d) cannot depend on itself", relMap.flattenedTasks[taskTempID].Title, depIntID)
				}
				depTempIDs = append(depTempIDs, depTargetTempID)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Dependency tempId '%d' for task '%s' not found. Skipping dependency.\n", depIntID, relMap.flattenedTasks[taskTempID].Title)
			}
		}
		relMap.tempTaskToDeps[taskTempID] = depTempIDs
	}

	finalCandidates := make([]models.Task, 0, len(relMap.taskOrder))
	for _, tempID := range relMap.taskOrder {
		finalCandidates = append(finalCandidates, relMap.flattenedTasks[tempID])
	}

	return finalCandidates, relMap, nil
}

func mapLLMPriority(llmPriority string) models.TaskPriority {
	switch strings.ToLower(strings.TrimSpace(llmPriority)) {
	case "urgent":
		return models.PriorityUrgent
	case "high":
		return models.PriorityHigh
	case "medium", "med", "": // Treat empty as medium
		return models.PriorityMedium
	case "low":
		return models.PriorityLow
	default:
		fmt.Fprintf(os.Stderr, "Warning: Unknown LLM priority '%s', defaulting to Medium.\n", llmPriority)
		return models.PriorityMedium
	}
}

func displayTaskCandidates(tasks []models.Task, relMap taskRelationshipMap) {
	fmt.Println("----------------------------------------------------------------------")

	tempIDToDisplayIndex := make(map[string]int)
	for i, tempID := range relMap.taskOrder {
		tempIDToDisplayIndex[tempID] = i + 1
	}

	for i, currentTempID := range relMap.taskOrder {
		task := relMap.flattenedTasks[currentTempID]
		fmt.Printf("[%d] Title: %s (Priority: %s)\n", i+1, task.Title, task.Priority)
		if task.Description != "" && task.Description != task.Title {
			fmt.Printf("    Description: %s\n", task.Description)
		}
		if task.AcceptanceCriteria != "" {
			// Format acceptance criteria for better readability
			formattedAC := strings.ReplaceAll(task.AcceptanceCriteria, "\n", "\n                 ")
			fmt.Printf("    Acceptance Criteria: %s\n", formattedAC)
		}

		if parentTempID, isChild := relMap.tempChildToParent[currentTempID]; isChild {
			if parentTask, ok := relMap.flattenedTasks[parentTempID]; ok {
				fmt.Printf("    Parent Task: %s (Ref: #%d)\n", parentTask.Title, tempIDToDisplayIndex[parentTempID])
			}
		}
		if subtaskTempIDs, hasSubtasks := relMap.tempParentToChildren[currentTempID]; hasSubtasks {
			var subtaskRefs []string
			for _, subTempID := range subtaskTempIDs {
				if subTask, ok := relMap.flattenedTasks[subTempID]; ok {
					subtaskRefs = append(subtaskRefs, fmt.Sprintf("%s (#%d)", subTask.Title, tempIDToDisplayIndex[subTempID]))
				}
			}
			if len(subtaskRefs) > 0 {
				fmt.Printf("    Subtasks: %s\n", strings.Join(subtaskRefs, "; "))
			}
		}
		if depTempIDs, hasDeps := relMap.tempTaskToDeps[currentTempID]; hasDeps {
			var depRefs []string
			for _, depTempID := range depTempIDs {
				if depTask, ok := relMap.flattenedTasks[depTempID]; ok {
					depRefs = append(depRefs, fmt.Sprintf("%s (#%d)", depTask.Title, tempIDToDisplayIndex[depTempID]))
				}
			}
			if len(depRefs) > 0 {
				fmt.Printf("    Depends On: %s\n", strings.Join(depRefs, "; "))
			}
		}
		fmt.Println("----------------------------------------------------------------------")
	}
}

func createTasksInStore(store store.TaskStore, taskCandidates []models.Task, relMap taskRelationshipMap) (createdCount int, errors []error) {
	tempIDToFinalID := make(map[string]string)

	// Pass 1: Assign final UUIDs to all tasks based on their tempIDs from relMap.taskOrder
	tasksWithFinalIDs := make(map[string]models.Task) // tempID -> task with final UUID
	for _, tempID := range relMap.taskOrder {
		candidate := relMap.flattenedTasks[tempID]
		finalID := uuid.New().String()
		tempIDToFinalID[tempID] = finalID

		taskWithFinalID := candidate
		taskWithFinalID.ID = finalID
		tasksWithFinalIDs[tempID] = taskWithFinalID
	}

	// Pass 2: Link ParentID and Dependencies using the final UUIDs
	tasksToCreate := make([]models.Task, 0, len(relMap.taskOrder))
	for _, tempID := range relMap.taskOrder {
		linkedTask := tasksWithFinalIDs[tempID]

		if parentTempID, isChild := relMap.tempChildToParent[tempID]; isChild {
			if finalParentID, ok := tempIDToFinalID[parentTempID]; ok {
				linkedTask.ParentID = &finalParentID
			} else {
				errors = append(errors, fmt.Errorf("internal error: could not find final ID for parent tempID %s of task %s", parentTempID, linkedTask.Title))
			}
		}

		if depTempIDs, hasDeps := relMap.tempTaskToDeps[tempID]; hasDeps {
			var finalDepIDs []string
			for _, depTempID := range depTempIDs {
				if finalDepID, ok := tempIDToFinalID[depTempID]; ok {
					finalDepIDs = append(finalDepIDs, finalDepID)
				} else {
					errors = append(errors, fmt.Errorf("internal error: could not find final ID for dependency tempID %s of task %s", depTempID, linkedTask.Title))
				}
			}
			linkedTask.Dependencies = finalDepIDs
		}
		tasksToCreate = append(tasksToCreate, linkedTask)
	}

	if len(errors) > 0 { // If linking failed, don't proceed to creation
		return 0, errors
	}

	// Pass 3: Create tasks in the store
	// The store's CreateTask method should handle setting SubtaskIDs on parents and Dependents on dependencies.
	for _, taskToSave := range tasksToCreate {
		createdTask, err := store.CreateTask(taskToSave)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to create task '%s' (ID: %s): %w", taskToSave.Title, taskToSave.ID, err))
		} else {
			createdCount++
			fmt.Printf("  Created task: %s (ID: %s)\n", createdTask.Title, createdTask.ID)
		}
	}
	return createdCount, errors
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.AddCommand(generateTasksCmd)

	generateTasksCmd.Flags().StringP("file", "f", "", "Path to the document file (PRD) to generate tasks from.")
	// MarkAsRequired is not strictly necessary if we check it and print help as above
	// generateTasksCmd.MarkFlagRequired("file")
}
