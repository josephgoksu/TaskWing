package llm

// ModelPricing holds pricing info for a model (per 1M tokens in USD)
type ModelPricing struct {
	Provider    string
	Model       string
	InputPer1M  float64 // $ per 1M input tokens
	OutputPer1M float64 // $ per 1M output tokens
}

// PricingTable contains known model pricing
// Prices last updated: 2025-12
var PricingTable = map[string]ModelPricing{
	// OpenAI
	"gpt-5.1":                 {Provider: "OpenAI", Model: "gpt-5.1", InputPer1M: 1.10, OutputPer1M: 9.00},
	"gpt-5-mini":              {Provider: "OpenAI", Model: "gpt-5-mini", InputPer1M: 0.22, OutputPer1M: 1.80},
	"gpt-5-mini-2025-08-07":   {Provider: "OpenAI", Model: "gpt-5-mini", InputPer1M: 0.22, OutputPer1M: 1.80},
	"gpt-5-nano":              {Provider: "OpenAI", Model: "gpt-5-nano", InputPer1M: 0.04, OutputPer1M: 0.36},
	"gpt-5-nano-2025-09-25":   {Provider: "OpenAI", Model: "gpt-5-nano", InputPer1M: 0.04, OutputPer1M: 0.36},
	"gpt-4.1-mini":            {Provider: "OpenAI", Model: "gpt-4.1-mini", InputPer1M: 0.15, OutputPer1M: 0.60},
	"gpt-4.1-mini-2025-04-14": {Provider: "OpenAI", Model: "gpt-4.1-mini", InputPer1M: 0.15, OutputPer1M: 0.60},
	"gpt-4o":                  {Provider: "OpenAI", Model: "gpt-4o", InputPer1M: 2.50, OutputPer1M: 10.00},
	"gpt-4o-2024-08-06":       {Provider: "OpenAI", Model: "gpt-4o", InputPer1M: 2.50, OutputPer1M: 10.00},
	"gpt-4o-mini":             {Provider: "OpenAI", Model: "gpt-4o-mini", InputPer1M: 0.15, OutputPer1M: 0.60},
	"gpt-4o-mini-2024-07-18":  {Provider: "OpenAI", Model: "gpt-4o-mini", InputPer1M: 0.15, OutputPer1M: 0.60},
	"gpt-4-turbo":             {Provider: "OpenAI", Model: "gpt-4-turbo", InputPer1M: 10.00, OutputPer1M: 30.00},
	"gpt-3.5-turbo":           {Provider: "OpenAI", Model: "gpt-3.5-turbo", InputPer1M: 0.50, OutputPer1M: 1.50},

	// Anthropic
	"claude-opus-4.5":   {Provider: "Anthropic", Model: "claude-opus-4.5", InputPer1M: 5.00, OutputPer1M: 25.00},
	"claude-sonnet-4.5": {Provider: "Anthropic", Model: "claude-sonnet-4.5", InputPer1M: 3.00, OutputPer1M: 15.00},
	"claude-haiku-4.5":  {Provider: "Anthropic", Model: "claude-haiku-4.5", InputPer1M: 1.00, OutputPer1M: 5.00},
	"claude-3-opus":     {Provider: "Anthropic", Model: "claude-3-opus", InputPer1M: 15.00, OutputPer1M: 75.00},
	"claude-3-sonnet":   {Provider: "Anthropic", Model: "claude-3-sonnet", InputPer1M: 3.00, OutputPer1M: 15.00},
	"claude-3-haiku":    {Provider: "Anthropic", Model: "claude-3-haiku", InputPer1M: 0.25, OutputPer1M: 1.25},

	// Google
	"gemini-2.5-flash":     {Provider: "Google", Model: "gemini-2.5-flash", InputPer1M: 0.07, OutputPer1M: 0.30},
	"gemini-3-pro-preview": {Provider: "Google", Model: "gemini-3-pro-preview", InputPer1M: 2.00, OutputPer1M: 12.00},
	"gemini-pro":           {Provider: "Google", Model: "gemini-pro", InputPer1M: 0.50, OutputPer1M: 1.50},
	"gemini-1.5-pro":       {Provider: "Google", Model: "gemini-1.5-pro", InputPer1M: 1.25, OutputPer1M: 5.00},
	"gemini-1.5-flash":     {Provider: "Google", Model: "gemini-1.5-flash", InputPer1M: 0.075, OutputPer1M: 0.30},

	// Mistral
	"mistral-medium": {Provider: "Mistral", Model: "mistral-medium", InputPer1M: 2.70, OutputPer1M: 8.10},
	"mistral-small":  {Provider: "Mistral", Model: "mistral-small", InputPer1M: 1.00, OutputPer1M: 3.00},
	"mistral-large":  {Provider: "Mistral", Model: "mistral-large", InputPer1M: 4.00, OutputPer1M: 12.00},
}

// GetPricing returns pricing for a model, or nil if unknown
func GetPricing(model string) *ModelPricing {
	if p, ok := PricingTable[model]; ok {
		return &p
	}
	return nil
}

// CalculateCost calculates cost in USD for token usage
func CalculateCost(model string, inputTokens, outputTokens int) float64 {
	p := GetPricing(model)
	if p == nil {
		return 0 // Unknown model
	}
	inputCost := float64(inputTokens) / 1_000_000 * p.InputPer1M
	outputCost := float64(outputTokens) / 1_000_000 * p.OutputPer1M
	return inputCost + outputCost
}
