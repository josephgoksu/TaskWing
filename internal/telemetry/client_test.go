package telemetry

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/posthog/posthog-go"
)

// mockEnqueuer captures events for testing.
type mockEnqueuer struct {
	mu     sync.Mutex
	events []posthog.Capture
	closed bool
}

func (m *mockEnqueuer) Enqueue(msg posthog.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if capture, ok := msg.(posthog.Capture); ok {
		m.events = append(m.events, capture)
	}
	return nil
}

func (m *mockEnqueuer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockEnqueuer) getEvents() []posthog.Capture {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]posthog.Capture, len(m.events))
	copy(result, m.events)
	return result
}

func (m *mockEnqueuer) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// newTestClient creates a PostHogClient with a mock enqueuer for testing.
func newTestClient(cfg *Config, version string) (*PostHogClient, *mockEnqueuer) {
	mock := &mockEnqueuer{}
	client := newPostHogClientWithEnqueuer(mock, cfg, version)
	return client, mock
}

func TestPostHogClient_Track_WhenEnabled(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		ConsentAsked: true,
		AnonymousID:  "test-anon-id-123",
	}

	client, mock := newTestClient(cfg, "1.2.3")

	// Track an event
	client.Track("command_executed", Properties{
		"command":  "bootstrap",
		"success":  true,
		"duration": 1500,
	})

	// Verify event was captured
	events := mock.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	// Check event name
	if event.Event != "command_executed" {
		t.Errorf("event name = %q, want %q", event.Event, "command_executed")
	}

	// Check distinct ID is anonymous ID
	if event.DistinctId != "test-anon-id-123" {
		t.Errorf("distinct_id = %q, want %q", event.DistinctId, "test-anon-id-123")
	}

	// Check custom properties
	if event.Properties["command"] != "bootstrap" {
		t.Errorf("command = %v, want %q", event.Properties["command"], "bootstrap")
	}
	if event.Properties["success"] != true {
		t.Errorf("success = %v, want true", event.Properties["success"])
	}
	if event.Properties["duration"] != 1500 {
		t.Errorf("duration = %v, want 1500", event.Properties["duration"])
	}

	// Check standard properties are added
	if event.Properties["os"] != runtime.GOOS {
		t.Errorf("os = %v, want %q", event.Properties["os"], runtime.GOOS)
	}
	if event.Properties["arch"] != runtime.GOARCH {
		t.Errorf("arch = %v, want %q", event.Properties["arch"], runtime.GOARCH)
	}
	if event.Properties["cli_version"] != "1.2.3" {
		t.Errorf("cli_version = %v, want %q", event.Properties["cli_version"], "1.2.3")
	}
}

func TestPostHogClient_Track_WhenDisabled(t *testing.T) {
	cfg := &Config{
		Enabled:      false, // Disabled
		ConsentAsked: true,
		AnonymousID:  "test-anon-id-123",
	}

	client, mock := newTestClient(cfg, "1.2.3")

	// Track an event
	client.Track("command_executed", Properties{
		"command": "bootstrap",
	})

	// Verify no events were captured
	events := mock.getEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events when disabled, got %d", len(events))
	}
}

func TestPostHogClient_Track_NotInitialized(t *testing.T) {
	client := &PostHogClient{
		config:      &Config{Enabled: true},
		initialized: false, // Not initialized
	}

	// This should not panic
	client.Track("test_event", nil)
}

func TestPostHogClient_Track_NilConfig(t *testing.T) {
	mock := &mockEnqueuer{}
	client := &PostHogClient{
		client:      mock,
		config:      nil, // Nil config
		initialized: true,
	}

	// This should not panic and should be a no-op
	client.Track("test_event", nil)

	events := mock.getEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events with nil config, got %d", len(events))
	}
}

func TestPostHogClient_Track_NilProperties(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		ConsentAsked: true,
		AnonymousID:  "test-anon-id",
	}

	client, mock := newTestClient(cfg, "1.0.0")

	// Track with nil properties
	client.Track("simple_event", nil)

	events := mock.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Standard properties should still be added
	event := events[0]
	if event.Properties["os"] != runtime.GOOS {
		t.Errorf("os should be set even with nil properties")
	}
}

func TestPostHogClient_Close(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		ConsentAsked: true,
		AnonymousID:  "test-anon-id",
	}

	client, mock := newTestClient(cfg, "1.0.0")

	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if !mock.isClosed() {
		t.Error("underlying client should be closed")
	}
}

func TestPostHogClient_Close_NotInitialized(t *testing.T) {
	client := &PostHogClient{
		initialized: false,
	}

	// Should not error
	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestNoopClient(t *testing.T) {
	client := NewNoopClient()

	// Track should not panic
	client.Track("event", Properties{"key": "value"})

	// Close should not error
	if err := client.Close(); err != nil {
		t.Errorf("NoopClient.Close() error = %v", err)
	}
}

func TestNewPostHogClient_EmptyAPIKey(t *testing.T) {
	client, err := NewPostHogClient(ClientConfig{
		APIKey:  "", // Empty
		Version: "1.0.0",
		Config:  &Config{Enabled: true},
	})

	if err != nil {
		t.Errorf("should not error with empty API key, got %v", err)
	}

	if client.initialized {
		t.Error("should not be initialized with empty API key")
	}

	// Track should be a no-op, not panic
	client.Track("event", nil)
}

func TestNewPostHogClient_NilConfig(t *testing.T) {
	client, err := NewPostHogClient(ClientConfig{
		APIKey:  "test-key",
		Version: "1.0.0",
		Config:  nil, // Nil config
	})

	if err != nil {
		t.Errorf("should not error with nil config, got %v", err)
	}

	if client.initialized {
		t.Error("should not be initialized with nil config")
	}
}

func TestPostHogClient_Track_Concurrent(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		ConsentAsked: true,
		AnonymousID:  "test-anon-id",
	}

	client, mock := newTestClient(cfg, "1.0.0")

	// Track concurrently
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			client.Track("concurrent_event", Properties{"iteration": n})
		}(i)
	}
	wg.Wait()

	events := mock.getEvents()
	if len(events) != 100 {
		t.Errorf("expected 100 events, got %d", len(events))
	}
}

func TestPostHogClient_Track_ReturnsImmediately(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		ConsentAsked: true,
		AnonymousID:  "test-anon-id",
	}

	client, _ := newTestClient(cfg, "1.0.0")

	// Track should return immediately (non-blocking)
	// This is a basic smoke test - the actual async behavior is handled by PostHog SDK
	done := make(chan bool, 1)
	go func() {
		client.Track("test_event", nil)
		done <- true
	}()

	// Give goroutine time to complete - Track should be nearly instant
	select {
	case <-done:
		// Success - returned quickly
	case <-time.After(100 * time.Millisecond):
		t.Error("Track() should return immediately (within 100ms)")
	}
}
