package telemetry

import (
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/posthog/posthog-go"
)

// Client is the interface for telemetry clients.
// This abstraction allows for mocking in tests and swapping implementations.
type Client interface {
	// Track sends an event asynchronously. Returns immediately without blocking.
	// If telemetry is disabled, this is a no-op.
	Track(event string, properties map[string]any)

	// Close flushes pending events and closes the client.
	// Uses a short timeout to avoid blocking the CLI.
	Close() error
}

// Properties is a type alias for event properties.
type Properties = map[string]any

// enqueuer is an internal interface for the PostHog client methods we use.
// This allows us to mock the client for testing.
type enqueuer interface {
	io.Closer
	Enqueue(msg posthog.Message) error
}

// PostHogClient wraps the PostHog SDK for async telemetry.
type PostHogClient struct {
	client      enqueuer
	config      *Config
	version     string
	mu          sync.RWMutex
	initialized bool
}

// ClientConfig holds configuration for initializing the telemetry client.
type ClientConfig struct {
	// APIKey is the PostHog project API key.
	APIKey string

	// Version is the CLI version string.
	Version string

	// Config is the telemetry configuration (enabled state, anonymous ID).
	Config *Config

	// Endpoint is an optional custom PostHog endpoint (for self-hosted).
	// Leave empty to use the default PostHog cloud endpoint.
	Endpoint string
}

// NewPostHogClient creates a new PostHog telemetry client.
// Returns an uninitialized client if APIKey is empty or Config is nil.
func NewPostHogClient(cfg ClientConfig) (*PostHogClient, error) {
	if cfg.APIKey == "" || cfg.Config == nil {
		return &PostHogClient{
			config:      cfg.Config,
			version:     cfg.Version,
			initialized: false,
		}, nil
	}

	// Configure PostHog client
	phConfig := posthog.Config{
		// Use a small batch size for CLI (we don't send many events)
		BatchSize: 10,
		// Short interval since CLI exits quickly
		Interval: 1 * time.Second,
		// Telemetry must never pollute normal CLI output with transport warnings.
		Logger: quietPostHogLogger{},
	}

	if cfg.Endpoint != "" {
		phConfig.Endpoint = cfg.Endpoint
	}

	client, err := posthog.NewWithConfig(cfg.APIKey, phConfig)
	if err != nil {
		return nil, err
	}

	return &PostHogClient{
		client:      client,
		config:      cfg.Config,
		version:     cfg.Version,
		initialized: true,
	}, nil
}

// newPostHogClientWithEnqueuer creates a client with a custom enqueuer (for testing).
func newPostHogClientWithEnqueuer(enq enqueuer, cfg *Config, version string) *PostHogClient {
	return &PostHogClient{
		client:      enq,
		config:      cfg,
		version:     version,
		initialized: true,
	}
}

// Track sends an event asynchronously.
// Returns immediately without blocking the CLI.
// No-op if telemetry is disabled or client is not initialized.
func (c *PostHogClient) Track(event string, properties map[string]any) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Skip if not initialized or telemetry disabled
	if !c.initialized || c.config == nil || !c.config.IsEnabled() {
		return
	}

	// Build properties with standard fields
	props := posthog.NewProperties()

	// Add custom properties first
	for k, v := range properties {
		props.Set(k, v)
	}

	// Add standard properties (these are always included per policy)
	props.Set("os", runtime.GOOS)
	props.Set("arch", runtime.GOARCH)
	props.Set("cli_version", c.version)

	// Disable person profile processing for GDPR compliance.
	// This ensures telemetry is truly anonymous - no user profiles are created.
	props.Set("$process_person_profile", false)

	// Enqueue event (PostHog client handles async dispatch)
	_ = c.client.Enqueue(posthog.Capture{
		DistinctId: c.config.AnonymousID,
		Event:      event,
		Properties: props,
	})
}

// Close flushes pending events with a short timeout.
// This attempts delivery without blocking the CLI for too long.
func (c *PostHogClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized || c.client == nil {
		return nil
	}

	// PostHog's Close() flushes the queue
	// It has internal timeouts, but we trust the SDK to be reasonable
	return c.client.Close()
}

// NoopClient is a telemetry client that does nothing.
// Used when telemetry is disabled or no API key is configured.
type NoopClient struct{}

// Track is a no-op.
func (c *NoopClient) Track(event string, properties map[string]any) {}

// Close is a no-op.
func (c *NoopClient) Close() error { return nil }

// NewNoopClient returns a client that does nothing.
func NewNoopClient() *NoopClient {
	return &NoopClient{}
}

// quietPostHogLogger suppresses PostHog client logs in normal CLI output.
type quietPostHogLogger struct{}

func (quietPostHogLogger) Debugf(string, ...interface{}) {}
func (quietPostHogLogger) Logf(string, ...interface{})   {}
func (quietPostHogLogger) Warnf(string, ...interface{})  {}
func (quietPostHogLogger) Errorf(string, ...interface{}) {}
