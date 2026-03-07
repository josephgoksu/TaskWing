package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// runStreaming runs Codex CLI with `exec --json` and parses the JSONL event stream,
// emitting ProgressEvents via req.OnProgress. The last assistant text content is
// accumulated and returned as InvokeResult.RawOutput.
func (r *codexRunner) runStreaming(ctx context.Context, req InvokeRequest, args []string) (*InvokeResult, error) {
	timeout := req.effectiveTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.binaryPath, args...)
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codex stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("codex start: %w", err)
	}

	// Parse JSONL stream
	var lastAssistantText string
	lastSummary := ""

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event codexStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "item.started":
			if event.Item.Role == "assistant" {
				emitDeduped(req.OnProgress, &lastSummary, ProgressEvent{
					Type:    ProgressThinking,
					Summary: "Thinking...",
				})
			}
			if event.Item.Type == "function_call" {
				summary := event.Item.Name
				if summary == "" {
					summary = "tool"
				}
				emitDeduped(req.OnProgress, &lastSummary, ProgressEvent{
					Type:    ProgressToolUse,
					Summary: summary,
				})
			}

		case "item.completed":
			if event.Item.Type == "function_call_output" {
				emitDeduped(req.OnProgress, &lastSummary, ProgressEvent{
					Type:    ProgressToolResult,
					Summary: "done",
				})
			}
			// Capture last assistant text
			if event.Item.Role == "assistant" {
				for _, c := range event.Item.Content {
					if c.Type == "text" && c.Text != "" {
						lastAssistantText = c.Text
						emitDeduped(req.OnProgress, &lastSummary, ProgressEvent{
							Type:    ProgressText,
							Summary: truncateStream(c.Text, 80),
						})
					}
				}
			}

		case "turn.completed":
			// Extract final assistant text from the completed turn
			if event.Turn.Role == "assistant" {
				for _, c := range event.Turn.Content {
					if c.Type == "text" && c.Text != "" {
						lastAssistantText = c.Text
					}
				}
			}
		}
	}

	waitErr := cmd.Wait()

	result := &InvokeResult{
		RawOutput: lastAssistantText,
		Stderr:    stderr.String(),
		CLIType:   CLICodex,
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if waitErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("codex invocation timed out after %v", timeout)
		}
		return result, fmt.Errorf("codex invocation failed (exit %d): %w\nstderr: %s",
			result.ExitCode, waitErr, truncate(result.Stderr, 500))
	}

	return result, nil
}

// emitDeduped sends a progress event only if the summary differs from the last one.
func emitDeduped(cb ProgressCallback, lastSummary *string, ev ProgressEvent) {
	if cb == nil {
		return
	}
	if ev.Summary == *lastSummary {
		return
	}
	*lastSummary = ev.Summary
	cb(ev)
}

// truncateStream shortens a string to maxLen for display, trimming whitespace.
func truncateStream(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// codexStreamEvent represents a single JSONL event from Codex CLI's exec --json output.
type codexStreamEvent struct {
	Type string          `json:"type"`
	Item codexStreamItem `json:"item"`
	Turn codexStreamItem `json:"turn"`
}

type codexStreamItem struct {
	Role    string               `json:"role,omitempty"`
	Type    string               `json:"type,omitempty"`
	Name    string               `json:"name,omitempty"` // Function name for function_call
	Content []codexStreamContent `json:"content,omitempty"`
}

type codexStreamContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
