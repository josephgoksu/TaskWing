package llm

// pricing.go provides backwards compatibility for code that uses the old pricing API.
// The actual model data now lives in models.go which is the single source of truth.

// ModelPricing holds pricing info for a model (per 1M tokens in USD)
// Deprecated: Use GetModel() from models.go instead.
type ModelPricing struct {
	Provider    string
	Model       string
	InputPer1M  float64
	OutputPer1M float64
}

// GetPricing returns pricing for a model, or nil if unknown.
// Deprecated: Use GetModel() from models.go instead.
func GetPricing(modelID string) *ModelPricing {
	m := GetModel(modelID)
	if m == nil {
		return nil
	}
	return &ModelPricing{
		Provider:    m.Provider,
		Model:       m.ID,
		InputPer1M:  m.InputPer1M,
		OutputPer1M: m.OutputPer1M,
	}
}
