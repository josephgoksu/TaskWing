/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/config"
	twgrpc "github.com/josephgoksu/TaskWing/internal/grpc"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/project"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	startPort    int
	startHost    string
	startProject string
	noWatch      bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the TaskWing server and watch mode",
	Long: `Start TaskWing with all services running:
  - gRPC server for desktop app communication
  - Watch mode for continuous file analysis

Connect with the TaskWing desktop app or grpcurl.

	Examples:
	  taskwing start                    # Start everything
	  taskwing start --host 0.0.0.0     # Expose server on all interfaces
	  taskwing start --project ~/repo    # Use explicit project directory
	  taskwing start --no-watch         # Server only, no file watching
	  taskwing start --port 8080        # Use custom port`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Server flags
	startCmd.Flags().IntVarP(&startPort, "port", "p", 5001, "Server port")
	startCmd.Flags().StringVar(&startHost, "host", "127.0.0.1", "Server host bind address")
	startCmd.Flags().StringVar(&startProject, "project", "", "Project directory to use (must contain .taskwing)")
	startCmd.Flags().BoolVar(&noWatch, "no-watch", false, "Don't run watch mode (server only)")

	// LLM configuration (reuse from watch)
	startCmd.Flags().String("provider", "", "LLM provider (openai, ollama, anthropic, bedrock, gemini)")
	startCmd.Flags().String("model", "", "Model to use")
	startCmd.Flags().String("api-key", "", "LLM API key (or set provider-specific env var)")
	startCmd.Flags().String("ollama-url", "http://localhost:11434", "Ollama server URL (only used when provider=ollama)")
}

func runStart(cmd *cobra.Command, args []string) error {
	verbose := viper.GetBool("verbose")
	startHost = strings.TrimSpace(startHost)
	if startHost == "" {
		startHost = "127.0.0.1"
	}
	startHost = strings.TrimPrefix(strings.TrimSuffix(startHost, "]"), "[")

	if startProject != "" {
		projectPath, err := resolveStartProject(startProject)
		if err != nil {
			return err
		}
		if err := os.Chdir(projectPath); err != nil {
			return fmt.Errorf("change directory to project %s: %w", projectPath, err)
		}

		ctx, err := project.Detect(projectPath)
		if err != nil {
			return fmt.Errorf("detect project context from %s: %w", projectPath, err)
		}
		if err := config.SetProjectContext(ctx); err != nil {
			return fmt.Errorf("set project context for %s: %w", projectPath, err)
		}
	} else {
		// CWD fallback: auto-detect project from working directory
		// (same pattern as detectProjectRoot in config.go)
		cwd, cwdErr := os.Getwd()
		if cwdErr == nil {
			if ctx, err := project.Detect(cwd); err == nil {
				_ = config.SetProjectContext(ctx)
			}
		}
	}

	// Get working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Print banner
	fmt.Println()
	fmt.Println("🚀 TaskWing Starting...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📁 Project: %s\n", cwd)
	fmt.Printf("🌐 Server: %s\n", net.JoinHostPort(startHost, strconv.Itoa(startPort)))
	if startHost != "127.0.0.1" && startHost != "localhost" {
		fmt.Println("⚠️  Plaintext gRPC is exposed on a non-loopback host. Use only in trusted local networks.")
	}
	if !noWatch {
		fmt.Println("👁️  Watch: enabled")
	}
	fmt.Println()

	// WaitGroup to track goroutines
	var wg sync.WaitGroup

	// Error channel to capture startup errors
	errChan := make(chan error, 2)

	// Configure LLM
	llmConfig, err := getLLMConfigForRole(cmd, llm.RoleBootstrap)
	if err != nil {
		return fmt.Errorf("configure LLM: %w", err)
	}

	// Start gRPC server
	memoryPath, err := config.GetMemoryBasePath()
	if err != nil {
		return fmt.Errorf("get memory path: %w", err)
	}
	listenAddr := net.JoinHostPort(startHost, strconv.Itoa(startPort))
	srv, err := twgrpc.New(listenAddr, memoryPath, twgrpc.WithVersion(GetVersion()))
	if err != nil {
		return fmt.Errorf("failed to create gRPC server: %w", err)
	}
	if err := srv.Start(&wg, errChan); err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	// Start watch mode if enabled
	var watchAgent *impl.WatchAgent
	if !noWatch {
		watchAgent, err = startWatchMode(cwd, verbose, llmConfig, &wg, errChan)
		if err != nil {
			srv.Shutdown(context.Background())
			return fmt.Errorf("failed to start watch mode: %w", err)
		}
	}

	fmt.Println()
	fmt.Println("✅ TaskWing is running! Press Ctrl+C to stop")
	fmt.Println("   Connect with the TaskWing desktop app or grpcurl.")
	fmt.Println()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		fmt.Printf("\n\n⏹️  Received %v, shutting down...\n", sig)
	case err := <-errChan:
		fmt.Printf("\n\n❌ Error: %v\n", err)
	}

	// Stop watch agent
	if watchAgent != nil {
		fmt.Println("   Stopping watch mode...")
		watchAgent.Stop()
	}

	// Shutdown gRPC server
	fmt.Println("   Stopping gRPC server...")
	srv.Shutdown(context.Background())

	wg.Wait()
	fmt.Println("✅ TaskWing stopped")

	return nil
}

// startWatchMode starts the watch agent in a goroutine
func startWatchMode(watchPath string, verbose bool, llmConfig llm.Config, wg *sync.WaitGroup, errChan chan<- error) (*impl.WatchAgent, error) {
	// Initialize knowledge service first (needed for context injection)
	memoryPath, err := config.GetMemoryBasePath()
	if err != nil {
		return nil, fmt.Errorf("get memory path: %w", err)
	}
	repo, err := memory.NewDefaultRepository(memoryPath)
	if err != nil {
		return nil, fmt.Errorf("create memory repository: %w", err)
	}

	ks := knowledge.NewService(repo, llmConfig)

	// Create watch agent with knowledge service
	watchAgent, err := impl.NewWatchAgent(impl.WatchConfig{
		BasePath:  watchPath,
		LLMConfig: llmConfig,
		Verbose:   verbose,
		Service:   ks,
	})
	if err != nil {
		return nil, fmt.Errorf("create watch agent: %w", err)
	}

	// Set up findings handler
	watchAgent.SetFindingsHandler(func(ctx context.Context, findings []core.Finding, filePaths []string) error {
		return ks.IngestFindings(ctx, findings, filePaths, verbose)
	})

	// Start watching
	if err := watchAgent.Start(); err != nil {
		return nil, fmt.Errorf("start watch: %w", err)
	}

	return watchAgent, nil
}

func resolveStartProject(projectFlag string) (string, error) {
	trimmed := strings.TrimSpace(projectFlag)
	if trimmed == "" {
		return "", fmt.Errorf("project path is empty")
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve project path %q: %w", trimmed, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("project path does not exist: %s", absPath)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project path is not a directory: %s", absPath)
	}

	taskwingDir := filepath.Join(absPath, ".taskwing")
	taskwingInfo, err := os.Stat(taskwingDir)
	if err != nil || !taskwingInfo.IsDir() {
		return "", fmt.Errorf("invalid project %s: missing .taskwing directory (run 'taskwing bootstrap')", absPath)
	}

	return absPath, nil
}
