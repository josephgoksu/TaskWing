/*
Package knowledge provides AI-powered classification and retrieval for the knowledge graph.
*/
package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
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
	chatModel, err := llm.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}

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
	return fmt.Sprintf(`Classify this text and extract key information.

TEXT:
%s

Respond in JSON format only:
{
  "type": "decision|feature|plan|note",
  "summary": "Brief 1-line summary (max 100 chars)",
  "relations": ["topic1", "topic2"]
}

CLASSIFICATION RULES:
- "decision": Explains WHY something was chosen, trade-offs, architectural choices
- "feature": Describes WHAT a component/capability does
- "plan": Future work, TODOs, proposed changes
- "note": General information, documentation, context

JSON ONLY, no explanation:`, content)
}

func parseClassifyResponse(response string) (*ClassifyResult, error) {
	// Try to extract JSON from response
	response = strings.TrimSpace(response)

	// Handle markdown code blocks
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	// Find JSON object
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		response = response[start : end+1]
	}

	var result ClassifyResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
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
