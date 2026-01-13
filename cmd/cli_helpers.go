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
