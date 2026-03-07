/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getLLMConfig loads LLM config with interactive prompts for missing values.
// Delegates core logic to config.LoadLLMConfig (single source of truth).
func getLLMConfig(cmd *cobra.Command) (llm.Config, error) {
	// Apply flag overrides before loading config
	if p, _ := cmd.Flags().GetString("provider"); p != "" {
		viper.Set("llm.provider", p)
	}
	if m, err := cmd.Flags().GetString("model"); err == nil && m != "" {
		viper.Set("llm.model", m)
		// Infer provider from model if provider not explicitly set
		if !cmd.Flags().Changed("provider") {
			if inferred, ok := llm.InferProviderFromModel(m); ok {
				viper.Set("llm.provider", inferred)
			}
		}
	}
	if k, _ := cmd.Flags().GetString("api-key"); k != "" {
		provider := viper.GetString("llm.provider")
		if provider == "" {
			provider = llm.DefaultProvider
		}
		viper.Set(fmt.Sprintf("llm.apiKeys.%s", provider), k)
	}
	if cmd.Flags().Changed("ollama-url") {
		if u, _ := cmd.Flags().GetString("ollama-url"); u != "" {
			viper.Set("llm.ollamaURL", u)
		}
	}

	// Interactive prompts if running in terminal and values still missing
	if ui.IsInteractive() {
		if !viper.IsSet("llm.provider") {
			if selected, err := ui.PromptLLMProvider(); err == nil && selected != "" {
				viper.Set("llm.provider", selected)
			}
		}

		provider := viper.GetString("llm.provider")
		if provider == "" {
			provider = llm.DefaultProvider
		}

		if !viper.IsSet("llm.model") {
			if selected, err := ui.PromptModelSelection(provider); err == nil && selected != "" {
				viper.Set("llm.model", selected)
			}
		}

		llmProvider, _ := llm.ValidateProvider(provider)
		requiresKey := llmProvider == llm.ProviderOpenAI ||
			llmProvider == llm.ProviderAnthropic ||
			llmProvider == llm.ProviderGemini ||
			llmProvider == llm.ProviderBedrock ||
			llmProvider == llm.ProviderTaskWing

		apiKey := config.ResolveAPIKey(llmProvider)
		if requiresKey && apiKey == "" {
			fmt.Printf("No API key found for %s.\n", provider)
			if inputKey, err := ui.PromptAPIKey(); err == nil && inputKey != "" {
				viper.Set(fmt.Sprintf("llm.apiKeys.%s", llmProvider), inputKey)
				model := viper.GetString("llm.model")
				if err := config.SaveGlobalLLMConfigWithModel(provider, model, inputKey); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to save config: %v\n", err)
				} else {
					ui.PrintSuccess("Configuration saved to ~/.taskwing/config.yaml")
				}
			}
		}

		if llmProvider == llm.ProviderBedrock && config.ResolveBedrockRegion() == "" {
			if region, err := promptBedrockRegion(); err == nil {
				viper.Set("llm.bedrock.region", region)
			}
		}
	}

	// Delegate to single source of truth
	return config.LoadLLMConfig()
}

func promptBedrockRegion() (string, error) {
	const defaultRegion = "us-east-1"
	fmt.Printf("AWS Bedrock region [%s]: ", defaultRegion)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	region := strings.TrimSpace(line)
	if region == "" {
		region = defaultRegion
	}
	return region, nil
}

// getLLMConfigForRole returns the appropriate LLM config for a specific role.
// Checks role-specific config first (llm.models.<role>), then falls back to getLLMConfig.
func getLLMConfigForRole(cmd *cobra.Command, role llm.ModelRole) (llm.Config, error) {
	roleConfigKey := fmt.Sprintf("llm.models.%s", role)
	if viper.IsSet(roleConfigKey) {
		spec := viper.GetString(roleConfigKey)
		return config.ParseModelSpec(spec, role)
	}
	return getLLMConfig(cmd)
}
