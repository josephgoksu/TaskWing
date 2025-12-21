package agents

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// MockChatModel implements model.BaseChatModel for testing
type MockChatModel struct {
	Response *schema.Message
	Err      error
}

func (m *MockChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Response, nil
}

func (m *MockChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil // Not used in this agent
}

func TestReactCodeAgent_Run(t *testing.T) {
	// Sample JSON response mimicking agent output
	jsonResponse := `{
		"decisions": [
			{
				"title": "Use Go",
				"what": "Backend language",
				"why": "Performance",
				"tradeoffs": "Verbose",
				"confidence": "high",
				"evidence": ["go.mod"]
			}
		]
	}`

	mockModel := &MockChatModel{
		Response: &schema.Message{
			Role:    schema.Assistant,
			Content: jsonResponse,
		},
	}

	agent := NewReactCodeAgent(llm.Config{}, ".")
	// Inject mock factory
	agent.modelFactory = func(ctx context.Context, cfg llm.Config) (model.BaseChatModel, error) {
		return mockModel, nil
	}

	output, err := agent.Run(context.Background(), Input{ProjectName: "TestProject"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(output.Findings) != 1 {
		t.Errorf("Run() got %d findings, want 1", len(output.Findings))
	}

	if output.Findings[0].Title != "Use Go" {
		t.Errorf("Run() finding title = %s, want 'Use Go'", output.Findings[0].Title)
	}
}
