package ui

import (
	"testing"
	"time"
)

func TestSpinnerStartStop(t *testing.T) {
	s := NewSpinner("testing...")
	s.Start()
	time.Sleep(200 * time.Millisecond) // Let a few frames render
	s.Stop()
	// Should not panic or hang
}

func TestSpinnerDoubleStop(t *testing.T) {
	s := NewSpinner("testing...")
	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()
	s.Stop() // Should not panic
}
