/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package agents provides callback handlers for observability and streaming
support for real-time output during agent execution.
*/
package agents

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// CallbackHandler provides observability hooks for agent execution
type CallbackHandler struct {
	out          io.Writer
	verbose      bool
	startTimes   map[string]time.Time
	mu           sync.Mutex
	onStart      func(name string)
	onEnd        func(name string, duration time.Duration)
	onError      func(name string, err error)
	onToolCall   func(name, tool, args string)
	onToolResult func(name, tool, result string)
}

// NewCallbackHandler creates a new callback handler with default output to stderr
func NewCallbackHandler() *CallbackHandler {
	return &CallbackHandler{
		out:        os.Stderr,
		verbose:    false,
		startTimes: make(map[string]time.Time),
	}
}

// SetOutput sets the output writer for callback messages
func (h *CallbackHandler) SetOutput(w io.Writer) *CallbackHandler {
	h.out = w
	return h
}

// SetVerbose enables verbose output mode
func (h *CallbackHandler) SetVerbose(v bool) *CallbackHandler {
	h.verbose = v
	return h
}

// OnStart sets a callback for when an agent starts
func (h *CallbackHandler) OnStart(fn func(name string)) *CallbackHandler {
	h.onStart = fn
	return h
}

// OnEnd sets a callback for when an agent completes
func (h *CallbackHandler) OnEnd(fn func(name string, duration time.Duration)) *CallbackHandler {
	h.onEnd = fn
	return h
}

// OnError sets a callback for when an agent encounters an error
func (h *CallbackHandler) OnError(fn func(name string, err error)) *CallbackHandler {
	h.onError = fn
	return h
}

// OnToolCall sets a callback for when a tool is invoked
func (h *CallbackHandler) OnToolCall(fn func(name, tool, args string)) *CallbackHandler {
	h.onToolCall = fn
	return h
}

// OnToolResult sets a callback for when a tool returns
func (h *CallbackHandler) OnToolResult(fn func(name, tool, result string)) *CallbackHandler {
	h.onToolResult = fn
	return h
}

// Build creates an Eino-compatible callback handler
func (h *CallbackHandler) Build() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			h.mu.Lock()
			h.startTimes[info.Name] = time.Now()
			h.mu.Unlock()

			if h.verbose {
				_, _ = fmt.Fprintf(h.out, "  ▶ %s starting (%s)\n", info.Name, info.Type)
			}

			if h.onStart != nil {
				h.onStart(info.Name)
			}

			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			h.mu.Lock()
			start := h.startTimes[info.Name]
			delete(h.startTimes, info.Name)
			h.mu.Unlock()

			duration := time.Since(start)

			if h.verbose {
				_, _ = fmt.Fprintf(h.out, "  ✓ %s completed in %.2fs\n", info.Name, duration.Seconds())
			}

			if h.onEnd != nil {
				h.onEnd(info.Name, duration)
			}

			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			_, _ = fmt.Fprintf(h.out, "  ✗ %s error: %v\n", info.Name, err)

			if h.onError != nil {
				h.onError(info.Name, err)
			}

			return ctx
		}).
		Build()
}

// StreamingOutput provides a channel-based interface for real-time agent output
type StreamingOutput struct {
	Events chan StreamEvent
	done   chan struct{}
}

// StreamEvent represents a single event in the agent execution stream
type StreamEvent struct {
	Type      StreamEventType
	Timestamp time.Time
	Agent     string
	Content   string
	Metadata  map[string]any
}

// StreamEventType identifies the type of streaming event
type StreamEventType string

const (
	EventAgentStart StreamEventType = "agent_start"
	EventAgentEnd   StreamEventType = "agent_end"
	EventAgentError StreamEventType = "agent_error"
	EventToolCall   StreamEventType = "tool_call"
	EventToolResult StreamEventType = "tool_result"
	EventLLMChunk   StreamEventType = "llm_chunk"
	EventFinding    StreamEventType = "finding"
	EventSynthesis  StreamEventType = "synthesis"
)

// NewStreamingOutput creates a new streaming output channel
func NewStreamingOutput(bufferSize int) *StreamingOutput {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &StreamingOutput{
		Events: make(chan StreamEvent, bufferSize),
		done:   make(chan struct{}),
	}
}

// Emit sends an event to the stream
func (s *StreamingOutput) Emit(eventType StreamEventType, agent, content string, metadata map[string]any) {
	select {
	case s.Events <- StreamEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		Agent:     agent,
		Content:   content,
		Metadata:  metadata,
	}:
	case <-s.done:
		// Stream closed
	default:
		// Buffer full, drop event
	}
}

// Close closes the streaming output
func (s *StreamingOutput) Close() {
	close(s.done)
	close(s.Events)
}

// CreateStreamingCallbackHandler creates a callback handler that emits to a stream
func CreateStreamingCallbackHandler(stream *StreamingOutput) *CallbackHandler {
	return NewCallbackHandler().
		OnStart(func(name string) {
			stream.Emit(EventAgentStart, name, fmt.Sprintf("Agent %s starting", name), nil)
		}).
		OnEnd(func(name string, duration time.Duration) {
			stream.Emit(EventAgentEnd, name, fmt.Sprintf("Agent %s completed in %.2fs", name, duration.Seconds()), map[string]any{
				"duration_ms": duration.Milliseconds(),
			})
		}).
		OnError(func(name string, err error) {
			stream.Emit(EventAgentError, name, err.Error(), nil)
		}).
		OnToolCall(func(agent, tool, args string) {
			stream.Emit(EventToolCall, agent, fmt.Sprintf("%s(%s)", tool, utils.Truncate(args, 50)), map[string]any{
				"tool": tool,
				"args": args,
			})
		}).
		OnToolResult(func(agent, tool, result string) {
			stream.Emit(EventToolResult, agent, utils.Truncate(result, 100), map[string]any{
				"tool": tool,
			})
		})
}

// ProgressReporter provides a simple progress reporting interface
type ProgressReporter struct {
	out         io.Writer
	currentStep string
	stepCount   int
	totalSteps  int
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter(out io.Writer) *ProgressReporter {
	if out == nil {
		out = os.Stderr
	}
	return &ProgressReporter{out: out}
}

// SetTotal sets the total number of steps
func (p *ProgressReporter) SetTotal(n int) {
	p.totalSteps = n
}

// Step reports progress on a step
func (p *ProgressReporter) Step(name string) {
	p.stepCount++
	p.currentStep = name

	if p.totalSteps > 0 {
		_, _ = fmt.Fprintf(p.out, "  [%d/%d] %s\n", p.stepCount, p.totalSteps, name)
	} else {
		_, _ = fmt.Fprintf(p.out, "  → %s\n", name)
	}
}

// Complete marks the overall operation as complete
func (p *ProgressReporter) Complete(message string) {
	_, _ = fmt.Fprintf(p.out, "  ✓ %s\n", message)
}

// Error reports an error
func (p *ProgressReporter) Error(message string) {
	_, _ = fmt.Fprintf(p.out, "  ✗ %s\n", message)
}
