package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// HandleFatalError handles unrecoverable errors that should terminate the application.
func HandleFatalError(userMsg string, technicalErr error) {
	PrintError(userMsg, technicalErr)
	os.Exit(1)
}

// PrintError prints an error message without exiting, allowing for recovery.
func PrintError(userMsg string, technicalErr error) {
	if viper.GetBool("verbose") && technicalErr != nil {
		// In verbose mode, print the detailed, underlying technical error.
		fmt.Fprintf(os.Stderr, "Error: %v\n", technicalErr)
	} else {
		// By default, print the clean, user-friendly message.
		fmt.Fprintln(os.Stderr, userMsg)
	}
}

// LogError logs an error without printing to stderr if verbose mode is off.
func LogError(msg string, err error) {
	if viper.GetBool("verbose") {
		if err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] %s: %v\n", msg, err)
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] %s\n", msg)
		}
	}
}
