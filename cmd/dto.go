package cmd

type deletedResponse struct {
	Status string `json:"status"`
	ID     string `json:"id"`
	Goal   string `json:"goal,omitempty"`
	Type   string `json:"type,omitempty"`
	Tasks  int    `json:"tasks,omitempty"`
}

type deleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type bulkDeleteResult struct {
	Type    string `json:"type"`
	Deleted int64  `json:"deleted"`
}

type nodeCreatedResponse struct {
	Status       string `json:"status"`
	ID           string `json:"id"`
	Type         string `json:"type"`
	Summary      string `json:"summary"`
	HasEmbedding bool   `json:"hasEmbedding"`
}
