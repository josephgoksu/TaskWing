package config

import (
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/viper"
)

func resetViperForTest(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestResolveBedrockBaseURL_FromRegion(t *testing.T) {
	resetViperForTest(t)
	viper.Set("llm.bedrock.region", "us-west-2")

	got, err := ResolveBedrockBaseURL()
	if err != nil {
		t.Fatalf("ResolveBedrockBaseURL() error = %v", err)
	}
	want := "https://bedrock-runtime.us-west-2.amazonaws.com/openai/v1"
	t.Logf("resolved Bedrock baseURL: %s", got)
	if got != want {
		t.Fatalf("ResolveBedrockBaseURL() = %q, want %q", got, want)
	}
}

func TestResolveBedrockBaseURL_MissingRegion(t *testing.T) {
	resetViperForTest(t)
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	_, err := ResolveBedrockBaseURL()
	if err == nil {
		t.Fatal("ResolveBedrockBaseURL() error = nil, want missing-region error")
	}
	if !strings.Contains(err.Error(), "llm.bedrock.region") {
		t.Fatalf("ResolveBedrockBaseURL() error = %v, want llm.bedrock.region guidance", err)
	}
}

func TestValidateBedrockBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid bedrock endpoint",
			url:     "https://bedrock-runtime.us-east-1.amazonaws.com/openai/v1",
			wantErr: false,
		},
		{
			name:    "reject non bedrock host",
			url:     "https://api.openai.com/v1",
			wantErr: true,
		},
		{
			name:    "reject invalid path",
			url:     "https://bedrock-runtime.us-east-1.amazonaws.com/v1",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBedrockBaseURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatalf("ValidateBedrockBaseURL(%q) error = nil, want error", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateBedrockBaseURL(%q) error = %v", tc.url, err)
			}
		})
	}
}

func TestResolveBedrockRegion_FromEnvVar(t *testing.T) {
	resetViperForTest(t)

	// Simulate TASKWING_LLM_BEDROCK_REGION env var via Viper auto-bind.
	// Viper maps TASKWING_LLM_BEDROCK_REGION → llm.bedrock.region
	// when SetEnvPrefix("TASKWING") + SetEnvKeyReplacer("." → "_") is configured.
	// We test the underlying resolution directly via Viper Set to verify the path.
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	viper.Set("llm.bedrock.region", "eu-west-1")

	region := ResolveBedrockRegion()
	if region != "eu-west-1" {
		t.Fatalf("ResolveBedrockRegion() = %q, want %q", region, "eu-west-1")
	}

	got, err := ResolveBedrockBaseURL()
	if err != nil {
		t.Fatalf("ResolveBedrockBaseURL() error = %v", err)
	}
	want := "https://bedrock-runtime.eu-west-1.amazonaws.com/openai/v1"
	if got != want {
		t.Fatalf("ResolveBedrockBaseURL() = %q, want %q", got, want)
	}
}

func TestResolveBedrockRegion_FallsBackToAWSRegion(t *testing.T) {
	resetViperForTest(t)

	// No Viper config, should fall back to AWS_REGION env var
	t.Setenv("AWS_REGION", "ap-southeast-1")
	t.Setenv("AWS_DEFAULT_REGION", "")

	region := ResolveBedrockRegion()
	if region != "ap-southeast-1" {
		t.Fatalf("ResolveBedrockRegion() = %q, want %q", region, "ap-southeast-1")
	}
}

func TestResolveBedrockRegion_FallsBackToAWSDefaultRegion(t *testing.T) {
	resetViperForTest(t)

	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "us-west-1")

	region := ResolveBedrockRegion()
	if region != "us-west-1" {
		t.Fatalf("ResolveBedrockRegion() = %q, want %q", region, "us-west-1")
	}
}

func TestLoadLLMConfig_BedrockEmbeddingDefault(t *testing.T) {
	resetViperForTest(t)
	viper.Set("llm.provider", "bedrock")
	viper.Set("llm.model", "us.anthropic.claude-sonnet-4-5-20250929-v1:0")
	viper.Set("llm.bedrock.region", "us-east-1")
	viper.Set("llm.apiKeys.bedrock", "test-bedrock-key")

	cfg, err := LoadLLMConfig()
	if err != nil {
		t.Fatalf("LoadLLMConfig() error = %v", err)
	}
	if cfg.EmbeddingModel != llm.DefaultBedrockEmbeddingModel {
		t.Fatalf("LoadLLMConfig() EmbeddingModel = %q, want %q", cfg.EmbeddingModel, llm.DefaultBedrockEmbeddingModel)
	}
}

