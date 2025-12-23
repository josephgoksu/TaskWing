package server

import "net/http"

// registerRoutes sets up all API endpoints
func (s *Server) registerRoutes() http.Handler {
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

	return corsMiddleware(mux)
}
