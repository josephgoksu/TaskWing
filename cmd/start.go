/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/server"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

var (
	startPort    int
	startHost    string
	noDashboard  bool
	dashboardURL string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start TaskWing with API server and dashboard",
	Long: `Start TaskWing with all services running:
  - HTTP API server for dashboard communication
  - Auto-open dashboard in browser

	Examples:
	  taskwing start                    # Start everything
	  taskwing start --host 0.0.0.0     # Expose API on all interfaces
	  taskwing start --no-dashboard     # Don't open browser
	  taskwing start --port 8080        # Use custom port`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Server flags
	startCmd.Flags().IntVarP(&startPort, "port", "p", 5001, "API server port")
	startCmd.Flags().StringVar(&startHost, "host", "127.0.0.1", "API server host bind address")
	startCmd.Flags().BoolVar(&noDashboard, "no-dashboard", false, "Don't auto-open dashboard in browser")
	startCmd.Flags().StringVar(&dashboardURL, "dashboard-url", "", "Dashboard URL (default: https://hub.taskwing.app, use http://localhost:5173 for local dev)")

	// LLM configuration
	startCmd.Flags().String("provider", "", "LLM provider (openai, ollama, anthropic, bedrock, gemini)")
	startCmd.Flags().String("model", "", "Model to use")
	startCmd.Flags().String("api-key", "", "LLM API key (or set provider-specific env var)")
	startCmd.Flags().String("ollama-url", "http://localhost:11434", "Ollama server URL (only used when provider=ollama)")
}

func runStart(cmd *cobra.Command, args []string) error {
	startHost = strings.TrimSpace(startHost)
	if startHost == "" {
		startHost = "127.0.0.1"
	}
	startHost = strings.TrimPrefix(strings.TrimSuffix(startHost, "]"), "[")

	// Get working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Print banner
	if !isQuiet() {
		fmt.Println()
		fmt.Printf("%s TaskWing Starting...\n", ui.IconRocket)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("%s Project: %s\n", ui.IconFolder, cwd)
		fmt.Printf("%s API: %s\n", ui.IconGlobe, apiURL(startHost, startPort))
		fmt.Println()
	}

	// WaitGroup to track goroutines
	var wg sync.WaitGroup

	// Error channel to capture startup errors
	errChan := make(chan error, 2)

	// Configure LLM
	llmConfig, err := getLLMConfigForRole(cmd, llm.RoleBootstrap)
	if err != nil {
		return fmt.Errorf("configure LLM: %w", err)
	}

	resolvedDashboardURL := dashboardURL
	if resolvedDashboardURL == "" {
		resolvedDashboardURL = "https://hub.taskwing.app"
	}

	// Start HTTP API server
	memoryPath, err := config.GetMemoryBasePath()
	if err != nil {
		return fmt.Errorf("get memory path: %w", err)
	}
	srv, err := server.New(startHost, startPort, cwd, memoryPath, GetVersion(), buildAllowedOrigins(resolvedDashboardURL), llmConfig)
	if err != nil {
		return fmt.Errorf("failed to create API server: %w", err)
	}
	if err := srv.Start(&wg, errChan); err != nil {
		return fmt.Errorf("failed to start API server: %w", err)
	}

	// Watch mode removed (WatchAgent deleted)

	// Open dashboard in browser
	if !noDashboard {
		// Give server a moment to start
		time.Sleep(500 * time.Millisecond)
		if err := openBrowser(resolvedDashboardURL); err != nil {
			if !isQuiet() {
				ui.PrintWarning(fmt.Sprintf("Could not open browser: %v", err))
				fmt.Printf("   Open manually: %s\n", resolvedDashboardURL)
			}
		} else if !isQuiet() {
			fmt.Printf("%s Dashboard opened: %s\n", ui.IconGlobe, resolvedDashboardURL)
		}
	}

	if !isQuiet() {
		fmt.Println()
		ui.PrintSuccess("TaskWing is running! Press Ctrl+C to stop")
		fmt.Println()
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		fmt.Printf("\n\n%s  Received %v, shutting down...\n", ui.IconStop, sig)
	case err := <-errChan:
		fmt.Print("\n\n")
		ui.PrintError(fmt.Sprintf("Error: %v", err))
	}

	// Shutdown HTTP server with timeout
	fmt.Println("   Stopping API server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		ui.PrintWarning(fmt.Sprintf("Server shutdown error: %v", err))
	}

	wg.Wait()
	ui.PrintSuccess("TaskWing stopped")

	return nil
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

func buildAllowedOrigins(dashboardURL string) []string {
	origins := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	addOrigin := func(origin string) {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			return
		}
		if _, ok := seen[origin]; ok {
			return
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}

	addOrigin("http://localhost:5173")
	addOrigin("http://127.0.0.1:5173")
	addOrigin("https://hub.taskwing.app")

	if parsed, err := url.Parse(dashboardURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		addOrigin(parsed.Scheme + "://" + parsed.Host)
	}

	return origins
}

func apiURL(host string, port int) string {
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}
