/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var gainCmd = &cobra.Command{
	Use:   "gain",
	Short: "Show token savings from output compression",
	Long: `Display a dashboard of token savings achieved by the compression proxy.

Shows total commands compressed, bytes/tokens saved, efficiency percentage,
and top commands by savings.

Examples:
  taskwing gain               # Dashboard summary
  taskwing gain --history     # Per-command breakdown
  taskwing gain --format json # Machine-readable output`,
	RunE: runGain,
}

func init() {
	rootCmd.AddCommand(gainCmd)
	gainCmd.Flags().Bool("history", false, "Show per-command history")
	gainCmd.Flags().String("format", "text", "Output format: text or json")
}

type gainSummary struct {
	TotalCommands    int     `json:"total_commands"`
	TotalInputBytes  int64   `json:"total_input_bytes"`
	TotalOutputBytes int64   `json:"total_output_bytes"`
	TotalSavedBytes  int64   `json:"total_saved_bytes"`
	TotalSavedTokens int64   `json:"total_saved_tokens"`
	AvgCompression   float64 `json:"avg_compression_pct"`
	TopCommands      []gainCommandStat `json:"top_commands"`
}

type gainCommandStat struct {
	Command     string  `json:"command"`
	Count       int     `json:"count"`
	SavedBytes  int64   `json:"saved_bytes"`
	SavedTokens int64   `json:"saved_tokens"`
	AvgRatio    float64 `json:"avg_ratio_pct"`
}

type gainHistoryEntry struct {
	Command          string  `json:"command"`
	InputBytes       int     `json:"input_bytes"`
	OutputBytes      int     `json:"output_bytes"`
	SavedTokens      int     `json:"saved_tokens"`
	CompressionRatio float64 `json:"compression_ratio"`
	CreatedAt        string  `json:"created_at"`
}

func runGain(cmd *cobra.Command, _ []string) error {
	history, _ := cmd.Flags().GetBool("history")
	format, _ := cmd.Flags().GetString("format")

	repo, err := openRepo()
	if err != nil {
		if isMissingProjectMemoryError(err) {
			fmt.Println("No project memory found. Run 'taskwing init' first.")
			return nil
		}
		return err
	}
	defer func() { _ = repo.Close() }()

	db := repo.GetDB().DB()

	if history {
		return showGainHistory(db, format)
	}
	return showGainSummary(db, format)
}

func showGainSummary(db *sql.DB, format string) error {
	var summary gainSummary

	// Get totals
	err := db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(input_bytes), 0),
			COALESCE(SUM(output_bytes), 0),
			COALESCE(SUM(saved_bytes), 0),
			COALESCE(SUM(saved_tokens), 0)
		FROM token_stats
	`).Scan(
		&summary.TotalCommands,
		&summary.TotalInputBytes,
		&summary.TotalOutputBytes,
		&summary.TotalSavedBytes,
		&summary.TotalSavedTokens,
	)
	if err != nil {
		return fmt.Errorf("query token stats: %w", err)
	}

	if summary.TotalInputBytes > 0 {
		summary.AvgCompression = float64(summary.TotalSavedBytes) / float64(summary.TotalInputBytes) * 100
	}

	// Get top commands by savings
	rows, err := db.Query(`
		SELECT
			command,
			COUNT(*) as cnt,
			SUM(saved_bytes) as total_saved,
			COALESCE(SUM(saved_tokens), 0) as total_tokens_saved,
			AVG(CASE WHEN input_bytes > 0 THEN (1.0 - CAST(output_bytes AS REAL) / input_bytes) * 100 ELSE 0 END) as avg_ratio
		FROM token_stats
		GROUP BY command
		ORDER BY total_saved DESC
		LIMIT 10
	`)
	if err != nil {
		return fmt.Errorf("query top commands: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cs gainCommandStat
		if err := rows.Scan(&cs.Command, &cs.Count, &cs.SavedBytes, &cs.SavedTokens, &cs.AvgRatio); err != nil {
			continue
		}
		summary.TopCommands = append(summary.TopCommands, cs)
	}

	if format == "json" {
		return printJSON(summary)
	}

	// Text output
	if summary.TotalCommands == 0 {
		fmt.Println("No compression data yet. Use 'taskwing proxy <cmd>' or enable the PreToolUse hook.")
		return nil
	}

	fmt.Println()
	fmt.Println("Token Savings Dashboard")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Commands compressed:  %d\n", summary.TotalCommands)
	fmt.Printf("  Bytes saved:          %s → %s (%.0f%% reduction)\n",
		formatBytes(summary.TotalInputBytes), formatBytes(summary.TotalOutputBytes), summary.AvgCompression)
	fmt.Printf("  Tokens saved:         ~%d\n", summary.TotalSavedTokens)
	fmt.Println()

	if len(summary.TopCommands) > 0 {
		fmt.Println("Top Commands by Savings")
		fmt.Println("────────────────────────────────────────")
		for _, cs := range summary.TopCommands {
			fmt.Printf("  %-30s %3dx  %6s saved  (%.0f%%)\n",
				truncateStr(cs.Command, 30), cs.Count, formatBytes(cs.SavedBytes), cs.AvgRatio)
		}
		fmt.Println()
	}

	return nil
}

func showGainHistory(db *sql.DB, format string) error {
	rows, err := db.Query(`
		SELECT command, input_bytes, output_bytes, COALESCE(saved_tokens, 0), COALESCE(compression_ratio, 0), created_at
		FROM token_stats
		ORDER BY created_at DESC
		LIMIT 50
	`)
	if err != nil {
		return fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	var entries []gainHistoryEntry
	for rows.Next() {
		var e gainHistoryEntry
		if err := rows.Scan(&e.Command, &e.InputBytes, &e.OutputBytes, &e.SavedTokens, &e.CompressionRatio, &e.CreatedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}

	if format == "json" {
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if len(entries) == 0 {
		fmt.Println("No compression history yet.")
		return nil
	}

	fmt.Println()
	fmt.Println("Recent Compression History")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  %-30s  %8s → %8s  %6s  %s\n", "Command", "Input", "Output", "Saved", "When")
	fmt.Println("  ────────────────────────────────────────────────────────────────")
	for _, e := range entries {
		saved := e.InputBytes - e.OutputBytes
		pct := float64(0)
		if e.InputBytes > 0 {
			pct = float64(saved) / float64(e.InputBytes) * 100
		}
		fmt.Printf("  %-30s  %8s → %8s  %5.0f%%  %s\n",
			truncateStr(e.Command, 30),
			formatBytes(int64(e.InputBytes)),
			formatBytes(int64(e.OutputBytes)),
			pct,
			e.CreatedAt,
		)
	}
	fmt.Println()
	return nil
}

func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// RecordTokenStats inserts a compression stats row into the database.
func RecordTokenStats(db *sql.DB, command string, inputBytes, outputBytes, savedBytes int, ratio float64, inputTokens, outputTokens, savedTokens int, sessionID string) error {
	_, err := db.Exec(`
		INSERT INTO token_stats (command, input_bytes, output_bytes, saved_bytes, compression_ratio, input_tokens, output_tokens, saved_tokens, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, command, inputBytes, outputBytes, savedBytes, ratio, inputTokens, outputTokens, savedTokens, sessionID)
	return err
}
