package llm

import "testing"

func TestGetPricing(t *testing.T) {
	tests := []struct {
		model        string
		wantNil      bool
		wantProvider string
	}{
		{"gpt-5-mini", false, "OpenAI"},
		{"gpt-5-mini-2025-08-07", false, "OpenAI"},
		{"claude-3-5-sonnet-latest", false, "Anthropic"},
		{"gemini-2.0-flash", false, "Google"},
		{"unknown-model", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			p := GetPricing(tt.model)
			if tt.wantNil {
				if p != nil {
					t.Errorf("GetPricing(%q) expected nil, got %+v", tt.model, p)
				}
				return
			}
			if p == nil {
				t.Errorf("GetPricing(%q) expected non-nil", tt.model)
				return
			}
			if p.Provider != tt.wantProvider {
				t.Errorf("GetPricing(%q).Provider = %q, want %q", tt.model, p.Provider, tt.wantProvider)
			}
		})
	}
}

func TestGetModelsForProvider(t *testing.T) {
	tests := []struct {
		provider   string
		wantEmpty  bool
		wantMinLen int
	}{
		{"openai", false, 3},
		{"anthropic", false, 2},
		{"gemini", false, 3},
		{"ollama", false, 1}, // Ollama has models (local, no pricing)
		{"unknown", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			models := GetModelsForProvider(tt.provider)
			if tt.wantEmpty {
				if len(models) != 0 {
					t.Errorf("GetModelsForProvider(%q) expected empty, got %d models", tt.provider, len(models))
				}
				return
			}
			if len(models) < tt.wantMinLen {
				t.Errorf("GetModelsForProvider(%q) got %d models, want at least %d", tt.provider, len(models), tt.wantMinLen)
			}

			// Verify default is first
			if len(models) > 0 && !models[0].IsDefault {
				t.Errorf("GetModelsForProvider(%q) first model should be default", tt.provider)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		model        string
		inputTokens  int
		outputTokens int
		wantCost     float64
	}{
		{"gpt-5-mini", 1000000, 0, 0.22},            // 1M input at $0.22/1M
		{"gpt-5-mini", 0, 1000000, 1.80},            // 1M output at $1.80/1M
		{"gpt-5-mini", 500000, 500000, 0.11 + 0.90}, // Half each
		{"unknown-model", 1000000, 1000000, 0},      // Unknown returns 0
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			cost := CalculateCost(tt.model, tt.inputTokens, tt.outputTokens)
			// Allow small floating point difference
			if cost < tt.wantCost-0.01 || cost > tt.wantCost+0.01 {
				t.Errorf("CalculateCost(%q, %d, %d) = %f, want %f", tt.model, tt.inputTokens, tt.outputTokens, cost, tt.wantCost)
			}
		})
	}
}