func TestLoadLLMConfig_Bedrock(t *testing.T) {
	resetViperForTest(t)
	viper.Set("llm.provider", "bedrock")
	viper.Set("llm.model", "us.anthropic.claude-sonnet-4-5-20250929-v1:0")
	viper.Set("llm.bedrock.region", "us-east-1")
	viper.Set("llm.apiKeys.bedrock", "test-bedrock-key")

	cfg, err := LoadLLMConfig()
	if err != nil {
		t.Fatalf("LoadLLMConfig() error = %v", err)
	}
	if cfg.Provider != llm.ProviderBedrock {
		t.Fatalf("LoadLLMConfig() provider = %q, want %q", cfg.Provider, llm.ProviderBedrock)
	}
	if cfg.BaseURL != "https://bedrock-runtime.us-east-1.amazonaws.com/openai/v1" {
		t.Fatalf("LoadLLMConfig() baseURL = %q", cfg.BaseURL)
	}
	if cfg.APIKey != "test-bedrock-key" {
		t.Fatalf("LoadLLMConfig() apiKey mismatch")
	}
}

// ============================================
// TaskWing managed provider tests
// ============================================

func TestResolveProviderBaseURL_TaskWing_Default(t *testing.T) {
	resetViperForTest(t)
	got, err := ResolveProviderBaseURL(llm.ProviderTaskWing)
	if err != nil {
		t.Fatalf("ResolveProviderBaseURL(taskwing) error = %v", err)
	}
	if got != llm.DefaultTaskWingURL {
		t.Fatalf("ResolveProviderBaseURL(taskwing) = %q, want %q", got, llm.DefaultTaskWingURL)
	}
}

func TestResolveProviderBaseURL_TaskWing_Custom(t *testing.T) {
	resetViperForTest(t)
	customURL := "https://custom.inference.example.com/v1"
	viper.Set("llm.taskwing.base_url", customURL)
	got, err := ResolveProviderBaseURL(llm.ProviderTaskWing)
	if err != nil {
		t.Fatalf("ResolveProviderBaseURL(taskwing) error = %v", err)
	}
	if got != customURL {
		t.Fatalf("ResolveProviderBaseURL(taskwing) = %q, want %q", got, customURL)
	}
}

func TestParseModelSpec_TaskWingProvider(t *testing.T) {
	resetViperForTest(t)
	t.Setenv("TASKWING_API_KEY", "tw-test-key")

	cfg, err := ParseModelSpec("taskwing:taskwing-brain", llm.RoleBootstrap)
	if err != nil {
		t.Fatalf("ParseModelSpec(taskwing:taskwing-brain) error = %v", err)
	}
	if cfg.Provider != llm.ProviderTaskWing {
		t.Fatalf("provider = %q, want %q", cfg.Provider, llm.ProviderTaskWing)
	}
	if cfg.Model != "taskwing-brain" {
		t.Fatalf("model = %q, want taskwing-brain", cfg.Model)
	}
	if cfg.APIKey != "tw-test-key" {
		t.Fatalf("apiKey = %q, want tw-test-key", cfg.APIKey)
	}
	if cfg.BaseURL != llm.DefaultTaskWingURL {
		t.Fatalf("baseURL = %q, want %q", cfg.BaseURL, llm.DefaultTaskWingURL)
	}
}

// Regression test: a stale llm.baseURL pointing to localhost must not leak into
// cloud providers (OpenAI, Anthropic, Gemini). This was the root cause of bootstrap
// agents hitting localhost:11434 despite being configured for OpenAI.
func TestResolveProviderBaseURL_OpenAI_IgnoresLocalhostBaseURL(t *testing.T) {
	resetViperForTest(t)
	viper.Set("llm.baseURL", "http://localhost:11434")

	got, err := ResolveProviderBaseURL(llm.ProviderOpenAI)
	if err != nil {
		t.Fatalf("ResolveProviderBaseURL(openai) error = %v", err)
	}
	if got != "" {
		t.Fatalf("ResolveProviderBaseURL(openai) = %q, want empty (should ignore localhost baseURL)", got)
	}
}

func TestResolveProviderBaseURL_OpenAI_AllowsCustomEndpoint(t *testing.T) {
	resetViperForTest(t)
	customURL := "https://my-proxy.example.com/v1"
	viper.Set("llm.baseURL", customURL)

	got, err := ResolveProviderBaseURL(llm.ProviderOpenAI)
	if err != nil {
		t.Fatalf("ResolveProviderBaseURL(openai) error = %v", err)
	}
	if got != customURL {
		t.Fatalf("ResolveProviderBaseURL(openai) = %q, want %q", got, customURL)
	}
}

func TestParseModelSpec_TaskWingNoKey(t *testing.T) {
	resetViperForTest(t)
	t.Setenv("TASKWING_API_KEY", "")

	_, err := ParseModelSpec("taskwing:taskwing-brain", llm.RoleBootstrap)
	if err == nil {
		t.Fatal("ParseModelSpec(taskwing:...) should fail without TASKWING_API_KEY")
	}
	if !strings.Contains(err.Error(), "TASKWING_API_KEY") {
		t.Fatalf("error should mention TASKWING_API_KEY, got: %v", err)
	}
}
