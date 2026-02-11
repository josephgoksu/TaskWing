package mcpcfg

import (
	"strings"
	"unicode"
)

const CanonicalServerName = "taskwing-mcp"

func IsCanonicalServerName(name string) bool {
	return strings.TrimSpace(strings.ToLower(name)) == CanonicalServerName
}

func IsLegacyServerName(name string) bool {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if normalized == "" || normalized == CanonicalServerName {
		return false
	}
	if normalized == "taskwing" {
		return true
	}
	return strings.HasPrefix(normalized, "taskwing-mcp-") || strings.HasPrefix(normalized, "taskwing-")
}

func ExtractTaskWingServerNames(output string) []string {
	fields := strings.FieldsFunc(strings.ToLower(output), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' && r != '.'
	})

	names := make([]string, 0)
	for _, field := range fields {
		if field == "" {
			continue
		}
		if field == "taskwing" || strings.HasPrefix(field, "taskwing-") {
			names = append(names, field)
		}
	}
	return names
}

func ContainsCanonicalServerName(output string) bool {
	for _, name := range ExtractTaskWingServerNames(output) {
		if IsCanonicalServerName(name) {
			return true
		}
	}
	return false
}

func ContainsLegacyServerName(output string) bool {
	for _, name := range ExtractTaskWingServerNames(output) {
		if IsLegacyServerName(name) {
			return true
		}
	}
	return false
}
