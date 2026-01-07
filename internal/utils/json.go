package utils

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Pre-compiled regexes for JSON repair (compiled once, used many times)
// NOTE: These handle common LLM output errors but have limitations:
// - Escaped quotes within single-quoted strings are not fully supported
// - Complex nested structures may not be repaired correctly
var (
	// Fix missing comma after value before new key: "value" "key" -> "value", "key"
	// Only match when followed by a key pattern (word + colon)
	missingCommaBeforeKeyRegex = regexp.MustCompile(`(")\s*\n\s*("[\w][^"]*"\s*:)`)

	// Fix missing comma after number/bool/null before quote (new key)
	missingCommaAfterValueRegex = regexp.MustCompile(`(\d|true|false|null)\s*\n\s*("[\w][^"]*"\s*:)`)

	// Fix missing comma after closing brace/bracket before quote
	missingCommaAfterBraceRegex = regexp.MustCompile(`([}\]])\s*\n?\s*("[\w])`)

	// Fix trailing commas before closing brace/bracket
	trailingCommaRegex = regexp.MustCompile(`,\s*([}\]])`)

	// Fix single quotes for object keys: {'key': -> {"key":
	// Only matches simple alphanumeric keys without special chars
	singleQuoteKeyRegex = regexp.MustCompile(`([{,]\s*)'(\w+)'(\s*:)`)

	// Fix single quotes for string values after colon: : 'value' -> : "value"
	// Uses non-greedy match and handles escaped single quotes (backslash-quote)
	// Pattern: match content that doesn't contain unescaped single quotes
	singleQuoteValueRegex = regexp.MustCompile(`(:\s*)'((?:[^'\\]|\\.)*)'(\s*[,}\]])`)

	// Fix unquoted string values: {"key": value} -> {"key": "value"}
	// Only matches simple identifiers (letters, numbers, underscores, hyphens)
	// Excludes: numbers, true, false, null, nested objects/arrays
	unquotedValueRegex = regexp.MustCompile(`(:\s*)([a-zA-Z][a-zA-Z0-9_-]*)(\s*[,}\]])`)

	// Fix unquoted semver values: {"version": ^1.0.0} -> {"version": "^1.0.0"}
	// Matches values starting with semver range prefixes:
	// - Single char: ^, ~, >, <, *
	// - Double char: >=, <=
	// Common in LLM outputs when analyzing package.json dependencies
	unquotedSemverRegex = regexp.MustCompile(`(:\s*)((?:>=|<=|[\^~><*])[\d.a-zA-Z_-]*)(\s*[,}\]])`)
)

// ExtractAndParseJSON extracts JSON from LLM responses and unmarshals it.
// Uses stream-based decoding to robustly ignore trailing text.
// Includes JSON repair for common LLM syntax errors.
func ExtractAndParseJSON[T any](response string) (T, error) {
	var result T

	// 1. Basic cleanup (markdown fences)
	cleaned := cleanLLMResponse(response)
	if cleaned == "" {
		return result, fmt.Errorf("no JSON found in response")
	}

	// 2. Find start of JSON structure
	idx := strings.IndexAny(cleaned, "{[")
	if idx == -1 {
		// Maybe it's a quoted string containing JSON?
		var asString string
		if err := json.Unmarshal([]byte(cleaned), &asString); err == nil {
			// Recurse on the unquoted string
			return ExtractAndParseJSON[T](asString)
		}
		return result, fmt.Errorf("no JSON start ({ or [) found")
	}

	// 3. Use Decoder to parse singular JSON value and ignore the rest
	// This handles cases like: {"a":1} some trailing text
	jsonPart := cleaned[idx:]
	decoder := json.NewDecoder(strings.NewReader(jsonPart))
	if err := decoder.Decode(&result); err != nil {
		// 4. Try JSON repair for common LLM errors
		repaired := repairJSON(jsonPart)
		if repaired != jsonPart {
			dec2 := json.NewDecoder(strings.NewReader(repaired))
			if err2 := dec2.Decode(&result); err2 == nil {
				return result, nil
			}
		}

		// 5. Try unescape fallback
		if strings.Contains(jsonPart, "\\") {
			unescaped := strings.ReplaceAll(jsonPart, "\\\"", "\"")
			unescaped = strings.ReplaceAll(unescaped, "\\n", "\n")
			// Try decoding unescaped version
			dec3 := json.NewDecoder(strings.NewReader(unescaped))
			if err3 := dec3.Decode(&result); err3 == nil {
				return result, nil
			}
			// Also try repair on unescaped
			repairedUnescaped := repairJSON(unescaped)
			dec4 := json.NewDecoder(strings.NewReader(repairedUnescaped))
			if err4 := dec4.Decode(&result); err4 == nil {
				return result, nil
			}
		}
		return result, fmt.Errorf("parse JSON: %w", err)
	}

	return result, nil
}

