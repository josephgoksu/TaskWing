package types

// TaskComplexity contains analysis for a single task
type TaskComplexity struct {
	TaskID              string `json:"task_id"`
	Title               string `json:"title"`
	Score               int    `json:"score"` // 1-10
	RecommendedSubtasks int    `json:"recommended_subtasks"`
	Reason              string `json:"reason,omitempty"`
	ExpandPrompt        string `json:"expand_prompt,omitempty"`
	ExpandCommand       string `json:"expand_command,omitempty"`
}

// ComplexityReport is the persisted JSON payload
type ComplexityReport struct {
	GeneratedAtISO  string           `json:"generated_at_iso"`
	Tag             string           `json:"tag,omitempty"`
	DefaultSubtasks int              `json:"default_subtasks"`
	Tasks           []TaskComplexity `json:"tasks"`
	Stats           ComplexityStats  `json:"stats"`
}

type ComplexityStats struct {
	Total  int `json:"total"`
	Low    int `json:"low"`    // score 1-3
	Medium int `json:"medium"` // 4-7
	High   int `json:"high"`   // 8-10
}
