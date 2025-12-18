package bootstrap

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Spinner provides a simple CLI spinner for long-running operations
type Spinner struct {
	frames   []string
	interval time.Duration
	writer   io.Writer
	message  string
	stopCh   chan struct{}
	done     chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(w io.Writer, message string) *Spinner {
	return &Spinner{
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		interval: 80 * time.Millisecond,
		writer:   w,
		message:  message,
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		frameIdx := 0
		for {
			select {
			case <-s.stopCh:
				close(s.done)
				return
			default:
				frame := s.frames[frameIdx%len(s.frames)]
				fmt.Fprintf(s.writer, "\r   %s %s", frame, s.message)
				frameIdx++
				time.Sleep(s.interval)
			}
		}
	}()
}

// Stop stops the spinner and prints the final message
func (s *Spinner) Stop(success bool, result string) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	<-s.done

	icon := "✓"
	if !success {
		icon = "✗"
	}
	// Clear line and print result
	fmt.Fprintf(s.writer, "\r   %s %s %s\n", icon, s.message, result)
}

// StepProgress represents a visual step in the bootstrap process
type StepProgress struct {
	writer io.Writer
}

// NewStepProgress creates a new step progress helper
func NewStepProgress(w io.Writer) *StepProgress {
	return &StepProgress{writer: w}
}

// Step prints a step with emoji and returns a function to complete it
func (sp *StepProgress) Step(emoji, message string) func(result string) {
	spinner := NewSpinner(sp.writer, message)
	spinner.Start()
	return func(result string) {
		spinner.Stop(true, result)
	}
}

// Info prints an info line
func (sp *StepProgress) Info(emoji, message, result string) {
	fmt.Fprintf(sp.writer, "   %s %s %s\n", emoji, message, result)
}

// Header prints a section header
func (sp *StepProgress) Header(message string) {
	fmt.Fprintf(sp.writer, "\n  %s\n", message)
}
