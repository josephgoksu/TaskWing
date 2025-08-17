/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package types

import "fmt"

// MCPError provides structured error information for MCP responses
type MCPError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewMCPError creates a new structured MCP error
func NewMCPError(code string, message string, details map[string]interface{}) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Details: details,
	}
}