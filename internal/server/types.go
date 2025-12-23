package server

import "github.com/josephgoksu/TaskWing/internal/knowledge"

// SearchRequest is the payload for /api/search
type SearchRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Answer bool   `json:"answer"`
}

// SearchResponse is the response for /api/search
type SearchResponse struct {
	Query   string                 `json:"query"`
	Results []knowledge.ScoredNode `json:"results"`
	Answer  string                 `json:"answer,omitempty"`
}

// BootstrapRequest is the payload for /api/bootstrap
type BootstrapRequest struct {
	ProjectPath string   `json:"projectPath"`
	Agents      []string `json:"agents"`
	Clear       bool     `json:"clear"` // If true, clear all existing data before bootstrap
}

// AgentWithCount describes an agent and its contribution count
type AgentWithCount struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	NodeCount   int    `json:"nodeCount"`
}

// StyledEdge represents an edge with pre-computed frontend styling
type StyledEdge struct {
	ID          string  `json:"id"`
	Source      string  `json:"source"` // ReactFlow field name
	Target      string  `json:"target"` // ReactFlow field name
	Relation    string  `json:"relation"`
	Confidence  float64 `json:"confidence"`  // AI confidence score (0.0-1.0)
	StrokeColor string  `json:"strokeColor"` // Pre-computed
	StrokeWidth int     `json:"strokeWidth"` // Pre-computed
	Animated    bool    `json:"animated"`    // Pre-computed
	Opacity     float64 `json:"opacity"`     // Pre-computed
}
