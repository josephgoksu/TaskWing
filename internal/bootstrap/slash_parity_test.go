package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSlashCommands_CrossAssistantDescriptionParity(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	assistants := []string{"claude", "codex", "opencode"}
	for _, ai := range assistants {
		if err := init.CreateSlashCommands(ai, false); err != nil {
			t.Fatalf("CreateSlashCommands(%s): %v", ai, err)
		}
	}

	for _, cmd := range SlashCommands {
		expected := cmd.Description

		paths := map[string]string{
			"claude":   filepath.Join(tmpDir, ".claude", "commands", cmd.BaseName+".md"),
			"codex":    filepath.Join(tmpDir, ".codex", "commands", cmd.BaseName+".md"),
			"opencode": filepath.Join(tmpDir, ".opencode", "commands", cmd.BaseName+".md"),
		}

		for ai, path := range paths {
			desc, err := readCommandDescription(path)
			if err != nil {
				t.Fatalf("read description %s (%s): %v", ai, cmd.BaseName, err)
			}
			if desc != expected {
				t.Fatalf("%s description mismatch for %s: got %q want %q", ai, cmd.BaseName, desc, expected)
			}
		}
	}
}

func readCommandDescription(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "description:") {
			continue
		}
		desc := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		desc = strings.Trim(desc, "\"")
		return desc, nil
	}

	return "", os.ErrNotExist
}
