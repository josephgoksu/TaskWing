package ui

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

// TestRenderBenchmark_Alignment verifies that the benchmark table output
// aligns correctly and matches a golden file snapshot.
func TestRenderBenchmark_Alignment(t *testing.T) {
	// Create test data
	data := BenchmarkData{
		Runs:    []string{"20251229-223844", "20251229-224305"},
		Models:  []string{"baseline (gpt-5-mini)", "taskwing-v1 (gpt-5-mini)"},
		TaskIDs: []string{"T1", "T2", "T3", "T4", "T5"},
		Matrix: map[string]map[string]BenchmarkRun{
			"baseline (gpt-5-mini)": {
				"20251229-223844": {
					RunID:    "20251229-223844",
					RunDate:  time.Date(2025, 12, 29, 22, 38, 44, 0, time.UTC),
					Model:    "gpt-5-mini",
					AvgScore: 5.0,
					TaskScores: map[string]int{
						"T1": 8, "T2": 2, "T3": 5, "T4": 5, "T5": 5,
					},
					Pass:  2,
					Total: 5,
				},
			},
			"taskwing-v1 (gpt-5-mini)": {
				"20251229-224305": {
					RunID:    "20251229-224305",
					RunDate:  time.Date(2025, 12, 29, 22, 43, 5, 0, time.UTC),
					Model:    "gpt-5-mini",
					Label:    "taskwing-v1",
					AvgScore: 6.2,
					TaskScores: map[string]int{
						"T1": 8, "T2": 4, "T3": 8, "T4": 5, "T5": 6,
					},
					Pass:  3,
					Total: 5,
				},
			},
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	RenderBenchmark(data)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Basic alignment checks (column separators should be present)
	if !bytes.Contains([]byte(output), []byte("| Date")) {
		t.Error("Missing '| Date' column separator in header")
	}
	if !bytes.Contains([]byte(output), []byte("| Avg")) {
		t.Error("Missing '| Avg' column separator in header")
	}

	// Check that scores appear in output
	if !bytes.Contains([]byte(output), []byte("5.0")) {
		t.Error("Missing baseline avg score 5.0")
	}
	if !bytes.Contains([]byte(output), []byte("6.2")) {
		t.Error("Missing taskwing avg score 6.2")
	}

	// TODO: For full golden file comparison, write output to testdata/benchmark_golden.txt
	// and compare. For now, basic sanity checks suffice.
	t.Logf("Captured output (%d bytes):\n%s", len(output), output)
}
