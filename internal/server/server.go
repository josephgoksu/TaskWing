package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/josephgoksu/TaskWing/internal/agents"
	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/viper"
)

// KnowledgeService abstracts the knowledge logic (Search, Ask)
type KnowledgeService interface {
	Search(ctx context.Context, query string, limit int) ([]knowledge.ScoredNode, error)
	Ask(ctx context.Context, query string, contextNodes []knowledge.ScoredNode) (string, error)
}

type Server struct {
	repo       *memory.Repository
	knowledge  KnowledgeService
	cwd        string
	memoryPath string
	port       int
	server     *http.Server
}

func New(port int, cwd, memoryPath string, llmCfg llm.Config) (*Server, error) {
	repo, err := memory.NewDefaultRepository(memoryPath)
	if err != nil {
		return nil, fmt.Errorf("open memory repo: %w", err)
	}

	// Use repo instead of store for consistent access
	ks := knowledge.NewService(repo, llmCfg)

	s := &Server{
		repo:       repo,
		knowledge:  ks,
		cwd:        cwd,
		memoryPath: memoryPath,
		port:       port,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/nodes", s.handleListNodes)
	mux.HandleFunc("GET /api/nodes/{id}", s.handleGetNode)
	mux.HandleFunc("POST /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/agents", s.handleAgents)
	mux.HandleFunc("GET /api/edges", s.handleEdges)
	mux.HandleFunc("POST /api/bootstrap", s.handleBootstrap)
	mux.HandleFunc("GET /api/activity", s.handleActivity)
	mux.HandleFunc("DELETE /api/activity", s.handleClearActivity)
	mux.HandleFunc("OPTIONS /api/", s.handleCORS)

	handler := corsMiddleware(mux)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	return s, nil
}

func (s *Server) Start(wg *sync.WaitGroup, errChan chan<- error) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { _ = s.repo.Close() }()

		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("API server error: %w", err)
		}
	}()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// apiScoredNode for search results

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
	var req struct {
		Query  string `json:"query"`
		Limit  int    `json:"limit"`
		Answer bool   `json:"answer"`
	}
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

	writeAPIJSON(w, struct {
		Query   string                 `json:"query"`
		Results []knowledge.ScoredNode `json:"results"`
		Answer  string                 `json:"answer,omitempty"`
	}{
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
		"version":     "0.1.0",
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

	type AgentWithCount struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		NodeCount   int    `json:"nodeCount"`
	}

	var result []AgentWithCount
	for _, a := range agents.Registry() {
		result = append(result, AgentWithCount{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			NodeCount:   counts[a.ID],
		})
	}

	writeAPIJSON(w, result)
}

// handleEdges returns all edges in the knowledge graph
func (s *Server) handleEdges(w http.ResponseWriter, r *http.Request) {
	edges, err := s.repo.GetAllNodeEdges()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeAPIJSON(w, edges)
}

// handleBootstrap
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectPath string   `json:"projectPath"`
		Agents      []string `json:"agents"`
	}
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

	apiKey := viper.GetString("llm.apiKey")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		http.Error(w, "OPENAI_API_KEY not set", http.StatusServiceUnavailable)
		return
	}

	// Create LLM Config
	llmCfg := llm.Config{
		Provider: llm.Provider(viper.GetString("llm.provider")),
		Model:    viper.GetString("llm.model"),
		APIKey:   apiKey,
		BaseURL:  viper.GetString("llm.baseURL"),
	}

	// Create Bootstrap Runner
	runner := bootstrap.NewRunner(llmCfg, req.ProjectPath)

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

	// Reuse err variable or shadow it. Using assignment since err exists.
	if err := ingestSvc.IngestFindings(r.Context(), findings, false); err != nil {
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
	activityLog := agents.NewActivityLog(s.cwd)

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
	activityLog := agents.NewActivityLog(s.cwd)
	activityLog.Clear()

	writeAPIJSON(w, map[string]any{
		"success": true,
	})
}

func (s *Server) handleCORS(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeAPIJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
