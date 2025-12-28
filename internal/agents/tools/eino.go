/*
Package tools provides Eino-compatible tools for ReAct agents.
*/
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/patterns"
)

var errPathTraversal = errors.New("path traversal not allowed")

func validateRelativePath(path string) (string, error) {
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "..") {
		return "", errPathTraversal
	}
	return cleanPath, nil
}

// CreateEinoTools returns all Eino-compatible tools for a given base path.
func CreateEinoTools(basePath string) []tool.InvokableTool {
	return []tool.InvokableTool{
		NewReadFileTool(basePath),
		NewGrepTool(basePath),
		NewListDirTool(basePath),
		NewExecTool(basePath),
	}
}

// =============================================================================
// ReadFileTool
// =============================================================================

type ReadFileTool struct{ basePath string }

func NewReadFileTool(basePath string) *ReadFileTool { return &ReadFileTool{basePath: basePath} }

func (t *ReadFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "read_file",
		Desc: `Read file contents with line numbers. Use this for gathering evidence (snippets with line numbers). Path is relative to project root.`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path":      {Type: "string", Desc: "Relative path to file", Required: true},
			"max_lines": {Type: "integer", Desc: "Max lines (default: 500)", Required: false},
		}),
	}, nil
}

func (t *ReadFileTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path     string `json:"path"`
		MaxLines int    `json:"max_lines,omitempty"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("path argument is required")
	}
	cleanPath, err := validateRelativePath(args.Path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(filepath.Join(t.basePath, cleanPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Sprintf("File not found: %s", cleanPath), nil
		}
		return "", fmt.Errorf("read file %s: %w", cleanPath, err)
	}
	maxLines := 500
	if args.MaxLines > 0 {
		maxLines = args.MaxLines
	}
	lines := strings.Split(string(content), "\n")
	total := len(lines)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	// Add line numbers (1-indexed) for evidence extraction
	var numbered []string
	for i, line := range lines {
		numbered = append(numbered, fmt.Sprintf("%4d\t%s", i+1, line))
	}
	if total > maxLines {
		numbered = append(numbered, fmt.Sprintf("\n... [truncated: showing %d of %d lines]", maxLines, total))
	}
	return strings.Join(numbered, "\n"), nil
}

var _ tool.InvokableTool = (*ReadFileTool)(nil)

// =============================================================================
// GrepTool
// =============================================================================

type GrepTool struct{ basePath string }

func NewGrepTool(basePath string) *GrepTool { return &GrepTool{basePath: basePath} }

func (t *GrepTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "grep_search",
		Desc: `Search for text pattern across files. Returns matching lines with file:line.`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {Type: "string", Desc: "Pattern to search", Required: true},
			"path":    {Type: "string", Desc: "Subdirectory to search", Required: false},
			"include": {Type: "string", Desc: "File glob pattern", Required: false},
		}),
	}, nil
}

func (t *GrepTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path,omitempty"`
		Include string `json:"include,omitempty"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.Pattern == "" {
		return "", fmt.Errorf("pattern argument is required")
	}

	searchPath := t.basePath
	if args.Path != "" {
		cleanPath, err := validateRelativePath(args.Path)
		if err != nil {
			return "", err
		}
		searchPath = filepath.Join(t.basePath, cleanPath)
	}

	grepArgs := []string{"-r", "-n", "-I", "--color=never"}
	if args.Include != "" {
		grepArgs = append(grepArgs, "--include="+args.Include)
	} else {
		grepArgs = append(grepArgs, "--include=*.go", "--include=*.ts", "--include=*.tsx",
			"--include=*.js", "--include=*.jsx", "--include=*.json",
			"--include=*.yaml", "--include=*.yml", "--include=*.md",
			"--include=*.py", "--include=*.rs", "--include=*.toml")
	}
	grepArgs = append(grepArgs, "--exclude-dir=node_modules", "--exclude-dir=vendor",
		"--exclude-dir=.git", "--exclude-dir=dist", "--exclude-dir=build")
	grepArgs = append(grepArgs, args.Pattern, searchPath)

	cmd := exec.CommandContext(ctx, "grep", grepArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run()

	output := stdout.String()
	if output == "" {
		return "No matches found.", nil
	}

	lines := strings.Split(output, "\n")
	if len(lines) > 50 {
		lines = lines[:50]
		lines = append(lines, fmt.Sprintf("\n... [truncated: 50+ matches]"))
	}

	var result []string
	for _, line := range lines {
		if line != "" {
			result = append(result, strings.TrimPrefix(line, t.basePath+"/"))
		}
	}
	return strings.Join(result, "\n"), nil
}

var _ tool.InvokableTool = (*GrepTool)(nil)

// =============================================================================
// ListDirTool
// =============================================================================

type ListDirTool struct{ basePath string }

func NewListDirTool(basePath string) *ListDirTool { return &ListDirTool{basePath: basePath} }

func (t *ListDirTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "list_dir",
		Desc: `List directory contents to understand project structure.`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path":      {Type: "string", Desc: "Subdirectory to list", Required: false},
			"max_depth": {Type: "integer", Desc: "Max depth (default: 2)", Required: false},
		}),
	}, nil
}

func (t *ListDirTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path     string `json:"path,omitempty"`
		MaxDepth int    `json:"max_depth,omitempty"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	targetPath := t.basePath
	if args.Path != "" {
		cleanPath, err := validateRelativePath(args.Path)
		if err != nil {
			return "", err
		}
		targetPath = filepath.Join(t.basePath, cleanPath)
	}

	maxDepth := 2
	if args.MaxDepth > 0 {
		maxDepth = args.MaxDepth
	}

	var result []string
	itemCount := 0
	maxItems := 150

	err := filepath.WalkDir(targetPath, func(p string, d os.DirEntry, err error) error {
		if err != nil || itemCount >= maxItems {
			if itemCount >= maxItems {
				return filepath.SkipAll
			}
			return nil
		}
		relPath, _ := filepath.Rel(targetPath, p)
		if relPath == "." {
			return nil
		}
		depth := strings.Count(relPath, string(os.PathSeparator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") && name != ".github" && name != ".env.example" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() && patterns.IgnoredDirs[name] {
			return filepath.SkipDir
		}
		indent := strings.Repeat("  ", depth)
		if d.IsDir() {
			result = append(result, fmt.Sprintf("%s📁 %s/", indent, name))
		} else {
			info, _ := d.Info()
			size := ""
			if info != nil {
				size = formatSize(info.Size())
			}
			result = append(result, fmt.Sprintf("%s📄 %s %s", indent, name, size))
		}
		itemCount++
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("walk directory: %w", err)
	}
	if len(result) == 0 {
		return "Directory is empty or does not exist.", nil
	}
	if itemCount >= maxItems {
		result = append(result, fmt.Sprintf("\n... [truncated: showing %d items]", maxItems))
	}
	return strings.Join(result, "\n"), nil
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("(%dB)", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("(%.1fKB)", float64(bytes)/1024)
	}
	return fmt.Sprintf("(%.1fMB)", float64(bytes)/(1024*1024))
}

var _ tool.InvokableTool = (*ListDirTool)(nil)

// =============================================================================
// ExecTool
// =============================================================================

type ExecTool struct {
	basePath        string
	allowedCommands map[string]bool
}

func NewExecTool(basePath string) *ExecTool {
	return &ExecTool{
		basePath:        basePath,
		allowedCommands: map[string]bool{"git": true, "head": true, "tail": true, "wc": true, "find": true},
	}
}

func (t *ExecTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "exec_command",
		Desc: `Execute a whitelisted command. ONLY for git history queries. Use read_file for file contents.
IMPORTANT: command must be the binary name ONLY (e.g. "git"), args must be a separate array.
Shell pipes (|) and shell syntax are NOT supported.
Example: {"command": "git", "args": ["log", "--oneline", "-20"]}
Allowed commands: git, head, tail, wc, find`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {Type: "string", Desc: "Binary name only (git, head, tail, wc, find). NOT a shell command.", Required: true},
			"args":    {Type: "array", ElemInfo: &schema.ParameterInfo{Type: "string"}, Desc: "Command arguments as separate strings", Required: false},
		}),
	}, nil
}

func (t *ExecTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Command string   `json:"command"`
		Args    []string `json:"args,omitempty"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.Command == "" {
		return "", fmt.Errorf("command argument is required")
	}
	if !t.allowedCommands[args.Command] {
		allowed := make([]string, 0, len(t.allowedCommands))
		for cmd := range t.allowedCommands {
			allowed = append(allowed, cmd)
		}
		return "", fmt.Errorf("command '%s' not allowed. Allowed: %v", args.Command, allowed)
	}

	cmd := exec.CommandContext(ctx, args.Command, args.Args...)
	cmd.Dir = t.basePath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s: %s", err, stderr.String())
		}
		return "", err
	}

	output := stdout.String()
	if len(output) > 10000 {
		output = output[:10000] + "\n... [truncated: output too large]"
	}
	return output, nil
}

var _ tool.InvokableTool = (*ExecTool)(nil)
