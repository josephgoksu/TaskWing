package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
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
	version    string
	origins    map[string]struct{}
	server     *http.Server
}

func New(host string, port int, cwd, memoryPath, version string, allowedOrigins []string, llmCfg llm.Config) (*Server, error) {
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
		version:    version,
		origins:    make(map[string]struct{}, len(allowedOrigins)),
	}
	for _, origin := range allowedOrigins {
		if origin == "" {
			continue
		}
		s.origins[origin] = struct{}{}
	}

	handler := s.registerRoutes()

	s.server = &http.Server{
		Addr:    net.JoinHostPort(host, strconv.Itoa(port)),
		Handler: handler,
	}

	return s, nil
}

func (s *Server) Start(wg *sync.WaitGroup, errChan chan<- error) error {
	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("bind API server %s: %w", s.server.Addr, err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { _ = s.repo.Close() }()

		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("API server error: %w", err)
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
