/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/watch"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	startPort    int
	noDashboard  bool
	noWatch      bool
	dashboardURL string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start TaskWing with API server, watch mode, and dashboard",
	Long: `Start TaskWing with all services running:
  - HTTP API server for dashboard communication
  - Watch mode for continuous file analysis
  - Auto-open dashboard in browser

This single command replaces running 'serve' and 'watch' separately.

Examples:
  taskwing start                    # Start everything
  taskwing start --no-dashboard     # Don't open browser
  taskwing start --no-watch         # Server only, no file watching
  taskwing start --port 8080        # Use custom port`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Server flags
	startCmd.Flags().IntVarP(&startPort, "port", "p", 5001, "API server port")
	startCmd.Flags().BoolVar(&noDashboard, "no-dashboard", false, "Don't auto-open dashboard in browser")
	startCmd.Flags().BoolVar(&noWatch, "no-watch", false, "Don't run watch mode (server only)")
	startCmd.Flags().StringVar(&dashboardURL, "dashboard-url", "", "Dashboard URL (default: https://hub.taskwing.app, use http://localhost:5173 for local dev)")

	// LLM configuration (reuse from watch)
	startCmd.Flags().String("provider", "", "LLM provider (openai, ollama, anthropic, gemini)")
	startCmd.Flags().String("model", "", "Model to use")
	startCmd.Flags().String("api-key", "", "LLM API key (or set provider-specific env var)")
	startCmd.Flags().String("ollama-url", "http://localhost:11434", "Ollama server URL")
}

func runStart(cmd *cobra.Command, args []string) error {
	verbose := viper.GetBool("verbose")

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
	fmt.Printf("🌐 API: http://localhost:%d\n", startPort)
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

	// Start HTTP API server
	memoryPath := config.GetMemoryBasePath()
	srv, err := server.New(startPort, cwd, memoryPath, GetVersion(), llmConfig)
	if err != nil {
		return fmt.Errorf("failed to create API server: %w", err)
	}
	srv.Start(&wg, errChan)

	// Start watch mode if enabled
	var watchAgent *watch.WatchAgent
	if !noWatch {
		watchAgent, err = startWatchMode(cwd, verbose, llmConfig, &wg, errChan)
		if err != nil {
			_ = srv.Shutdown(context.Background())
			return fmt.Errorf("failed to start watch mode: %w", err)
		}
	}

	// Open dashboard in browser
	if !noDashboard {
		url := dashboardURL
		if url == "" {
			url = "https://hub.taskwing.app"
		}
		// Give server a moment to start
		time.Sleep(500 * time.Millisecond)
		if err := openBrowser(url); err != nil {
			fmt.Printf("⚠️  Could not open browser: %v\n", err)
			fmt.Printf("   Open manually: %s\n", url)
		} else {
			fmt.Printf("🌐 Dashboard opened: %s\n", url)
		}
	}

	fmt.Println()
	fmt.Println("✅ TaskWing is running! Press Ctrl+C to stop")
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

	// Shutdown HTTP server with timeout
	fmt.Println("   Stopping API server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("   ⚠️  Server shutdown error: %v\n", err)
	}

	wg.Wait()
	fmt.Println("✅ TaskWing stopped")

	return nil
}

// startWatchMode starts the watch agent in a goroutine
func startWatchMode(watchPath string, verbose bool, llmConfig llm.Config, wg *sync.WaitGroup, errChan chan<- error) (*watch.WatchAgent, error) {
	// Initialize knowledge service first (needed for context injection)
	memoryPath := config.GetMemoryBasePath()
	repo, err := memory.NewDefaultRepository(memoryPath)
	if err != nil {
		return nil, fmt.Errorf("create memory repository: %w", err)
	}

	ks := knowledge.NewService(repo, llmConfig)

	// Create watch agent with knowledge service
	watchAgent, err := watch.NewWatchAgent(watch.WatchConfig{
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

// openBrowser opens the URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
