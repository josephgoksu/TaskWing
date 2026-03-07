/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/compress"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy [command] [args...]",
	Short: "Execute a command and compress its output for token savings",
	Long: `Execute a shell command, then compress its output to reduce token usage.

Examples:
  taskwing proxy git status
  taskwing proxy git log --oneline -20
  taskwing proxy go test ./...
  taskwing proxy --raw git diff          # Pass through without compression
  taskwing proxy --ultra git log         # Maximum compression`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE:               runProxy,
}

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.Flags().Bool("raw", false, "Pass through without compression (for debugging)")
	proxyCmd.Flags().Bool("ultra", false, "Use ultra-compact compression mode")
	proxyCmd.Flags().Bool("stats", false, "Print compression stats to stderr")
}

func runProxy(cmd *cobra.Command, args []string) error {
	raw, _ := cmd.Flags().GetBool("raw")
	ultra, _ := cmd.Flags().GetBool("ultra")
	showStats, _ := cmd.Flags().GetBool("stats")

	// Build the command string for pipeline selection
	cmdStr := strings.Join(args, " ")

	// Execute the command
	child := exec.Command(args[0], args[1:]...)
	child.Stdin = os.Stdin
	child.Stderr = os.Stderr

	output, err := child.Output()
	exitErr := err

	if raw {
		os.Stdout.Write(output)
		if exitErr != nil {
			if ee, ok := exitErr.(*exec.ExitError); ok {
				os.Exit(ee.ExitCode())
			}
			return exitErr
		}
		return nil
	}

	// Compress the output
	var compressed []byte
	var stats compress.Stats
	if ultra {
		compressed, stats = compress.CompressWithLevel(cmdStr, output, true)
	} else {
		compressed, stats = compress.Compress(cmdStr, output)
	}

	os.Stdout.Write(compressed)
	if len(compressed) > 0 && compressed[len(compressed)-1] != '\n' {
		fmt.Println()
	}

	// Record stats to database (best-effort, don't fail proxy on DB errors)
	inputTokens := compress.EstimateTokens(output)
	outputTokens := compress.EstimateTokens(compressed)
	savedTokens := inputTokens - outputTokens
	savedBytes := stats.InputBytes - stats.OutputBytes
	recordProxyStats(cmdStr, stats, savedBytes, inputTokens, outputTokens, savedTokens)

	if showStats {
		savedPct := stats.Saved()
		fmt.Fprintf(os.Stderr, "[compress] %s: %d→%d bytes (%.0f%% saved, ~%d tokens saved)\n",
			cmdStr, stats.InputBytes, stats.OutputBytes, savedPct, savedTokens)
	}

	// Preserve exit code from child process
	if exitErr != nil {
		if ee, ok := exitErr.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return exitErr
	}

	return nil
}

// recordProxyStats writes compression stats to the database (best-effort).
func recordProxyStats(cmdStr string, stats compress.Stats, savedBytes, inputTokens, outputTokens, savedTokens int) {
	repo, err := openRepo()
	if err != nil {
		return // No database available — skip silently
	}
	defer func() { _ = repo.Close() }()

	sessionID := ""
	if session, sessionErr := loadHookSession(); sessionErr == nil && session != nil {
		sessionID = session.SessionID
	}

	_ = RecordTokenStats(
		repo.GetDB().DB(),
		cmdStr,
		stats.InputBytes, stats.OutputBytes, savedBytes,
		stats.Ratio(), inputTokens, outputTokens, savedTokens,
		sessionID,
	)
}
