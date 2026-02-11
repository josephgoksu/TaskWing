/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/ui"
)

// Ordered list for consistent display
var aiConfigOrder = bootstrap.ValidAINames()

// promptAISelection shows the AI selection UI.
// preSelected: optional list of AI IDs to pre-select (e.g., from detected global config)
func promptAISelection(preSelected ...string) []string {
	descriptions := bootstrap.AIDisplayNames()

	selected, err := ui.PromptAISelection(aiConfigOrder, descriptions, preSelected...)
	if err != nil {
		fmt.Printf("Error running selection: %v\n", err)
		return nil
	}
	return selected
}
