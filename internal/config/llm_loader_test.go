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
