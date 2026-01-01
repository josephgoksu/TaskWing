package llm

import (
	"context"
	"testing"
)

func TestValidateProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     Provider
		wantErr  bool
	}{
		{
			name:     "valid openai",
			provider: "openai",
			want:     ProviderOpenAI,
			wantErr:  false,
		},
		{
			name:     "valid ollama",
			provider: "ollama",
			want:     ProviderOllama,
			wantErr:  false,
		},
		{
			name:     "valid anthropic",
			provider: "anthropic",
			want:     ProviderAnthropic,
			wantErr:  false,
		},
		{
			name:     "valid gemini",
			provider: "gemini",
			want:     ProviderGemini,
			wantErr:  false,
		},
		{
			name:     "invalid provider",
			provider: "invalid",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "empty provider",
			provider: "",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "case sensitive - OPENAI fails",
			provider: "OPENAI",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateProvider(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProvider(%q) error = %v, wantErr %v", tt.provider, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateProvider(%q) = %v, want %v", tt.provider, got, tt.want)
			}
		})
	}
}

func TestDefaultModelForProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{
			name:     "openai default model",
			provider: "openai",
			want:     "gpt-5-mini-2025-08-07", // From ModelRegistry, uses alias for API compat
		},
		{
			name:     "ollama default model",
			provider: "ollama",
			want:     "llama3.2",
		},
		{
			name:     "anthropic default model",
			provider: "anthropic",
			want:     "claude-3-5-sonnet-latest",
		},
		{
			name:     "gemini default model",
			provider: "gemini",
			want:     "gemini-2.0-flash",
		},
		{
			name:     "unknown provider returns empty",
			provider: "unknown",
			want:     "",
		},
		{
			name:     "empty provider returns empty",
			provider: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultModelForProvider(tt.provider)
			if got != tt.want {
				t.Errorf("DefaultModelForProvider(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestNewCloseableChatModel_Validation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "openai requires API key",
			cfg: Config{
				Provider: ProviderOpenAI,
				Model:    "gpt-4",
				APIKey:   "",
			},
			wantErr: "OpenAI API key is required",
		},
		{
			name: "anthropic requires API key",
			cfg: Config{
				Provider: ProviderAnthropic,
				Model:    "claude-3",
				APIKey:   "",
			},
			wantErr: "anthropic API key is required",
		},
		{
			name: "gemini requires API key",
			cfg: Config{
				Provider: ProviderGemini,
				Model:    "gemini-pro",
				APIKey:   "",
			},
			wantErr: "gemini API key is required",
		},
		{
			name: "unsupported provider",
			cfg: Config{
				Provider: "unknown",
				Model:    "model",
				APIKey:   "key",
			},
			wantErr: "unsupported LLM provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCloseableChatModel(ctx, tt.cfg)
			if err == nil {
				t.Errorf("NewCloseableChatModel() expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !containsString(err.Error(), tt.wantErr) {
				t.Errorf("NewCloseableChatModel() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestNewCloseableEmbedder_Validation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "openai requires API key",
			cfg: Config{
				Provider: ProviderOpenAI,
				APIKey:   "",
			},
			wantErr: "OpenAI API key is required",
		},
		{
			name: "gemini requires API key",
			cfg: Config{
				Provider: ProviderGemini,
				APIKey:   "",
			},
			wantErr: "gemini API key is required",
		},
		{
			name: "unsupported provider",
			cfg: Config{
				Provider: "unknown",
				APIKey:   "key",
			},
			wantErr: "unsupported LLM provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCloseableEmbedder(ctx, tt.cfg)
			if err == nil {
				t.Errorf("NewCloseableEmbedder() expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !containsString(err.Error(), tt.wantErr) {
				t.Errorf("NewCloseableEmbedder() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCloseableChatModel_Close(t *testing.T) {
	// Test that Close() is safe to call on a model without a closer
	cm := &CloseableChatModel{
		BaseChatModel: nil,
		closer:        nil,
	}

	err := cm.Close()
	if err != nil {
		t.Errorf("Close() on nil closer should return nil, got %v", err)
	}

	// Test multiple closes are safe
	err = cm.Close()
	if err != nil {
		t.Errorf("Second Close() should return nil, got %v", err)
	}
}

func TestCloseableEmbedder_Close(t *testing.T) {
	// Test that Close() is safe to call on an embedder without a closer
	ce := &CloseableEmbedder{
		Embedder: nil,
		closer:   nil,
	}

	err := ce.Close()
	if err != nil {
		t.Errorf("Close() on nil closer should return nil, got %v", err)
	}
}

func TestGenaiClientCloser_Close(t *testing.T) {
	// Test that genaiClientCloser sets client to nil on close
	closer := &genaiClientCloser{
		client: nil, // Can't create real client without credentials
	}

	err := closer.Close()
	if err != nil {
		t.Errorf("genaiClientCloser.Close() should return nil, got %v", err)
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
