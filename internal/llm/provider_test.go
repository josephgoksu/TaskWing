package llm

import "testing"

func TestValidateProvider_Bedrock(t *testing.T) {
	got, err := ValidateProvider("bedrock")
	if err != nil {
		t.Fatalf("ValidateProvider(bedrock) error = %v", err)
	}
	if got != ProviderBedrock {
		t.Fatalf("ValidateProvider(bedrock) = %q, want %q", got, ProviderBedrock)
	}
}

func TestInferProvider_BedrockModelID(t *testing.T) {
	tests := []string{
		"anthropic.claude-opus-4-6-v1",
		"us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		"amazon.nova-pro-v1:0",
		"amazon.nova-premier-v1:0",
		"meta.llama4-maverick-17b-instruct-v1:0",
		"meta.llama3-3-70b-instruct-v1:0",
		"openai.gpt-oss-120b-1:0",
		"qwen.qwen3-235b-a22b-instruct-2507-v1:0",
		"google.gemma-3-27b-it-v1:0",
	}
	for _, modelID := range tests {
		provider, ok := InferProvider(modelID)
		if !ok {
			t.Fatalf("InferProvider(%q) = not inferred", modelID)
		}
		if provider != ProviderBedrock {
			t.Fatalf("InferProvider(%q) = %q, want %q", modelID, provider, ProviderBedrock)
		}
	}
}

func TestGetProviders_IncludesBedrock(t *testing.T) {
	providers := GetProviders()
	t.Logf("providers: %+v", providers)
	found := false
	for _, p := range providers {
		if p.ID == ProviderBedrock {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("GetProviders() missing %q", ProviderBedrock)
	}
}
