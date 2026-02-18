/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/cobra"
)

// Training data format matching finetune/karluk/ pipeline
type trainingMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type trainingExample struct {
	Messages []trainingMessage      `json:"messages"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Classification system prompt (matches generate_classification_data.py)
const classificationSystemPrompt = `You are a software architecture analyst. Given a piece of content from a codebase (code snippet, commit message, documentation paragraph, or config file), classify it into exactly one architecture knowledge type and extract structured metadata.

Classification types:
- decision: An architectural choice, technology selection, or design tradeoff
- feature: A product capability or user-facing functionality
- pattern: A recurring design approach, coding convention, or workflow
- constraint: A mandatory rule, limitation, or invariant

You MUST respond with valid JSON only. No markdown, no explanation, just the JSON object.`

var memoryExportTrainingCmd = &cobra.Command{
	Use:   "export-training",
	Short: "Export structured nodes as training data for Karluk",
	Long: `Export bootstrap findings with structured content as chat-format JSONL
compatible with the finetune/karluk/ pipeline.

Only nodes with structured content (from bootstrap) are included.
Plain-text nodes (from remember/MCP) are skipped.

Examples:
  taskwing memory export-training --output training.jsonl
  taskwing memory export-training --type decision --min-confidence 0.8
  taskwing memory export-training | wc -l  # count exportable examples`,
	Hidden: true,
	RunE:   runExportTraining,
}

func runExportTraining(cmd *cobra.Command, args []string) error {
	memoryPath, err := config.GetMemoryBasePath()
	if err != nil {
		return fmt.Errorf("get memory path: %w", err)
	}
	repo, err := memory.NewDefaultRepository(memoryPath)
	if err != nil {
		return fmt.Errorf("open memory: %w", err)
	}

	filterType, _ := cmd.Flags().GetString("type")
	minConfidence, _ := cmd.Flags().GetFloat64("min-confidence")
	outputPath, _ := cmd.Flags().GetString("output")

	nodes, err := repo.ListNodes(filterType)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	var out *os.File
	if outputPath != "" {
		out, err = os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer out.Close()
	} else {
		out = os.Stdout
	}

	exported := 0
	encoder := json.NewEncoder(out)

	for _, n := range nodes {
		sc := n.ParseStructuredContent()
		if sc == nil {
			continue
		}

		if minConfidence > 0 && n.ConfidenceScore < minConfidence {
			continue
		}

		example := buildTrainingExample(n, sc)
		if err := encoder.Encode(example); err != nil {
			return fmt.Errorf("encode example: %w", err)
		}
		exported++
	}

	if outputPath != "" {
		fmt.Fprintf(os.Stderr, "Exported %d training examples to %s\n", exported, outputPath)
	}

	return nil
}

func buildTrainingExample(n memory.Node, sc *memory.StructuredContent) trainingExample {
	// Build user message from evidence snippets (the "input" — raw source content)
	var userContent strings.Builder
	userContent.WriteString("Classify the following content from a go codebase.\n\n")
	userContent.WriteString("Content type: code_snippet\n")

	// Use first snippet's file path if available
	sourceFile := "unknown"
	if len(sc.Snippets) > 0 {
		sourceFile = sc.Snippets[0].FilePath
		userContent.WriteString(fmt.Sprintf("Source file: %s\n", sourceFile))
	}

	userContent.WriteString("\n---\n")
	if len(sc.Snippets) > 0 {
		for i, s := range sc.Snippets {
			if i > 0 {
				userContent.WriteString("\n\n")
			}
			if len(sc.Snippets) > 1 {
				userContent.WriteString(fmt.Sprintf("// %s\n", s.FilePath))
			}
			userContent.WriteString(s.Code)
		}
	} else {
		// Fallback: use description as content
		userContent.WriteString(sc.Description)
	}
	userContent.WriteString("\n---\n\n")
	userContent.WriteString(`Respond with this exact JSON structure:`)
	userContent.WriteString("\n")
	userContent.WriteString(`{"type": "<decision|feature|pattern|constraint>", "confidence": <0.0-1.0>, "one_liner": "<concise summary>", "reasoning": "<why this classification>"}`)

	// Build assistant response (the "output" — the classification result)
	reasoning := sc.Description
	if sc.Why != "" {
		reasoning += " " + sc.Why
	}
	assistantResponse := map[string]any{
		"type":       n.Type,
		"confidence": n.ConfidenceScore,
		"one_liner":  sc.Title,
		"reasoning":  reasoning,
	}
	assistantJSON, _ := json.Marshal(assistantResponse)

	return trainingExample{
		Messages: []trainingMessage{
			{Role: "system", Content: classificationSystemPrompt},
			{Role: "user", Content: userContent.String()},
			{Role: "assistant", Content: string(assistantJSON)},
		},
		Metadata: map[string]any{
			"source_file":         sourceFile,
			"content_type":        "code_snippet",
			"classification_type": n.Type,
			"node_id":             n.ID,
		},
	}
}

func init() {
	memoryCmd.AddCommand(memoryExportTrainingCmd)
	memoryExportTrainingCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	memoryExportTrainingCmd.Flags().StringP("type", "t", "", "Filter by node type (decision, feature, pattern, constraint)")
	memoryExportTrainingCmd.Flags().Float64("min-confidence", 0, "Minimum confidence score (0.0-1.0)")
}
