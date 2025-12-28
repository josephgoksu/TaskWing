/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package core provides MCP (Model Context Protocol) integration for
live updates when agents discover new findings.
*/
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MCPNotifier sends live updates to connected MCP clients
type MCPNotifier struct {
	subscribers []MCPSubscriber
	mu          sync.RWMutex
	buffer      []MCPNotification
	bufferSize  int
	stream      *StreamingOutput
}

// MCPSubscriber receives notifications about agent findings
type MCPSubscriber interface {
	OnFindingAdded(ctx context.Context, finding Finding) error
	OnFindingUpdated(ctx context.Context, finding Finding) error
	OnFindingRemoved(ctx context.Context, findingID string) error
	OnBatchComplete(ctx context.Context, summary MCPBatchSummary) error
}

// MCPNotification represents a single notification to MCP clients
type MCPNotification struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

// MCPBatchSummary summarizes a batch of agent findings
type MCPBatchSummary struct {
	AgentName     string    `json:"agent_name"`
	TotalFindings int       `json:"total_findings"`
	NewFindings   int       `json:"new_findings"`
	UpdatedCount  int       `json:"updated_count"`
	Duration      string    `json:"duration"`
	Timestamp     time.Time `json:"timestamp"`
}

// NewMCPNotifier creates a new MCP notifier
func NewMCPNotifier() *MCPNotifier {
	return &MCPNotifier{
		subscribers: make([]MCPSubscriber, 0),
		buffer:      make([]MCPNotification, 0),
		bufferSize:  100,
	}
}

// AttachStream connects the notifier to a streaming output for UI observability.
func (n *MCPNotifier) AttachStream(stream *StreamingOutput) {
	n.mu.Lock()
	n.stream = stream
	n.mu.Unlock()
}

// Subscribe adds a subscriber to receive notifications
func (n *MCPNotifier) Subscribe(sub MCPSubscriber) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.subscribers = append(n.subscribers, sub)
}

// Unsubscribe removes a subscriber
func (n *MCPNotifier) Unsubscribe(sub MCPSubscriber) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i, s := range n.subscribers {
		if s == sub {
			n.subscribers = append(n.subscribers[:i], n.subscribers[i+1:]...)
			return
		}
	}
}

// NotifyFindingAdded notifies all subscribers of a new finding
func (n *MCPNotifier) NotifyFindingAdded(ctx context.Context, finding Finding) {
	n.mu.RLock()
	subs := append([]MCPSubscriber(nil), n.subscribers...)
	n.mu.RUnlock()

	n.addToBuffer("finding_added", finding)
	n.emitStream(EventFinding, "mcp", "finding_added", map[string]any{
		"title": finding.Title,
	})

	for _, sub := range subs {
		go func(s MCPSubscriber) {
			notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_ = s.OnFindingAdded(notifyCtx, finding)
		}(sub)
	}
}

// NotifyFindingUpdated notifies all subscribers of an updated finding
func (n *MCPNotifier) NotifyFindingUpdated(ctx context.Context, finding Finding) {
	n.mu.RLock()
	subs := append([]MCPSubscriber(nil), n.subscribers...)
	n.mu.RUnlock()

	n.addToBuffer("finding_updated", finding)
	n.emitStream(EventFinding, "mcp", "finding_updated", map[string]any{
		"title": finding.Title,
	})

	for _, sub := range subs {
		go func(s MCPSubscriber) {
			notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_ = s.OnFindingUpdated(notifyCtx, finding)
		}(sub)
	}
}

// NotifyFindingRemoved notifies all subscribers of a removed finding
func (n *MCPNotifier) NotifyFindingRemoved(ctx context.Context, findingID string) {
	n.mu.RLock()
	subs := append([]MCPSubscriber(nil), n.subscribers...)
	n.mu.RUnlock()

	n.addToBuffer("finding_removed", map[string]string{"id": findingID})
	n.emitStream(EventFinding, "mcp", "finding_removed", map[string]any{
		"finding_id": findingID,
	})

	for _, sub := range subs {
		go func(s MCPSubscriber) {
			notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_ = s.OnFindingRemoved(notifyCtx, findingID)
		}(sub)
	}
}

