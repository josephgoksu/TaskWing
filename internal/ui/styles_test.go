package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

func TestStyles(t *testing.T) {
	// Force color profile for testing
	lipgloss.SetColorProfile(termenv.ANSI256)

	// Verify critical styles are defined and return something
	assert.NotNil(t, StyleTitle)
	assert.NotNil(t, StyleSuccess)

	out := StyleSuccess.Render("Test")
	assert.Contains(t, out, "Test")
	// Verify ANSI codes are present
	assert.NotEqual(t, "Test", out, "Style should add ANSI codes when forced")
}

func TestIcon(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)

	icon := "X"
	out := Icon(icon, StyleError)
	assert.Contains(t, out, icon)
	assert.NotEqual(t, icon, out)
}
