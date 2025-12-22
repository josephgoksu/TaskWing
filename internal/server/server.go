package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/agents"
	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/spf13/viper"
)

type Server struct {
	store      *memory.SQLiteStore
	cwd        string
	memoryPath string
	port       int
	server     *http.Server
}

func New(port int, cwd, memoryPath string) (*Server, error) {
	store, err := memory.NewSQLiteStore(memoryPath)
	if err != nil {
		return nil, fmt.Errorf("open memory store: %w", err)
	}

	s := &Server{
		store:      store,
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
		defer s.store.Close()

		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("API server error: %w", err)
		}
	}()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// apiScoredNode for search results
type apiScoredNode struct {
	Node  *memory.Node `json:"node"`
	Score float32      `json:"score"`
}

// handleListNodes
func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	typeFilter := r.URL.Query().Get("type")

	nodes, err := s.store.ListNodes(typeFilter)
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

	node, err := s.store.GetNode(id)
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

	apiKey := viper.GetString("llm.apiKey")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		http.Error(w, "OPENAI_API_KEY not set", http.StatusServiceUnavailable)
		return
	}

	ctx := context.Background()
	provider := llm.Provider(viper.GetString("llm.provider"))
	if provider == "" {
		provider = llm.ProviderOpenAI
	}
	llmCfg := llm.Config{
		Provider: provider,
		APIKey:   apiKey,
		BaseURL:  viper.GetString("llm.baseURL"),
	}

	nodes, err := s.store.ListNodes("")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	queryEmbedding, err := knowledge.GenerateEmbedding(ctx, req.Query, llmCfg)
	if err != nil {
		http.Error(w, "embedding generation failed", http.StatusInternalServerError)
		return
	}

	var scored []apiScoredNode
	for _, n := range nodes {
		fullNode, err := s.store.GetNode(n.ID)
		if err != nil || len(fullNode.Embedding) == 0 {
			continue
		}

		score := knowledge.CosineSimilarity(queryEmbedding, fullNode.Embedding)
		scored = append(scored, apiScoredNode{Node: fullNode, Score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > req.Limit {
		scored = scored[:req.Limit]
	}

	var answer string
	if req.Answer && len(scored) > 0 {
		answerText, err := generateSearchAnswer(ctx, req.Query, scored, llmCfg)
		if err != nil {
			fmt.Printf("[API] Answer generation failed: %v\n", err)
		} else {
			answer = answerText
		}
	}

	writeAPIJSON(w, struct {
		Query   string          `json:"query"`
		Results []apiScoredNode `json:"results"`
		Answer  string          `json:"answer,omitempty"`
	}{
		Query:   req.Query,
		Results: scored,
		Answer:  answer,
	})
}

func generateSearchAnswer(ctx context.Context, query string, scored []apiScoredNode, cfg llm.Config) (string, error) {
	var contextParts []string
	for _, s := range scored {
		nodeContext := fmt.Sprintf("[%s] %s\n%s", s.Node.Type, s.Node.Summary, s.Node.Content)
		contextParts = append(contextParts, nodeContext)
	}
	retrievedContext := strings.Join(contextParts, "\n\n---\n\n")

	prompt := fmt.Sprintf(`You are an expert on this codebase. Answer the user's question using ONLY the context below.
If the context doesn't contain enough information to answer, say so.
Be concise and direct.

## Retrieved Context:
%s

## Question:
%s

## Answer:`, retrievedContext, query)

	provider := llm.Provider(viper.GetString("llm.provider"))
	if provider == "" {
		provider = llm.ProviderOpenAI
	}
	model := viper.GetString("llm.model")
	if model == "" {
		model = llm.DefaultModelForProvider(provider)
	}

	llmCfg := llm.Config{
		Provider: provider,
		Model:    model,
		APIKey:   cfg.APIKey,
		BaseURL:  viper.GetString("llm.baseURL"),
	}

	chatModel, err := llm.NewChatModel(ctx, llmCfg)
	if err != nil {
		return "", fmt.Errorf("create chat model: %w", err)
	}

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	return resp.Content, nil
}

// handleStats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.store.ListNodes("")
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
	nodes, _ := s.store.ListNodes("")

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
	for _, a := range agents.Registry {
		result = append(result, AgentWithCount{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
			NodeCount:   counts[a.ID],
		})
	}

	writeAPIJSON(w, result)
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

	ctx := context.Background()
	// Use new internal/bootstrap runner
	err := bootstrap.RunAPI(ctx, req.ProjectPath, false, apiKey, req.Agents, s.memoryPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("bootstrap failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Reload the store
	s.store.Close()
	newStore, err := memory.NewSQLiteStore(s.memoryPath)
	if err != nil {
		http.Error(w, "failed to reload store", http.StatusInternalServerError)
		return
	}
	s.store = newStore

	nodes, _ := s.store.ListNodes("")
	stats := map[string]interface{}{
		"success":  true,
		"total":    len(nodes),
		"feature":  0,
		"decision": 0,
	}
	for _, n := range nodes {
		switch n.Type {
		case "feature":
			stats["feature"] = stats["feature"].(int) + 1
		case "decision":
			stats["decision"] = stats["decision"].(int) + 1
		}
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
	json.NewEncoder(w).Encode(data)
}
