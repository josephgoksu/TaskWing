package llm

import "testing"

func TestInferProviderFromModel(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		wantProvider string
		wantOk       bool
	}{
		// OpenAI models
		{"gpt-5-mini", "gpt-5-mini", ProviderOpenAI, true},
		{"gpt-5-mini dated", "gpt-5-mini-2025-08-07", ProviderOpenAI, true},
		{"gpt-4o", "gpt-4o", ProviderOpenAI, true},
		{"gpt-4o-mini", "gpt-4o-mini", ProviderOpenAI, true},
		{"o1 model", "o1-preview", ProviderOpenAI, true},
		{"o3 model", "o3-mini", ProviderOpenAI, true},

		// Anthropic models
		{"claude-3-5-sonnet-latest", "claude-3-5-sonnet-latest", ProviderAnthropic, true},
		{"claude-3-5-sonnet dated", "claude-3-5-sonnet-20241022", ProviderAnthropic, true},
		{"claude-3-opus", "claude-3-opus-latest", ProviderAnthropic, true},
		{"claude-3-haiku", "claude-3-haiku-20240307", ProviderAnthropic, true},

		// Gemini models
		{"gemini-2.0-flash", "gemini-2.0-flash", ProviderGemini, true},
		{"gemini-2.5-pro", "gemini-2.5-pro", ProviderGemini, true},
		{"gemini-3-flash-preview", "gemini-3-flash-preview", ProviderGemini, true},
		{"gemini-1.5-pro", "gemini-1.5-pro", ProviderGemini, true},

		// Ollama models (prefix-based)
		{"llama3.2", "llama3.2", ProviderOllama, true},
		{"codellama", "codellama:7b", ProviderOllama, true},
		{"phi", "phi3", ProviderOllama, true},

		// Unknown models
		{"unknown model", "some-random-model", "", false},
		{"empty string", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, ok := InferProviderFromModel(tt.model)
			if ok != tt.wantOk {
				t.Errorf("InferProviderFromModel(%q) ok = %v, want %v", tt.model, ok, tt.wantOk)
			}
			if provider != tt.wantProvider {
				t.Errorf("InferProviderFromModel(%q) = %q, want %q", tt.model, provider, tt.wantProvider)
			}
		})
	}
}
