// Package grpc provides the gRPC server that exposes TaskWing business logic
// to the macOS desktop app and other gRPC clients. Each service is a thin
// adapter over internal/app/ — no business logic lives here.
//
// Repositories must be instantiated with memory.NewDefaultRepository when
// creating the app context (constraint from internal/memory).
package grpc

import (
	"context"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/memory"

	pb "github.com/josephgoksu/TaskWing/gen/go/taskwing/v1"
)

// Server wraps a gRPC server with TaskWing service registrations.
type Server struct {
	grpcServer *grpc.Server
	repo       *memory.Repository
	appCtx     *app.Context
	addr       string
	version    string
}

// Option configures the gRPC server.
type Option func(*Server)

// WithVersion sets the server version string.
func WithVersion(v string) Option {
	return func(s *Server) { s.version = v }
}

// New creates a gRPC server with all TaskWing services registered.
// The listenAddr should be "host:port" (e.g., "127.0.0.1:5001").
//
// NOTE: Uses memory.NewDefaultRepository to open the SQLite database
// (constraint: always use NewDefaultRepository for repo instantiation).
func New(listenAddr, memoryPath string, opts ...Option) (*Server, error) {
	repo, err := memory.NewDefaultRepository(memoryPath)
	if err != nil {
		return nil, fmt.Errorf("open memory repo: %w", err)
	}

	appCtx := app.NewContext(repo)

	s := &Server{
		repo:   repo,
		appCtx: appCtx,
		addr:   listenAddr,
	}
	for _, o := range opts {
		o(s)
	}

	s.grpcServer = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recoveryUnaryInterceptor(),
			loggingUnaryInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			recoveryStreamInterceptor(),
			loggingStreamInterceptor(),
		),
	)

	// Register TaskWing services — each is a thin adapter over internal/app/.
	pb.RegisterPlanServiceServer(s.grpcServer, NewPlanService(appCtx))
	pb.RegisterTaskServiceServer(s.grpcServer, NewTaskService(appCtx))
	pb.RegisterKnowledgeServiceServer(s.grpcServer, NewKnowledgeService(appCtx))
	pb.RegisterCodeIntelServiceServer(s.grpcServer, NewCodeIntelService(appCtx))
	pb.RegisterServerServiceServer(s.grpcServer, NewServerService(appCtx, s.version))

	// gRPC health check protocol (used by macOS app to verify connectivity).
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(s.grpcServer, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Reflection for grpcurl debugging.
	reflection.Register(s.grpcServer)

	return s, nil
}

// Start begins serving gRPC requests. It blocks until the server stops.
// It blocks in a goroutine, reporting errors via errChan.
func (s *Server) Start(wg *sync.WaitGroup, errChan chan<- error) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("bind gRPC server %s: %w", s.addr, err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { _ = s.repo.Close() }()

		if err := s.grpcServer.Serve(listener); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()
	return nil
}

// Shutdown gracefully stops the gRPC server.
func (s *Server) Shutdown(_ context.Context) {
	s.grpcServer.GracefulStop()
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return s.addr
}
