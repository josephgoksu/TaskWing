package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ConsentStatus represents the user's telemetry consent
type ConsentStatus struct {
	InstallID   string    `json:"install_id"`
	Enabled     bool      `json:"enabled"`
	ConsentDate time.Time `json:"consent_date"`
	Version     string    `json:"version"`
}

// GetConsentStatus reads the consent status from disk
func GetConsentStatus() (*ConsentStatus, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	consentPath := filepath.Join(configDir, ConsentFileName)

	data, err := os.ReadFile(consentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No consent file yet
		}
		return nil, err
	}

	var consent ConsentStatus
	if err := json.Unmarshal(data, &consent); err != nil {
		return nil, err
	}

	return &consent, nil
}

// SetConsentStatus saves the consent status to disk
func SetConsentStatus(enabled bool, version string) error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	consentPath := filepath.Join(configDir, ConsentFileName)

	// Check if we have an existing install ID
	var installID string
	existing, err := GetConsentStatus()
	if err == nil && existing != nil && existing.InstallID != "" {
		installID = existing.InstallID
	} else {
		installID = generateInstallID()
	}

	consent := ConsentStatus{
		InstallID:   installID,
		Enabled:     enabled,
		ConsentDate: time.Now().UTC(),
		Version:     version,
	}

	data, err := json.MarshalIndent(consent, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(consentPath, data, 0644)
}

// NeedsConsent returns true if the user hasn't given consent yet
func NeedsConsent() (bool, error) {
	consent, err := GetConsentStatus()
	if err != nil {
		return true, err
	}
	return consent == nil, nil
}

// PromptForConsent displays the consent prompt and returns the user's choice
func PromptForConsent(version string) (bool, error) {
	// Check if we're in a non-interactive environment
	if !isInteractive() {
		// In non-interactive mode, default to disabled
		if err := SetConsentStatus(false, version); err != nil {
			return false, err
		}
		return false, nil
	}

	// Display consent prompt
	fmt.Println()
	fmt.Println("╭──────────────────────────────────────────────────────────────╮")
	fmt.Println("│  📊 Help improve TaskWing?                                   │")
	fmt.Println("│                                                              │")
	fmt.Println("│  TaskWing collects anonymous usage statistics to improve     │")
	fmt.Println("│  the product. No personal data or code is ever collected.    │")
	fmt.Println("│                                                              │")
	fmt.Println("│  What we collect:                                            │")
	fmt.Println("│  • Command usage (e.g., \"bootstrap ran successfully\")        │")
	fmt.Println("│  • Errors (type only, no file paths)                         │")
	fmt.Println("│  • OS and architecture                                       │")
	fmt.Println("│                                                              │")
	fmt.Println("│  What we DON'T collect:                                      │")
	fmt.Println("│  • Your code, file paths, or project names                   │")
	fmt.Println("│  • API keys, usernames, or IP addresses                      │")
	fmt.Println("│                                                              │")
	fmt.Println("│  You can change this anytime with:                           │")
	fmt.Println("│    taskwing config telemetry disable                         │")
	fmt.Println("╰──────────────────────────────────────────────────────────────╯")
	fmt.Println()
	fmt.Print("Enable anonymous telemetry? [Y/n] ")

	// Read user input
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		// Default to disabled on error
		if err := SetConsentStatus(false, version); err != nil {
			return false, err
		}
		return false, nil
	}

	input = strings.TrimSpace(strings.ToLower(input))
	enabled := input == "" || input == "y" || input == "yes"

	if err := SetConsentStatus(enabled, version); err != nil {
		return false, err
	}

	if enabled {
		fmt.Println("✅ Telemetry enabled. Thank you for helping improve TaskWing!")
	} else {
		fmt.Println("✅ Telemetry disabled. You can enable it anytime with: taskwing config telemetry enable")
	}
	fmt.Println()

	return enabled, nil
}

// isInteractive returns true if stdin is a terminal
func isInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// CheckAndPromptConsent checks if consent is needed and prompts if so
func CheckAndPromptConsent(version string) (bool, error) {
	needs, err := NeedsConsent()
	if err != nil {
		return false, err
	}

	if needs {
		return PromptForConsent(version)
	}

	consent, err := GetConsentStatus()
	if err != nil {
		return false, err
	}

	return consent.Enabled, nil
}
