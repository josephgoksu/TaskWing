package runner

import (
	"bytes"
	"fmt"
	"text/template"
)

// RenderTemplate renders a Go text/template string with the given variables.
// This is used to render system prompt templates (e.g., SystemPromptClarifyingAgent)
// with input variables before sending them as prompts to AI CLI runners.
func RenderTemplate(templateStr string, vars map[string]any) (string, error) {
	tmpl, err := template.New("prompt").Option("missingkey=zero").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}
