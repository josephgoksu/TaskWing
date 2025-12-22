// Package telemetry provides anonymous usage analytics for TaskWing.
//
// This package implements GDPR-compliant telemetry with:
// - First-run consent prompt (opt-in)
// - Anonymous data only (no PII)
// - Local consent storage (~/.taskwing/telemetry.json)
// - Easy opt-out via config or flag
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultEndpoint is the telemetry collection endpoint
	DefaultEndpoint = "https://telemetry.taskwing.dev/v1/events"

	// ConsentFileName is the name of the consent file
	ConsentFileName = "telemetry.json"

	// FlushInterval is how often to send batched events
	FlushInterval = 30 * time.Second

	// MaxBatchSize is the maximum number of events to batch
	MaxBatchSize = 10
)

// Client is the telemetry client that tracks anonymous usage
type Client struct {
	installID  string
	enabled    bool
	endpoint   string
	cliVersion string
	httpClient *http.Client

	mu     sync.Mutex
	events []Event
}

// NewClient creates a new telemetry client
func NewClient(endpoint, cliVersion string) *Client {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	return &Client{
		endpoint:   endpoint,
		cliVersion: cliVersion,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		events: make([]Event, 0),
	}
}

// Initialize loads consent status and install ID
func (c *Client) Initialize() error {
	consent, err := GetConsentStatus()
	if err != nil {
		return err
	}

	// If no consent file exists yet, telemetry is disabled by default
	if consent == nil {
		c.enabled = false
		c.installID = ""
		return nil
	}

	c.enabled = consent.Enabled
	c.installID = consent.InstallID

	return nil
}

// IsEnabled returns whether telemetry is enabled
func (c *Client) IsEnabled() bool {
	return c.enabled
}

// SetEnabled enables or disables telemetry
func (c *Client) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// Track records an event (async, non-blocking)
func (c *Client) Track(event Event) {
	if !c.enabled {
		return
	}

	// Add common properties
	event.Timestamp = time.Now().UTC()
	event.InstallID = c.installID
	if event.Props == nil {
		event.Props = make(map[string]any)
	}
	event.Props["cli_version"] = c.cliVersion
	event.Props["os"] = runtime.GOOS
	event.Props["arch"] = runtime.GOARCH

	c.mu.Lock()
	c.events = append(c.events, event)
	shouldFlush := len(c.events) >= MaxBatchSize
	c.mu.Unlock()

	if shouldFlush {
		go func() { _ = c.Flush() }()
	}
}

// Flush sends all pending events to the endpoint
func (c *Client) Flush() error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	if len(c.events) == 0 {
		c.mu.Unlock()
		return nil
	}
	events := c.events
	c.events = make([]Event, 0)
	c.mu.Unlock()

	// Send events
	payload, err := json.Marshal(map[string]any{
		"events": events,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Silently fail - telemetry should never block the user
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	return nil
}

// TrackCommand is a convenience method for tracking command execution
func (c *Client) TrackCommand(command string, durationMs int64, success bool, errorType string) {
	event := Event{
		Name: "command_executed",
		Props: map[string]any{
			"command":     command,
			"duration_ms": durationMs,
			"success":     success,
		},
	}
	if errorType != "" {
		event.Props["error_type"] = errorType
	}
	c.Track(event)
}

// TrackBootstrap tracks bootstrap completion
func (c *Client) TrackBootstrap(provider string, nodeCount, fileCount int) {
	c.Track(Event{
		Name: "bootstrap_complete",
		Props: map[string]any{
			"provider":   provider,
			"node_count": nodeCount,
			"file_count": fileCount,
		},
	})
}

// TrackSearch tracks search usage (NOT the query content)
func (c *Client) TrackSearch(resultCount int, hasAnswer bool) {
	c.Track(Event{
		Name: "search_query",
		Props: map[string]any{
			"result_count": resultCount,
			"has_answer":   hasAnswer,
		},
	})
}

// getConfigDir returns the TaskWing config directory
func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".taskwing"), nil
}

// generateInstallID creates a new random install ID
func generateInstallID() string {
	return uuid.New().String()
}

// Global client instance
var defaultClient *Client

// GetClient returns the global telemetry client
func GetClient() *Client {
	return defaultClient
}

// Init initializes the global telemetry client
func Init(cliVersion string, disabled bool) error {
	defaultClient = NewClient("", cliVersion)

	if disabled {
		defaultClient.enabled = false
		return nil
	}

	return defaultClient.Initialize()
}

// Track records an event using the global client
func Track(event Event) {
	if defaultClient != nil {
		defaultClient.Track(event)
	}
}

// Flush sends pending events using the global client
func Flush() error {
	if defaultClient != nil {
		return defaultClient.Flush()
	}
	return nil
}

// Shutdown flushes remaining events and closes the client
func Shutdown() {
	if defaultClient != nil {
		_ = defaultClient.Flush()
	}
}
