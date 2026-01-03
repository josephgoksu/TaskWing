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
// Handles: missing commas, trailing commas, single quotes for keys and values.
// Uses pre-compiled regexes for performance.
func repairJSON(input string) string {
	result := input

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

	return result
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
