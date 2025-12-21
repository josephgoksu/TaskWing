package telemetry

import "time"

// Event represents a single telemetry event
type Event struct {
	Name      string         `json:"name"`
	Timestamp time.Time      `json:"ts"`
	InstallID string         `json:"install_id"`
	Props     map[string]any `json:"props,omitempty"`
}

// Common event names
const (
	EventCommandExecuted   = "command_executed"
	EventCommandError      = "command_error"
	EventBootstrapComplete = "bootstrap_complete"
	EventSearchQuery       = "search_query"
	EventSessionStart      = "session_start"
)

// NewEvent creates a new event with the given name
func NewEvent(name string) Event {
	return Event{
		Name:      name,
		Timestamp: time.Now().UTC(),
		Props:     make(map[string]any),
	}
}

// WithProp adds a property to the event
func (e Event) WithProp(key string, value any) Event {
	if e.Props == nil {
		e.Props = make(map[string]any)
	}
	e.Props[key] = value
	return e
}
