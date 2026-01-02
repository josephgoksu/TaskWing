/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package core provides callback handlers for observability and streaming
support for real-time output during agent execution.
*/
package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// CallbackHandler provides observability hooks for agent execution
type CallbackHandler struct {
	out             io.Writer
	verbose         bool
	startTimes      map[string]time.Time
	mu              sync.Mutex
	onStart         func(name string)
	onEnd           func(name string, duration time.Duration)
	onError         func(name string, err error)
	onToolCall      func(name, tool, args string)
	onToolResult    func(name, tool, result string)
	onNodeStart     func(name, nodeType string)
	onNodeEnd       func(name, nodeType string)
	onNodeStartMeta func(name, nodeType string, meta map[string]any)
	onNodeEndMeta   func(name, nodeType string, meta map[string]any)
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

// OnNodeStart sets a callback for when an Eino node starts
func (h *CallbackHandler) OnNodeStart(fn func(name, nodeType string)) *CallbackHandler {
	h.onNodeStart = fn
	return h
}

// OnNodeStartMeta sets a callback for when an Eino node starts with metadata.
func (h *CallbackHandler) OnNodeStartMeta(fn func(name, nodeType string, meta map[string]any)) *CallbackHandler {
	h.onNodeStartMeta = fn
	return h
}

// OnNodeEnd sets a callback for when an Eino node ends
func (h *CallbackHandler) OnNodeEnd(fn func(name, nodeType string)) *CallbackHandler {
	h.onNodeEnd = fn
	return h
}

// OnNodeEndMeta sets a callback for when an Eino node ends with metadata.
func (h *CallbackHandler) OnNodeEndMeta(fn func(name, nodeType string, meta map[string]any)) *CallbackHandler {
	h.onNodeEndMeta = fn
	return h
}

// Build creates an Eino-compatible callback handler
func (h *CallbackHandler) Build() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			h.mu.Lock()
			h.startTimes[info.Name] = time.Now()
			h.mu.Unlock()

			if h.onStart != nil && (string(info.Component) == "Graph" || string(info.Component) == "Chain") {
				h.onStart(info.Name)
			}

			// If this is a node (component), trigger node start
			if string(info.Component) != "Chain" && string(info.Component) != "Graph" {
				if h.onNodeStartMeta != nil {
					meta := map[string]any{
						"component": string(info.Component),
					}
					if info.Component == components.ComponentOfChatModel {
						mInput := model.ConvCallbackInput(input)
						if mInput != nil && mInput.Config != nil {
							meta["model"] = mInput.Config.Model
							meta["max_tokens"] = mInput.Config.MaxTokens
							meta["temperature"] = mInput.Config.Temperature
						}
						if mInput != nil {
							meta["messages"] = len(mInput.Messages)
							meta["tools"] = len(mInput.Tools)
						}
					}
					h.onNodeStartMeta(info.Name, string(info.Component), meta)
				} else if h.onNodeStart != nil {
					h.onNodeStart(info.Name, string(info.Component))
				}
			}

			if h.onToolCall != nil && info.Component == components.ComponentOfTool {
				if toolInput := tool.ConvCallbackInput(input); toolInput != nil {
					h.onToolCall(info.Name, info.Name, toolInput.ArgumentsInJSON)
				}
			}

			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			h.mu.Lock()
			start, exists := h.startTimes[info.Name]
			if exists {
				delete(h.startTimes, info.Name)
			}
			h.mu.Unlock()

			// Guard against missing start time (can happen with parallel chain invokes)
			var duration time.Duration
			if exists && !start.IsZero() {
				duration = time.Since(start)
			}

			if h.onEnd != nil && (string(info.Component) == "Graph" || string(info.Component) == "Chain") {
				h.onEnd(info.Name, duration)
			}

			if h.onNodeEnd != nil && string(info.Component) != "Chain" && string(info.Component) != "Graph" {
				h.onNodeEnd(info.Name, string(info.Component))
			}

			if h.onNodeEndMeta != nil && string(info.Component) != "Chain" && string(info.Component) != "Graph" {
				meta := map[string]any{
					"component": string(info.Component),
				}
				if info.Component == components.ComponentOfChatModel {
					mOutput := model.ConvCallbackOutput(output)
					if mOutput != nil && mOutput.TokenUsage != nil {
						meta["prompt_tokens"] = mOutput.TokenUsage.PromptTokens
						meta["completion_tokens"] = mOutput.TokenUsage.CompletionTokens
						meta["total_tokens"] = mOutput.TokenUsage.TotalTokens
					}
					if mOutput != nil && mOutput.Config != nil {
						meta["model"] = mOutput.Config.Model
					}
				}
				h.onNodeEndMeta(info.Name, string(info.Component), meta)
			}

			if h.onToolResult != nil && info.Component == components.ComponentOfTool {
				if toolOutput := tool.ConvCallbackOutput(output); toolOutput != nil {
					h.onToolResult(info.Name, info.Name, toolOutput.Response)
				}
			}

			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			if h.onError != nil {
				h.onError(info.Name, err)
			}
			return ctx
		}).
		Build()
}

