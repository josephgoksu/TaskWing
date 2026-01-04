/*
Package knowledge provides AI-powered classification and retrieval for the knowledge graph.
*/
package knowledge

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// ClassifyResult contains the AI-inferred classification of content.
type ClassifyResult struct {
	Type      string   `json:"type"`      // decision, feature, plan, note
	Summary   string   `json:"summary"`   // Extracted title/summary
	Relations []string `json:"relations"` // Related topics/concepts
}

// Classify uses LLM to classify content and extract metadata.
func Classify(ctx context.Context, content string, cfg llm.Config) (*ClassifyResult, error) {
	chatModel, err := llm.NewCloseableChatModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}
	defer func() { _ = chatModel.Close() }()

	prompt := buildClassifyPrompt(content)

	messages := []*schema.Message{
		{Role: schema.User, Content: prompt},
	}

	// Use streaming for responsiveness
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("stream: %w", err)
	}
	defer stream.Close()

	var sb strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("recv: %w", err)
		}
		sb.WriteString(chunk.Content)
	}

	response := sb.String()

	// Parse JSON response
	result, err := parseClassifyResponse(response)
	if err != nil {
		// Fallback: try to extract from non-JSON response
		return &ClassifyResult{
			Type:    memory.NodeTypeNote,
			Summary: utils.Truncate(content, 100),
		}, nil
	}

	return result, nil
}

func buildClassifyPrompt(content string) string {
	return fmt.Sprintf(config.PromptTemplateClassify, content)
}

func parseClassifyResponse(response string) (*ClassifyResult, error) {
	result, err := utils.ExtractAndParseJSON[ClassifyResult](response)
	if err != nil {
		return nil, err
	}

	// Validate type
	validTypes := map[string]bool{
		memory.NodeTypeDecision: true,
		memory.NodeTypeFeature:  true,
		memory.NodeTypePlan:     true,
		memory.NodeTypeNote:     true,
	}
	if !validTypes[result.Type] {
		result.Type = memory.NodeTypeNote
	}

	return &result, nil
}
