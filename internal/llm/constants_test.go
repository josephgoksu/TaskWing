package llm

import "testing"

func TestInferProviderFromModel(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		wantProvider string
		wantOk       bool
	}{
		// OpenAI models (current)
		{"gpt-5-mini", "gpt-5-mini", ProviderOpenAI, true},
		{"gpt-5", "gpt-5", ProviderOpenAI, true},
		{"o3", "o3", ProviderOpenAI, true},
		{"o4-mini", "o4-mini", ProviderOpenAI, true},
		{"gpt-4.1", "gpt-4.1", ProviderOpenAI, true},
		// Legacy OpenAI (prefix inference should still work)
		{"gpt-4o legacy", "gpt-4o", ProviderOpenAI, true},
		{"o1 model", "o1-preview", ProviderOpenAI, true},

		// Anthropic models (current)
		{"claude-sonnet-4-5", "claude-sonnet-4-5", ProviderAnthropic, true},
		{"claude-opus-4-5", "claude-opus-4-5", ProviderAnthropic, true},
		{"claude-haiku-4-5", "claude-haiku-4-5", ProviderAnthropic, true},
		// Legacy Claude (prefix inference should still work)
		{"claude-3-opus legacy", "claude-3-opus-latest", ProviderAnthropic, true},

		// Gemini models (current)
		{"gemini-2.0-flash", "gemini-2.0-flash", ProviderGemini, true},
		{"gemini-2.5-pro", "gemini-2.5-pro", ProviderGemini, true},
		{"gemini-3-pro-preview", "gemini-3-pro-preview", ProviderGemini, true},
		// Legacy Gemini (prefix inference should still work)
		{"gemini-1.5-pro legacy", "gemini-1.5-pro", ProviderGemini, true},

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