// repairJSON attempts to fix common JSON syntax errors from LLMs.
// Handles: control characters, missing commas, trailing commas, single quotes for keys and values.
// Uses pre-compiled regexes for performance.
func repairJSON(input string) string {
	result := input

	// 0. Sanitize control characters inside strings (LLMs often output literal tabs/newlines)
	// These are invalid in JSON strings and must be escaped
	result = sanitizeControlChars(result)

	// 1. Fix missing commas between properties (only when followed by a key pattern)
	// Pattern: "value"\n"key": -> "value",\n"key":
	result = missingCommaBeforeKeyRegex.ReplaceAllString(result, `$1, $2`)

	// 2. Fix missing comma after number/bool/null before new key
	// Pattern: 123\n"key": -> 123,\n"key":
	result = missingCommaAfterValueRegex.ReplaceAllString(result, `$1, $2`)

	// 3. Fix missing comma after closing brace/bracket before quote
	// Pattern: } "key" -> }, "key" or ] "key" -> ], "key"
	result = missingCommaAfterBraceRegex.ReplaceAllString(result, `$1, $2`)

	// 4. Fix trailing commas before closing brace/bracket
	// Pattern: ,} -> } or ,] -> ]
	result = trailingCommaRegex.ReplaceAllString(result, `$1`)

	// 5. Fix single quotes for object keys: {'key': -> {"key":
	result = singleQuoteKeyRegex.ReplaceAllString(result, `$1"$2"$3`)

	// 6. Fix single quotes for string values: : 'value' -> : "value"
	// Also convert escaped single quotes (\') to regular quotes for JSON
	result = singleQuoteValueRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract the parts using the regex
		parts := singleQuoteValueRegex.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		// parts[1] = prefix (: ), parts[2] = value content, parts[3] = suffix (, } or ])
		// Convert \' to just ' and escape any double quotes in the value
		value := parts[2]
		value = strings.ReplaceAll(value, `\'`, `'`) // Unescape single quotes
		value = strings.ReplaceAll(value, `"`, `\"`) // Escape double quotes for JSON
		return parts[1] + `"` + value + `"` + parts[3]
	})

	// 7. Fix unquoted string values: {"key": value} -> {"key": "value"}
	// Skip known JSON literals (true, false, null are valid unquoted)
	result = unquotedValueRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := unquotedValueRegex.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		value := parts[2]
		// Don't quote JSON literals
		if value == "true" || value == "false" || value == "null" {
			return match
		}
		return parts[1] + `"` + value + `"` + parts[3]
	})

	// 8. Fix unquoted semver values: {"version": ^1.0.0} -> {"version": "^1.0.0"}
	// Common in LLM outputs when analyzing package.json dependencies
	result = unquotedSemverRegex.ReplaceAllString(result, `$1"$2"$3`)

	// 9. Fix truncated JSON (incomplete string at end)
	// If we have unbalanced quotes, try to close the string and structure
	result = fixTruncatedJSON(result)

	return result
}

// sanitizeControlChars escapes literal control characters inside JSON strings.
// LLMs often output raw tabs, newlines, etc. which are invalid in JSON.
func sanitizeControlChars(input string) string {
	var result strings.Builder
	result.Grow(len(input))

	inString := false
	escaped := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		if escaped {
			result.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' && inString {
			result.WriteByte(c)
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			result.WriteByte(c)
			continue
		}

		// Only sanitize control chars inside strings
		if inString {
			switch c {
			case '\t':
				result.WriteString(`\t`)
			case '\n':
				result.WriteString(`\n`)
			case '\r':
				result.WriteString(`\r`)
			case '\b':
				result.WriteString(`\b`)
			case '\f':
				result.WriteString(`\f`)
			default:
				// Escape other control characters (0x00-0x1F)
				if c < 0x20 {
					result.WriteString(fmt.Sprintf(`\u%04x`, c))
				} else {
					result.WriteByte(c)
				}
			}
		} else {
			result.WriteByte(c)
		}
	}

	return result.String()
}

// fixTruncatedJSON attempts to fix JSON that was truncated mid-string.
// Common with LLM output truncation.
func fixTruncatedJSON(input string) string {
	// Count quotes to detect imbalance
	quoteCount := 0
	escaped := false
	for _, c := range input {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			quoteCount++
		}
	}

	// If odd number of quotes, the string was truncated
	if quoteCount%2 != 0 {
		input = input + `"`
	}

	// Count braces and brackets to balance
	openBraces := strings.Count(input, "{") - strings.Count(input, "}")
	openBrackets := strings.Count(input, "[") - strings.Count(input, "]")

	// Add missing closing brackets (in reverse order for proper nesting)
	for i := 0; i < openBrackets; i++ {
		input = input + "]"
	}
	for i := 0; i < openBraces; i++ {
		input = input + "}"
	}

	return input
}

// cleanLLMResponse extracts JSON from LLM response text.
// Handles markdown code blocks.
func cleanLLMResponse(response string) string {
	response = strings.TrimSpace(response)

	// Strip markdown code blocks
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}
	// Also handle suffix if it exists, regardless of prefix
	response = strings.TrimSuffix(response, "```")

	return strings.TrimSpace(response)
}