// StreamingOutput provides a channel-based interface for real-time agent output
type StreamingOutput struct {
	Events    chan StreamEvent
	done      chan struct{}
	observers []func(StreamEvent)
	mu        sync.RWMutex
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
	EventNodeStart  StreamEventType = "node_start"
	EventNodeEnd    StreamEventType = "node_end"
)

// NewStreamingOutput creates a new streaming output channel
func NewStreamingOutput(bufferSize int) *StreamingOutput {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &StreamingOutput{
		Events:    make(chan StreamEvent, bufferSize),
		done:      make(chan struct{}),
		observers: nil,
	}
}

// Emit sends an event to the stream
func (s *StreamingOutput) Emit(eventType StreamEventType, agent, content string, metadata map[string]any) {
	event := StreamEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		Agent:     agent,
		Content:   content,
		Metadata:  metadata,
	}
	select {
	case s.Events <- event:
	case <-s.done:
		// Stream closed
	default:
		// Buffer full, drop event
	}

	s.mu.RLock()
	observers := append([]func(StreamEvent){}, s.observers...)
	s.mu.RUnlock()
	for _, obs := range observers {
		if obs == nil {
			continue
		}
		go obs(event)
	}
}

// Close closes the streaming output
func (s *StreamingOutput) Close() {
	close(s.done)
	close(s.Events)
}

// AddObserver registers a callback that receives all events.
func (s *StreamingOutput) AddObserver(fn func(StreamEvent)) {
	if fn == nil {
		return
	}
	s.mu.Lock()
	s.observers = append(s.observers, fn)
	s.mu.Unlock()
}

// CreateStreamingCallbackHandler creates a callback handler that emits to a stream
func CreateStreamingCallbackHandler(agentName string, stream *StreamingOutput) *CallbackHandler {
	handler := NewCallbackHandler()

	handler.
		OnStart(func(name string) {
			// Name here is "Run Name", which usually matches agentName for root
			stream.Emit(EventAgentStart, agentName, fmt.Sprintf("Agent %s starting", agentName), map[string]any{
				"agent": agentName,
			})
		}).
		OnEnd(func(name string, duration time.Duration) {
			stream.Emit(EventAgentEnd, agentName, fmt.Sprintf("Agent %s completed in %.2fs", agentName, duration.Seconds()), map[string]any{
				"agent":       agentName,
				"duration_ms": duration.Milliseconds(),
			})
		}).
		OnError(func(name string, err error) {
			stream.Emit(EventAgentError, agentName, err.Error(), map[string]any{
				"agent": agentName,
			})
		}).
		OnToolCall(func(name, tool, args string) {
			safeArgs := strings.ReplaceAll(args, "\r", " ")
			safeArgs = strings.ReplaceAll(safeArgs, "\n", " ")
			stream.Emit(EventToolCall, agentName, fmt.Sprintf("%s(%s)", tool, utils.Truncate(safeArgs, 50)), map[string]any{
				"tool": tool,
				"args": args,
			})
		}).
		OnToolResult(func(name, tool, result string) {
			safeResult := strings.ReplaceAll(result, "\r", " ")
			safeResult = strings.ReplaceAll(safeResult, "\n", " ")
			stream.Emit(EventToolResult, agentName, utils.Truncate(safeResult, 120), map[string]any{
				"tool": tool,
			})
		})

	// Add node-level visibility for "The Pulse"
	handler.OnNodeStartMeta(func(name, nodeType string, meta map[string]any) {
		// Map internal node names to user-friendly activity descriptions
		activity := name

		// Map by node type first
		switch nodeType {
		case "ChatModel":
			if modelName, ok := meta["model"].(string); ok && modelName != "" {
				activity = fmt.Sprintf("Calling LLM (%s)", modelName)
			} else {
				activity = "Calling LLM..."
			}
		case "Lambda":
			switch name {
			case "prompt", "template":
				activity = "Preparing prompt..."
			case "model":
				activity = "Thinking..."
			case "parser", "parse":
				activity = "Processing response..."
			default:
				activity = "Processing..."
			}
		case "ToolsNode":
			activity = "Using tools..."
		case "Retriever":
			activity = "Searching..."
		default:
			// Use name if no type match
			if name != "" && name != agentName {
				activity = name + "..."
			} else {
				activity = "Analyzing..."
			}
		}

		if meta == nil {
			meta = map[string]any{}
		}
		meta["node_name"] = name
		meta["node_type"] = nodeType
		stream.Emit(EventNodeStart, agentName, activity, meta)
	})

	handler.OnNodeEndMeta(func(name, nodeType string, meta map[string]any) {
		if meta == nil {
			meta = map[string]any{}
		}
		meta["node_name"] = name
		meta["node_type"] = nodeType
		stream.Emit(EventNodeEnd, agentName, name, meta)
	})

	return handler
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
