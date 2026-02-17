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

// ============================================
// TaskWing provider and model tests
// ============================================

func TestValidateProvider_TaskWing(t *testing.T) {
	got, err := ValidateProvider("taskwing")
	if err != nil {
		t.Fatalf("ValidateProvider(taskwing) error = %v", err)
	}
	if got != ProviderTaskWing {
		t.Fatalf("ValidateProvider(taskwing) = %q, want %q", got, ProviderTaskWing)
	}
}

func TestGetProviders_IncludesTaskWing(t *testing.T) {
	providers := GetProviders()
	found := false
	for _, p := range providers {
		if p.ID == ProviderTaskWing {
			found = true
			if p.EnvVar != "TASKWING_API_KEY" {
				t.Fatalf("TaskWing provider EnvVar = %q, want TASKWING_API_KEY", p.EnvVar)
			}
			if p.DefaultModel != ModelTaskWingBrain {
				t.Fatalf("TaskWing provider DefaultModel = %q, want %q", p.DefaultModel, ModelTaskWingBrain)
			}
			break
		}
	}
	if !found {
		t.Fatalf("GetProviders() missing %q", ProviderTaskWing)
	}
}

func TestGetModel_TaskWingBrain(t *testing.T) {
	tests := []struct {
		modelID  string
		wantNil  bool
		wantProv string
	}{
		{ModelTaskWingBrain, false, ProviderTaskWing},
		{ModelTaskWingBrainLite, false, ProviderTaskWing},
		{"taskwing-brain-7b", false, ProviderTaskWing}, // alias
		{"taskwing-brain-4b", false, ProviderTaskWing}, // alias
	}
	for _, tc := range tests {
		t.Run(tc.modelID, func(t *testing.T) {
			m := GetModel(tc.modelID)
			if tc.wantNil && m != nil {
				t.Fatalf("GetModel(%q) = %v, want nil", tc.modelID, m)
			}
			if !tc.wantNil && m == nil {
				t.Fatalf("GetModel(%q) = nil, want non-nil", tc.modelID)
			}
			if m != nil && m.ProviderID != tc.wantProv {
				t.Fatalf("GetModel(%q).ProviderID = %q, want %q", tc.modelID, m.ProviderID, tc.wantProv)
			}
		})
	}
}

func TestInferProvider_TaskWingBrain(t *testing.T) {
	tests := []string{
		"taskwing-brain",
		"taskwing-brain-7b",
		"taskwing-brain-lite",
		"taskwing-brain-custom",
	}
	for _, modelID := range tests {
		provider, ok := InferProvider(modelID)
		if !ok {
			t.Fatalf("InferProvider(%q) = not inferred", modelID)
		}
		if provider != ProviderTaskWing {
			t.Fatalf("InferProvider(%q) = %q, want %q", modelID, provider, ProviderTaskWing)
		}
	}
}

func TestTaskWingBrain_ManagedPricing(t *testing.T) {
	m := GetModel(ModelTaskWingBrain)
	if m == nil {
		t.Fatal("GetModel(taskwing-brain) = nil")
	}
	// Managed models have $0 in registry (pricing is account-based, not per-token in registry)
	cost := CalculateCost(ModelTaskWingBrain, 1_000_000, 1_000_000)
	if cost != 0 {
		t.Fatalf("CalculateCost(taskwing-brain, 1M, 1M) = $%.4f, want $0.00", cost)
	}
}

func TestTaskWingBrain_MaxInputTokens(t *testing.T) {
	tokens := GetMaxInputTokens(ModelTaskWingBrain)
	if tokens != 32_768 {
		t.Fatalf("GetMaxInputTokens(taskwing-brain) = %d, want 32768", tokens)
	}
}

func TestTaskWingBrain_Categories(t *testing.T) {
	m := GetModel(ModelTaskWingBrain)
	if m == nil {
		t.Fatal("GetModel(taskwing-brain) = nil")
	}
	if m.Category != CategoryBalanced {
		t.Fatalf("taskwing-brain category = %q, want %q", m.Category, CategoryBalanced)
	}

	mLite := GetModel(ModelTaskWingBrainLite)
	if mLite == nil {
		t.Fatal("GetModel(taskwing-brain-lite) = nil")
	}
	if mLite.Category != CategoryFast {
		t.Fatalf("taskwing-brain-lite category = %q, want %q", mLite.Category, CategoryFast)
	}
}

func TestTaskWingBrain_IsDefault(t *testing.T) {
	m := GetDefaultModel(ProviderTaskWing)
	if m == nil {
		t.Fatal("GetDefaultModel(taskwing) = nil")
	}
	if m.ID != ModelTaskWingBrain {
		t.Fatalf("GetDefaultModel(taskwing) = %q, want %q", m.ID, ModelTaskWingBrain)
	}
}

func TestGetEnvVarForProvider_TaskWing(t *testing.T) {
	envVar := GetEnvVarForProvider(ProviderTaskWing)
	if envVar != "TASKWING_API_KEY" {
		t.Fatalf("GetEnvVarForProvider(taskwing) = %q, want TASKWING_API_KEY", envVar)
	}
}
