package ui

import (
	"fmt"
	"sync"
	"time"
)

// Spinner is a simple text-based spinner for CLI usage
type Spinner struct {
	chars    []string
	delay    time.Duration
	suffix   string
	stopChan chan struct{}
	wg       sync.WaitGroup
	active   bool
	mu       sync.Mutex
}

// NewSpinner creates a new spinner
func NewSpinner(suffix string) *Spinner {
	return &Spinner{
		chars:    []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		delay:    100 * time.Millisecond,
		suffix:   suffix,
		stopChan: make(chan struct{}),
	}
}

// Start starts the spinner in a background goroutine
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.stopChan = make(chan struct{})
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		i := 0
		for {
			select {
			case <-s.stopChan:
				return
			case <-time.After(s.delay):
				i = (i + 1) % len(s.chars)
				// Use \r to overwrite line
				fmt.Printf("\r%s %s", StylePrimary.Render(s.chars[i]), s.suffix)
			}
		}
	}()
}

// Stop stops the spinner and clears the line
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	close(s.stopChan)
	s.mu.Unlock()

	s.wg.Wait()
	fmt.Printf("\r\033[K") // Clear line
}
