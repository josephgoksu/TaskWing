package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The skill teaches agents the CLI, so every command and flag it names in a
// taskwing command line must exist, or agents run commands that fail.
func TestSkillMatchesCLI(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "skills", "taskwing", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	skill := string(raw)

	// Agents skip a skill whose frontmatter is not the very first thing, or
	// lacks a name or a description (Agent Skills spec: description <= 1024).
	front, _, ok := strings.Cut(strings.TrimPrefix(skill, "---\n"), "\n---\n")
	if !strings.HasPrefix(skill, "---\n") || !ok {
		t.Fatal("SKILL.md must start with a --- frontmatter block")
	}
	if !strings.Contains(front, "\nname: taskwing") && !strings.HasPrefix(front, "name: taskwing") {
		t.Error("frontmatter needs name: taskwing")
	}
	desc := regexp.MustCompile(`(?m)^description: (.+)$`).FindStringSubmatch(front)
	if desc == nil || len(desc[1]) > 1024 {
		t.Errorf("frontmatter needs a description of at most 1024 characters")
	}

	var help bytes.Buffer
	usage(&help)
	known := help.String() + " taskwing help --check --clean --version "
	cmdRe := regexp.MustCompile("taskwing ([a-z]+)")
	flagRe := regexp.MustCompile(`(?:^|\s)(--[a-z][a-z-]*)`)
	for _, line := range strings.Split(skill, "\n") {
		if !strings.Contains(line, "taskwing ") || strings.Contains(line, "brew ") || strings.Contains(line, "go install") || strings.Contains(line, "gh skill") {
			continue
		}
		for _, m := range cmdRe.FindAllStringSubmatch(line, -1) {
			if !strings.Contains(known, "taskwing "+m[1]) {
				t.Errorf("the skill uses taskwing %s, which is not a command: %s", m[1], line)
			}
		}
		for _, m := range flagRe.FindAllStringSubmatch(line, -1) {
			if !strings.Contains(known, m[1]+" ") && !strings.Contains(known, m[1]+"]") {
				t.Errorf("the skill uses %s, which no command has: %s", m[1], line)
			}
		}
	}
}
