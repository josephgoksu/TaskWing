package llm

// LLMConfig holds the specific configuration fields needed by the LLM package.
// This helps avoid circular dependencies with the cmd package.
type LLMConfig struct {
	Provider        string
	ModelName       string
	APIKey          string // Resolved API key
	ProjectID       string // For Google Cloud
	MaxOutputTokens int
	Temperature     float64
}

// TaskOutput is the expected structure for tasks extracted by an LLM.
// This structure is designed to be easily convertible to models.Task.
type TaskOutput struct {
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	Priority        string       `json:"priority"` // e.g., "high", "medium", "low", "urgent"
	Subtasks        []TaskOutput `json:"subtasks,omitempty"`
	DependsOnTitles []string     `json:"dependsOnTitles,omitempty"` // Titles of other tasks in the same PRD
}

// Provider defines the interface for interacting with different LLM providers
// to generate tasks from a document.
type Provider interface {
	// GenerateTasks takes the content of a document (e.g., PRD), model parameters,
	// and returns a list of TaskOutput objects or an error.
	GenerateTasks(prdContent string, modelName string, apiKey string, projectID string, maxTokens int, temperature float64) ([]TaskOutput, error)
}
