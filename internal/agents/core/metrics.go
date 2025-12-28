/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package core provides metrics collection for agent performance monitoring.
*/
package core

import (
	"sync"
	"sync/atomic"
	"time"
)

// AgentMetrics collects performance metrics for agents
type AgentMetrics struct {
	// Counters
	TotalRuns      atomic.Int64
	TotalFindings  atomic.Int64
	TotalToolCalls atomic.Int64
	TotalErrors    atomic.Int64

	// Per-agent counters
	agentRuns   map[string]*atomic.Int64
	agentErrors map[string]*atomic.Int64

	// Timing
	totalDuration atomic.Int64 // nanoseconds

	// Watch metrics
	FilesWatched    atomic.Int64
	ChangesDetected atomic.Int64
	ChangesBatched  atomic.Int64

	mu sync.RWMutex
}

// NewAgentMetrics creates a new metrics collector
func NewAgentMetrics() *AgentMetrics {
	return &AgentMetrics{
		agentRuns:   make(map[string]*atomic.Int64),
		agentErrors: make(map[string]*atomic.Int64),
	}
}

// RecordRun records an agent run
func (m *AgentMetrics) RecordRun(agentName string, findings int, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRuns.Add(1)
	m.TotalFindings.Add(int64(findings))
	m.totalDuration.Add(int64(duration))

	if m.agentRuns[agentName] == nil {
		m.agentRuns[agentName] = &atomic.Int64{}
		m.agentErrors[agentName] = &atomic.Int64{}
	}
	m.agentRuns[agentName].Add(1)

	if err != nil {
		m.TotalErrors.Add(1)
		m.agentErrors[agentName].Add(1)
	}
}

// RecordToolCall records a tool invocation
func (m *AgentMetrics) RecordToolCall() {
	m.TotalToolCalls.Add(1)
}

// RecordFileChange records a file change detection
func (m *AgentMetrics) RecordFileChange() {
	m.ChangesDetected.Add(1)
}

// RecordBatch records a batched change
func (m *AgentMetrics) RecordBatch() {
	m.ChangesBatched.Add(1)
}

// GetSnapshot returns a snapshot of current metrics
func (m *AgentMetrics) GetSnapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agentStats := make(map[string]AgentStats)
	for name, runs := range m.agentRuns {
		errors := m.agentErrors[name]
		agentStats[name] = AgentStats{
			Runs:   runs.Load(),
			Errors: errors.Load(),
		}
	}

	totalDuration := time.Duration(m.totalDuration.Load())
	runs := m.TotalRuns.Load()

	var avgDuration time.Duration
	if runs > 0 {
		avgDuration = totalDuration / time.Duration(runs)
	}

	return MetricsSnapshot{
		TotalRuns:       runs,
		TotalFindings:   m.TotalFindings.Load(),
		TotalToolCalls:  m.TotalToolCalls.Load(),
		TotalErrors:     m.TotalErrors.Load(),
		TotalDuration:   totalDuration,
		AvgRunDuration:  avgDuration,
		AgentStats:      agentStats,
		FilesWatched:    m.FilesWatched.Load(),
		ChangesDetected: m.ChangesDetected.Load(),
		ChangesBatched:  m.ChangesBatched.Load(),
	}
}

// MetricsSnapshot is a point-in-time view of metrics
type MetricsSnapshot struct {
	TotalRuns       int64
	TotalFindings   int64
	TotalToolCalls  int64
	TotalErrors     int64
	TotalDuration   time.Duration
	AvgRunDuration  time.Duration
	AgentStats      map[string]AgentStats
	FilesWatched    int64
	ChangesDetected int64
	ChangesBatched  int64
}

// AgentStats contains per-agent statistics
type AgentStats struct {
	Runs   int64
	Errors int64
}

// String returns a human-readable metrics summary
func (s MetricsSnapshot) String() string {
	return formatMetrics(s)
}

func formatMetrics(s MetricsSnapshot) string {
	result := "=== Agent Metrics ===\n"
	result += fmtInt("Total Runs", s.TotalRuns)
	result += fmtInt("Total Findings", s.TotalFindings)
	result += fmtInt("Total Tool Calls", s.TotalToolCalls)
	result += fmtInt("Total Errors", s.TotalErrors)
	result += fmtDuration("Total Duration", s.TotalDuration)
	result += fmtDuration("Avg Run Duration", s.AvgRunDuration)

	if len(s.AgentStats) > 0 {
		result += "\n--- Per-Agent ---\n"
		for name, stats := range s.AgentStats {
			result += fmtAgentStats(name, stats)
		}
	}

	if s.FilesWatched > 0 || s.ChangesDetected > 0 {
		result += "\n--- Watch Mode ---\n"
		result += fmtInt("Files Watched", s.FilesWatched)
		result += fmtInt("Changes Detected", s.ChangesDetected)
		result += fmtInt("Batches Processed", s.ChangesBatched)
	}

	return result
}

func fmtInt(label string, val int64) string {
	return label + ": " + itoa(val) + "\n"
}

func fmtDuration(label string, d time.Duration) string {
	return label + ": " + d.Round(time.Millisecond).String() + "\n"
}

func fmtAgentStats(name string, stats AgentStats) string {
	return name + ": " + itoa(stats.Runs) + " runs, " + itoa(stats.Errors) + " errors\n"
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}

	var result []byte
	neg := i < 0
	if neg {
		i = -i
	}

	for i > 0 {
		result = append([]byte{byte('0' + i%10)}, result...)
		i /= 10
	}

	if neg {
		result = append([]byte{'-'}, result...)
	}

	return string(result)
}
