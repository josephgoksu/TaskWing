package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractAndParseJSON extracts JSON from LLM responses and unmarshals it.
// Uses stream-based decoding to robustly ignore trailing text.
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
		// If basic decode fails, try one fallback: Unescape common chars if present
		if strings.Contains(jsonPart, "\\") {
			unescaped := strings.ReplaceAll(jsonPart, "\\\"", "\"")
			unescaped = strings.ReplaceAll(unescaped, "\\n", "\n")
			// Try decoding unescaped version
			dec2 := json.NewDecoder(strings.NewReader(unescaped))
			if err2 := dec2.Decode(&result); err2 == nil {
				return result, nil
			}
		}
		return result, fmt.Errorf("parse JSON: %w", err)
	}

	return result, nil
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
