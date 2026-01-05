package utils

import (
	"strings"
)

// Truncate returns a truncated string with "..." if it exceeds maxLen.
// This function is Unicode-safe, counting runes instead of bytes.
func Truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// ToTitle converts the first character of a string to uppercase.
func ToTitle(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