// NotifyBatchComplete notifies all subscribers that a batch is complete
func (n *MCPNotifier) NotifyBatchComplete(ctx context.Context, summary MCPBatchSummary) {
	n.mu.RLock()
	subs := append([]MCPSubscriber(nil), n.subscribers...)
	n.mu.RUnlock()

	n.addToBuffer("batch_complete", summary)
	n.emitStream(EventSynthesis, "mcp", "batch_complete", map[string]any{
		"agent":         summary.AgentName,
		"total":         summary.TotalFindings,
		"new_findings":  summary.NewFindings,
		"updated_count": summary.UpdatedCount,
		"duration":      summary.Duration,
	})

	for _, sub := range subs {
		go func(s MCPSubscriber) {
			notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_ = s.OnBatchComplete(notifyCtx, summary)
		}(sub)
	}
}

func (n *MCPNotifier) emitStream(eventType StreamEventType, agent, content string, metadata map[string]any) {
	n.mu.RLock()
	stream := n.stream
	n.mu.RUnlock()
	if stream == nil {
		return
	}
	stream.Emit(eventType, agent, content, metadata)
}

// addToBuffer adds a notification to the circular buffer
func (n *MCPNotifier) addToBuffer(notifType string, payload any) {
	n.mu.Lock()
	defer n.mu.Unlock()

	notification := MCPNotification{
		Type:      notifType,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	if len(n.buffer) >= n.bufferSize {
		n.buffer = n.buffer[1:]
	}
	n.buffer = append(n.buffer, notification)
}

// GetRecentNotifications returns the most recent notifications
func (n *MCPNotifier) GetRecentNotifications(count int) []MCPNotification {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if count > len(n.buffer) {
		count = len(n.buffer)
	}

	start := len(n.buffer) - count
	result := make([]MCPNotification, count)
	copy(result, n.buffer[start:])
	return result
}

// SubscriberCount returns the number of active subscribers
func (n *MCPNotifier) SubscriberCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.subscribers)
}

// LoggingSubscriber is a simple subscriber that logs notifications
type LoggingSubscriber struct {
	Name string
}

func (s *LoggingSubscriber) OnFindingAdded(ctx context.Context, finding Finding) error {
	fmt.Printf("[%s] Finding added: %s\n", s.Name, finding.Title)
	return nil
}

func (s *LoggingSubscriber) OnFindingUpdated(ctx context.Context, finding Finding) error {
	fmt.Printf("[%s] Finding updated: %s\n", s.Name, finding.Title)
	return nil
}

func (s *LoggingSubscriber) OnFindingRemoved(ctx context.Context, findingID string) error {
	fmt.Printf("[%s] Finding removed: %s\n", s.Name, findingID)
	return nil
}

func (s *LoggingSubscriber) OnBatchComplete(ctx context.Context, summary MCPBatchSummary) error {
	fmt.Printf("[%s] Batch complete: %s (%d findings)\n", s.Name, summary.AgentName, summary.TotalFindings)
	return nil
}

// JSONSubscriber sends notifications as JSON to a callback
type JSONSubscriber struct {
	Callback func(json string)
}

func (s *JSONSubscriber) OnFindingAdded(ctx context.Context, finding Finding) error {
	return s.sendJSON("finding_added", finding)
}

func (s *JSONSubscriber) OnFindingUpdated(ctx context.Context, finding Finding) error {
	return s.sendJSON("finding_updated", finding)
}

func (s *JSONSubscriber) OnFindingRemoved(ctx context.Context, findingID string) error {
	return s.sendJSON("finding_removed", map[string]string{"id": findingID})
}

func (s *JSONSubscriber) OnBatchComplete(ctx context.Context, summary MCPBatchSummary) error {
	return s.sendJSON("batch_complete", summary)
}

func (s *JSONSubscriber) sendJSON(eventType string, payload any) error {
	if s.Callback == nil {
		return nil
	}

	data, err := json.Marshal(map[string]any{
		"type":      eventType,
		"timestamp": time.Now().Format(time.RFC3339),
		"payload":   payload,
	})
	if err != nil {
		return err
	}

	s.Callback(string(data))
	return nil
}
