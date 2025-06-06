package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// HandleError provides a centralized way to manage CLI errors.
// It prints a user-friendly message by default. If the --verbose
// flag is set, it prints the full technical error.
// After printing the message, it exits the application with a status code of 1.
func HandleError(userMsg string, technicalErr error) {
	if viper.GetBool("verbose") && technicalErr != nil {
		// In verbose mode, print the detailed, underlying technical error.
		fmt.Fprintf(os.Stderr, "Error: %v\n", technicalErr)
	} else {
		// By default, print the clean, user-friendly message.
		fmt.Fprintln(os.Stderr, userMsg)
	}
	os.Exit(1)
}
