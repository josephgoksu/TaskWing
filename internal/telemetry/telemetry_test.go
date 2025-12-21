package telemetry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("", "1.0.0")

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.endpoint != DefaultEndpoint {
		t.Errorf("expected endpoint %s, got %s", DefaultEndpoint, client.endpoint)
	}

	if client.cliVersion != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", client.cliVersion)
	}

	if client.enabled {
		t.Error("new client should be disabled by default")
	}
}

func TestClientInitialize_NoConsentFile(t *testing.T) {
	// Use temp directory
	origHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	client := NewClient("", "1.0.0")
	err := client.Initialize()

	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if client.enabled {
		t.Error("telemetry should be disabled when no consent file exists")
	}
}

func TestSetConsentStatus(t *testing.T) {
	// Use temp directory
	origHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Set consent
	err := SetConsentStatus(true, "2.0.0")
	if err != nil {
		t.Fatalf("SetConsentStatus failed: %v", err)
	}

	// Verify file exists
	configDir := filepath.Join(tempDir, ".taskwing")
	consentPath := filepath.Join(configDir, "telemetry.json")
	if _, err := os.Stat(consentPath); os.IsNotExist(err) {
		t.Fatal("consent file was not created")
	}

	// Read consent
	consent, err := GetConsentStatus()
	if err != nil {
		t.Fatalf("GetConsentStatus failed: %v", err)
	}

	if consent == nil {
		t.Fatal("GetConsentStatus returned nil")
	}

	if !consent.Enabled {
		t.Error("expected consent to be enabled")
	}

	if consent.InstallID == "" {
		t.Error("expected install ID to be set")
	}

	if consent.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %s", consent.Version)
	}
}

func TestNeedsConsent(t *testing.T) {
	// Use temp directory
	origHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Should need consent initially
	needs, err := NeedsConsent()
	if err != nil {
		t.Fatalf("NeedsConsent failed: %v", err)
	}
	if !needs {
		t.Error("expected to need consent when no file exists")
	}

	// Set consent
	err = SetConsentStatus(false, "2.0.0")
	if err != nil {
		t.Fatalf("SetConsentStatus failed: %v", err)
	}

	// Should not need consent after setting
	needs, err = NeedsConsent()
	if err != nil {
		t.Fatalf("NeedsConsent failed: %v", err)
	}
	if needs {
		t.Error("expected to not need consent after setting")
	}
}

func TestTrack_Disabled(t *testing.T) {
	client := NewClient("", "1.0.0")
	client.enabled = false

	// Should not panic and should not add events
	client.Track(Event{Name: "test_event"})

	if len(client.events) != 0 {
		t.Error("events should not be added when disabled")
	}
}

func TestTrack_Enabled(t *testing.T) {
	client := NewClient("", "1.0.0")
	client.enabled = true
	client.installID = "test-install-id"

	client.Track(Event{Name: "test_event"})

	if len(client.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(client.events))
	}

	event := client.events[0]
	if event.Name != "test_event" {
		t.Errorf("expected event name 'test_event', got '%s'", event.Name)
	}

	if event.InstallID != "test-install-id" {
		t.Errorf("expected install ID 'test-install-id', got '%s'", event.InstallID)
	}

	if event.Props["cli_version"] != "1.0.0" {
		t.Error("expected cli_version to be set")
	}

	if event.Props["os"] == nil {
		t.Error("expected os to be set")
	}

	if event.Props["arch"] == nil {
		t.Error("expected arch to be set")
	}
}

func TestFlush_Empty(t *testing.T) {
	client := NewClient("", "1.0.0")
	client.enabled = true

	// Should not error when no events
	err := client.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}
}

func TestInit(t *testing.T) {
	// Use temp directory
	origHome := os.Getenv("HOME")
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Init with disabled
	err := Init("1.0.0", true)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	client := GetClient()
	if client == nil {
		t.Fatal("GetClient returned nil after Init")
	}

	if client.IsEnabled() {
		t.Error("expected telemetry to be disabled")
	}
}

func TestGenerateInstallID(t *testing.T) {
	id1 := generateInstallID()
	id2 := generateInstallID()

	if id1 == "" {
		t.Error("install ID should not be empty")
	}

	if id1 == id2 {
		t.Error("install IDs should be unique")
	}
}

func TestNewEvent(t *testing.T) {
	event := NewEvent("test_event")

	if event.Name != "test_event" {
		t.Errorf("expected name 'test_event', got '%s'", event.Name)
	}

	if event.Props == nil {
		t.Error("props should not be nil")
	}

	if event.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestEventWithProp(t *testing.T) {
	event := NewEvent("test").WithProp("key", "value")

	if event.Props["key"] != "value" {
		t.Error("expected prop to be set")
	}
}
