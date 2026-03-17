// Package skills provides embedded skill definitions for TaskWing.
//
// Skills are self-contained SKILL.md files. They are the single source of truth
// for prompt content used by both:
//   - Claude Code commands (generated as .claude/commands/taskwing/*.md with embedded content)
//   - CLI output (taskwing slash <cmd> for non-Claude-Code tools)
package skills

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed taskwing-*/SKILL.md
var content embed.FS

// Get returns the SKILL.md content for a given skill name (e.g., "next", "done", "plan").
// The name maps to the directory taskwing-<name>/SKILL.md.
func Get(name string) (string, error) {
	data, err := content.ReadFile(fmt.Sprintf("taskwing-%s/SKILL.md", name))
	if err != nil {
		return "", fmt.Errorf("skill %q not found: %w", name, err)
	}
	return string(data), nil
}

// GetBody returns only the body content (after YAML frontmatter) for a given skill.
// This strips the --- frontmatter block, returning just the prompt instructions.
func GetBody(name string) (string, error) {
	full, err := Get(name)
	if err != nil {
		return "", err
	}
	return stripFrontmatter(full), nil
}

// List returns all available skill names (without the "taskwing-" prefix).
func List() ([]string, error) {
	entries, err := content.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "taskwing-") {
			names = append(names, strings.TrimPrefix(e.Name(), "taskwing-"))
		}
	}
	return names, nil
}

// stripFrontmatter removes YAML frontmatter (--- delimited block) from the content.
func stripFrontmatter(s string) string {
	if !strings.HasPrefix(s, "---") {
		return s
	}
	// Find the closing ---
	rest := s[3:]
	_, body, found := strings.Cut(rest, "\n---")
	if !found {
		return s
	}
	// Skip past any trailing newline after ---
	body = strings.TrimPrefix(body, "\n")
	return body
}
