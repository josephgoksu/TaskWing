package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/watch"
	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/task"
)

// handleListNodes
func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	typeFilter := r.URL.Query().Get("type")

	nodes, err := s.repo.ListNodes(typeFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeAPIJSON(w, nodes)
}

// handleGetNode
func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	node, err := s.repo.GetNode(id)
	if err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	writeAPIJSON(w, node)
}

// handleSearch
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}
	if req.Limit == 0 {
		req.Limit = 5
	}

	ctx := context.Background()

	// Use KnowledgeService
	scored, err := s.knowledge.Search(ctx, req.Query, req.Limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var answer string
	if req.Answer && len(scored) > 0 {
		answerText, err := s.knowledge.Ask(ctx, req.Query, scored)
		if err != nil {
			fmt.Printf("[API] Answer generation failed: %v\n", err)
		} else {
			answer = answerText
		}
	}

	writeAPIJSON(w, SearchResponse{
		Query:   req.Query,
		Results: scored,
		Answer:  answer,
	})
}

// handleStats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	// Use DB directly for list (read-only)
	nodes, err := s.repo.ListNodes("")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats := map[string]int{
		"total":    len(nodes),
		"feature":  0,
		"decision": 0,
		"pattern":  0,
	}

	for _, n := range nodes {
		switch n.Type {
		case "feature":
			stats["feature"]++
		case "decision":
			stats["decision"]++
		case "pattern":
			stats["pattern"]++
		}
	}

	writeAPIJSON(w, stats)
}

// handleInfo
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeAPIJSON(w, map[string]string{
		"projectPath": s.cwd,
		"version":     s.version,
	})
}

// handleAgents
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	nodes, _ := s.repo.ListNodes("")

	counts := make(map[string]int)
	for _, n := range nodes {
		if n.SourceAgent != "" {
			counts[n.SourceAgent]++
		}
	}

	var result []AgentWithCount
	for _, a := range core.Registry() {
		result = append(result, AgentWithCount{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			NodeCount:   counts[a.ID],
		})
	}

	writeAPIJSON(w, result)
}

// handleEdges returns all edges in the knowledge graph with pre-computed styling
// Frontend should just render these without additional computation
func (s *Server) handleEdges(w http.ResponseWriter, r *http.Request) {
	edges, err := s.repo.GetAllNodeEdges()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	styled := make([]StyledEdge, len(edges))
	for i, e := range edges {
		isSemantic := e.Relation == "semantically_similar"
		strokeColor := "#10b981" // emerald for relates_to
		if isSemantic {
			strokeColor = "#f59e0b" // amber for semantic
		}
		strokeWidth := 2
		if isSemantic {
			strokeWidth = int(e.Confidence * 3)
			if strokeWidth < 1 {
				strokeWidth = 1
			}
		}
		opacity := e.Confidence
		if opacity < 0.4 {
			opacity = 0.4
		}

		styled[i] = StyledEdge{
			ID:          fmt.Sprintf("e-%d", e.ID),
			Source:      e.FromNode,
			Target:      e.ToNode,
			Relation:    e.Relation,
			Confidence:  e.Confidence,
			StrokeColor: strokeColor,
			StrokeWidth: strokeWidth,
			Animated:    isSemantic,
			Opacity:     opacity,
		}

	}

	writeAPIJSON(w, styled)
}

