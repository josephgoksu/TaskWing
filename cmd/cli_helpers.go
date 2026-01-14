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

func isPreview() bool {
	return viper.GetBool("preview")
}

// truncateForLog truncates a string to maxLen characters for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func isQuiet() bool {
	return viper.GetBool("quiet")
}

func isVerbose() bool {
	return viper.GetBool("verbose")
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

func resolveNodeID(repo *memory.Repository, id string) (string, *memory.Node, error) {
	if id == "" {
		return "", nil, fmt.Errorf("node id cannot be empty")
	}

	node, err := repo.GetNode(id)
	if err == nil && node != nil {
		return id, node, nil
	}

	nodes, listErr := repo.ListNodes("")
	if listErr != nil {
		return "", nil, fmt.Errorf("node not found: %s", id)
	}

	var matches []memory.Node
	for _, n := range nodes {
		if strings.HasPrefix(n.ID, id) {
			matches = append(matches, n)
		}
	}

	if len(matches) == 0 {
		return "", nil, fmt.Errorf("node not found: %s", id)
	}
	if len(matches) > 1 {
		var ids []string
		for i, n := range matches {
			if i >= 5 {
				ids = append(ids, "...")
				break
			}
			ids = append(ids, n.ID)
		}
		return "", nil, fmt.Errorf("ambiguous node id %q (matches: %s)", id, strings.Join(ids, ", "))
	}

	return matches[0].ID, &matches[0], nil
}
