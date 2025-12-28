package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractAndParseJSON extracts JSON from LLM responses and unmarshals it.
// Handles: markdown code blocks, JSON embedded in text, arrays, and objects.
// This is the SINGLE implementation - all JSON parsing should use this.
func ExtractAndParseJSON[T any](response string) (T, error) {
	var result T

	cleaned := cleanLLMResponse(response)
	if cleaned == "" {
		return result, fmt.Errorf("no JSON found in response")
	}

	if err := json.Unmarshal([]byte(cleaned), &result); err == nil {
		return result, nil
	} else {
		lastErr := err

		// Some models return JSON as a quoted string (escaped JSON).
		var asString string
		if err := json.Unmarshal([]byte(cleaned), &asString); err == nil {
			asString = strings.TrimSpace(asString)
			if asString != "" {
				if err := json.Unmarshal([]byte(asString), &result); err == nil {
					return result, nil
				}
				if extracted := extractJSONFromText(asString); extracted != "" {
					if err := json.Unmarshal([]byte(extracted), &result); err == nil {
						return result, nil
					} else {
						lastErr = err
					}
				}
			}
		}

		// If there is leading junk/backslashes, try slicing from first JSON token.
		if idx := strings.IndexAny(cleaned, "{["); idx > 0 {
			sliced := strings.TrimSpace(cleaned[idx:])
			if err := json.Unmarshal([]byte(sliced), &result); err == nil {
				return result, nil
			} else {
				lastErr = err
			}
			if extracted := extractJSONFromText(sliced); extracted != "" {
				if err := json.Unmarshal([]byte(extracted), &result); err == nil {
					return result, nil
				} else {
					lastErr = err
				}
			}
		}

		// Attempt basic unescape for common sequences.
		if strings.Contains(cleaned, "\\") {
			unescaped := strings.ReplaceAll(cleaned, "\\n", "\n")
			unescaped = strings.ReplaceAll(unescaped, "\\\"", "\"")
			unescaped = strings.ReplaceAll(unescaped, "\\\\", "\\")
			unescaped = strings.TrimSpace(unescaped)
			if extracted := extractJSONFromText(unescaped); extracted != "" {
				if err := json.Unmarshal([]byte(extracted), &result); err == nil {
					return result, nil
				} else {
					lastErr = err
				}
			}
		}

		return result, fmt.Errorf("parse JSON: %w", lastErr)
	}

	return result, nil
}

// cleanLLMResponse extracts JSON from LLM response text.
// Handles markdown code blocks and embedded JSON objects/arrays.
func cleanLLMResponse(response string) string {
	response = strings.TrimSpace(response)

	// Strip markdown code blocks
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Try direct parse first (already clean JSON)
	if isValidJSON(response) {
		return response
	}

	// Extract JSON object or array from surrounding text
	return extractJSONFromText(response)
}

// extractJSONFromText finds and extracts JSON from arbitrary text.
func extractJSONFromText(text string) string {
	// Try object first
	objStart := strings.Index(text, "{")
	objEnd := strings.LastIndex(text, "}")

	// Try array
	arrStart := strings.Index(text, "[")
	arrEnd := strings.LastIndex(text, "]")

	// Determine which comes first and is valid
	if objStart >= 0 && objEnd > objStart {
		obj := text[objStart : objEnd+1]
		if isValidJSON(obj) {
			return obj
		}
	}

	if arrStart >= 0 && arrEnd > arrStart {
		arr := text[arrStart : arrEnd+1]
		if isValidJSON(arr) {
			return arr
		}
	}

	// Fallback: return object extraction even if invalid (let caller handle error)
	if objStart >= 0 && objEnd > objStart {
		return text[objStart : objEnd+1]
	}

	return ""
}

// isValidJSON checks if a string is valid JSON.
func isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}