// handleBootstrap
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	var req BootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ProjectPath = s.cwd
	}
	if req.ProjectPath == "" {
		req.ProjectPath = s.cwd
	}

	if _, err := os.Stat(req.ProjectPath); os.IsNotExist(err) {
		http.Error(w, "project path does not exist", http.StatusBadRequest)
		return
	}

	// Load LLM Config (Centralized)
	llmCfg, err := config.LoadLLMConfig()
	if err != nil {
		fmt.Printf("[API] Failed to load LLM config: %v\n", err)
		http.Error(w, fmt.Sprintf("llm config error: %v", err), http.StatusInternalServerError)
		return
	}

	// Create Bootstrap Runner
	runner := bootstrap.NewRunner(llmCfg, req.ProjectPath)
	defer runner.Close()

	// Run Agents
	findings, err := runner.Run(r.Context(), req.ProjectPath)
	if err != nil {
		fmt.Printf("[API] Bootstrap failed: %v\n", err)
		http.Error(w, fmt.Sprintf("bootstrap failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Ingest
	ingestSvc, ok := s.knowledge.(*knowledge.Service)
	if !ok {
		// Should not happen if initialized correctly
		http.Error(w, "knowledge service not available", http.StatusInternalServerError)
		return
	}

	// Clear all existing data if requested (clean-slate re-bootstrap)
	if req.Clear {
		if err := s.repo.ClearAllKnowledge(); err != nil {
			fmt.Printf("[API] Clear failed: %v\n", err)
			// Continue anyway - this is non-fatal
		}
	}

	// Reuse err variable or shadow it. Using assignment since err exists.
	if err := ingestSvc.IngestFindings(r.Context(), findings, nil, false); err != nil {
		http.Error(w, fmt.Sprintf("ingestion failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Response stats
	stats := map[string]interface{}{
		"success":  true,
		"findings": len(findings),
	}
	writeAPIJSON(w, stats)
}

// handleActivity
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	activityLog := watch.NewActivityLog(s.cwd)

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	entries := activityLog.GetRecent(limit)
	summary := activityLog.Summary()

	writeAPIJSON(w, map[string]any{
		"entries": entries,
		"summary": summary,
	})
}

// handleClearActivity
func (s *Server) handleClearActivity(w http.ResponseWriter, r *http.Request) {
	activityLog := watch.NewActivityLog(s.cwd)
	activityLog.Clear()

	writeAPIJSON(w, map[string]any{
		"success": true,
	})
}

// handleListPlans
func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := s.repo.ListPlans()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeAPIJSON(w, plans)
}

// handleCreatePlan
func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var p task.Plan
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.repo.CreatePlan(&p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeAPIJSON(w, p)
}

// handleGetPlan
func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	p, err := s.repo.GetPlan(id)
	if err != nil {
		http.Error(w, "plan not found", http.StatusNotFound)
		return
	}

	writeAPIJSON(w, p)
}

// handlePromoteToTask
func (s *Server) handlePromoteToTask(w http.ResponseWriter, r *http.Request) {
	var req PromoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	activityLog := watch.NewActivityLog(s.cwd)
	entries := activityLog.GetRecent(500) // Large enough to find the id

	var finding *watch.ActivityEntry
	for i := range entries {
		if entries[i].ID == req.FindingID {
			finding = &entries[i]
			break
		}
	}

	if finding == nil {
		http.Error(w, "finding not found in activity log", http.StatusNotFound)
		return
	}

	// Create a task from the finding
	planID := req.PlanID
	if planID == "" {
		newPlan := &task.Plan{
			Goal: fmt.Sprintf("Address finding: %s", finding.Message),
		}
		if err := s.repo.CreatePlan(newPlan); err != nil {
			http.Error(w, fmt.Sprintf("create plan failed: %v", err), http.StatusInternalServerError)
			return
		}
		planID = newPlan.ID
	}

	newTask := &task.Task{
		PlanID:      planID,
		Title:       finding.Message,
		Description: fmt.Sprintf("Automatically promoted from activity finding. Original agent: %s", finding.Agent),
		Status:      task.StatusPending,
		Priority:    50,
	}
	// Populate AI integration fields (scope, keywords, suggested_recall_queries)
	newTask.EnrichAIFields()

	if err := s.repo.CreateTask(newTask); err != nil {
		http.Error(w, fmt.Sprintf("create task failed: %v", err), http.StatusInternalServerError)
		return
	}

	writeAPIJSON(w, newTask)
}

func writeAPIJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
