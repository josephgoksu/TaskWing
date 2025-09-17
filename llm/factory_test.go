package llm

import (
	"testing"

	"github.com/josephgoksu/TaskWing/types"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name      string
		config    types.LLMConfig
		wantType  string
		wantError bool
	}{
		{
			name: "openai provider",
			config: types.LLMConfig{
				Provider:  "openai",
				APIKey:    "test-key",
				ModelName: "gpt-4",
			},
			wantType:  "*llm.OpenAIProvider",
			wantError: false,
		},
		{
			name: "unsupported provider",
			config: types.LLMConfig{
				Provider: "unsupported",
			},
			wantType:  "",
			wantError: true,
		},
		{
			name: "empty provider returns error",
			config: types.LLMConfig{
				Provider:  "",
				APIKey:    "test-key",
				ModelName: "gpt-4",
			},
			wantType:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(&tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("NewProvider() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && provider == nil {
				t.Errorf("NewProvider() returned nil provider")
			}
		})
	}
}
