package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/viper"
)

func isJSON() bool {
	return viper.GetBool("json")
}

func isQuiet() bool {
	return viper.GetBool("quiet")
}

func printJSON(v any) error {
	output, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(output))
	return nil
}

func openRepo() (*memory.Repository, error) {
	memoryPath, err := config.GetMemoryBasePath()
	if err != nil {
		return nil, fmt.Errorf("get memory path: %w", err)
	}
	return memory.NewDefaultRepository(memoryPath)
}

func openRepoOrHandleMissingMemory() (*memory.Repository, error) {
	repo, err := openRepo()
	if err == nil {
		return repo, nil
	}

	handled, handleErr := handleMissingProjectMemoryError(err)
	if handled {
		return nil, handleErr
	}
	if handleErr != nil {
		return nil, handleErr
	}

	return nil, fmt.Errorf("open memory repo: %w", err)
}

func handleMissingProjectMemoryError(err error) (bool, error) {
	if err == nil {
		return false, nil
	}

	if !isMissingProjectMemoryError(err) {
		return false, nil
	}

	if isJSON() {
		return true, printJSON(map[string]any{
			"ok":      false,
			"error":   "project memory not initialized",
			"next":    "run taskwing bootstrap",
			"command": "taskwing bootstrap",
		})
	}

	fmt.Println("No project memory found for this repository.")
	fmt.Println("Run 'taskwing bootstrap' first.")
	return true, nil
}

func isMissingProjectMemoryError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no .taskwing directory found at project root")
}

func confirmOrAbort(prompt string) bool {
	if isJSON() {
		return true
	}
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Cancelled.")
		return false
	}
	return true
}
