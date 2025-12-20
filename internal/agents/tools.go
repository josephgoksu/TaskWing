/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package agents

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ReadFileTool reads file contents with optional line limits
type ReadFileTool struct {
	BasePath string
}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. Args: path (string), max_lines (int, optional)"
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path argument required")
	}

	fullPath := filepath.Join(t.BasePath, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	maxLines := 500
	if ml, ok := args["max_lines"].(float64); ok {
		maxLines = int(ml)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, fmt.Sprintf("... truncated (%d lines total)", len(lines)))
	}

	return strings.Join(lines, "\n"), nil
}

// GrepTool searches for patterns in files
type GrepTool struct {
	BasePath string
}

func (t *GrepTool) Name() string { return "grep_search" }

func (t *GrepTool) Description() string {
	return "Search for a pattern in files. Args: pattern (string), path (string, optional), include (string glob, optional)"
}

func (t *GrepTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern argument required")
	}

	searchPath := t.BasePath
	if p, ok := args["path"].(string); ok {
		searchPath = filepath.Join(t.BasePath, p)
	}

	grepArgs := []string{"-r", "-n", "-I", "--include=*.go", "--include=*.ts", "--include=*.js", "--include=*.json", "--include=*.yaml", "--include=*.yml", "--include=*.md", pattern, searchPath}
	if include, ok := args["include"].(string); ok {
		grepArgs = []string{"-r", "-n", "-I", "--include=" + include, pattern, searchPath}
	}

	cmd := exec.CommandContext(ctx, "grep", grepArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run() // grep returns non-zero if no matches, which is fine

	// Limit output
	output := stdout.String()
	lines := strings.Split(output, "\n")
	if len(lines) > 50 {
		lines = lines[:50]
		lines = append(lines, fmt.Sprintf("... truncated (%d matches)", len(lines)))
	}

	return strings.Join(lines, "\n"), nil
}

// ListDirTool lists directory contents
type ListDirTool struct {
	BasePath string
}

func (t *ListDirTool) Name() string { return "list_dir" }

func (t *ListDirTool) Description() string {
	return "List contents of a directory. Args: path (string), max_depth (int, optional)"
}

func (t *ListDirTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	path := t.BasePath
	if p, ok := args["path"].(string); ok {
		path = filepath.Join(t.BasePath, p)
	}

	maxDepth := 2
	if md, ok := args["max_depth"].(float64); ok {
		maxDepth = int(md)
	}

	var result []string
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		relPath, _ := filepath.Rel(path, p)
		depth := strings.Count(relPath, string(os.PathSeparator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			result = append(result, relPath+"/")
		} else {
			result = append(result, relPath)
		}

		if len(result) > 100 {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return strings.Join(result, "\n"), nil
}

// ExecCommandTool executes shell commands (limited)
type ExecCommandTool struct {
	BasePath        string
	AllowedCommands []string // e.g., ["git", "cat", "head", "tail"]
}

func (t *ExecCommandTool) Name() string { return "exec_command" }

func (t *ExecCommandTool) Description() string {
	return "Execute a shell command. Args: command (string), args ([]string)"
}

func (t *ExecCommandTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	command, ok := args["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command argument required")
	}

	// Security: only allow specific commands
	allowed := false
	for _, c := range t.AllowedCommands {
		if c == command {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("command '%s' not allowed", command)
	}

	var cmdArgs []string
	if a, ok := args["args"].([]any); ok {
		for _, arg := range a {
			if s, ok := arg.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}

	cmd := exec.CommandContext(ctx, command, cmdArgs...)
	cmd.Dir = t.BasePath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// CreateDefaultTools returns the standard set of tools for agents
func CreateDefaultTools(basePath string) []Tool {
	return []Tool{
		&ReadFileTool{BasePath: basePath},
		&GrepTool{BasePath: basePath},
		&ListDirTool{BasePath: basePath},
		&ExecCommandTool{
			BasePath:        basePath,
			AllowedCommands: []string{"git", "cat", "head", "tail", "wc"},
		},
	}
}
