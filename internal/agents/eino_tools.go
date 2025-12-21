/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package agents provides Eino-compatible tools for ReAct agents.
These tools implement the tool.BaseTool interface from CloudWeGo Eino,
enabling LLMs to dynamically explore codebases during analysis.
*/
package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// =============================================================================
// EinoReadFileTool - Read file contents with line limits
// =============================================================================

// EinoReadFileTool implements tool.InvokableTool for reading file contents
type EinoReadFileTool struct {
	basePath string
}

// NewEinoReadFileTool creates a new Eino-compatible read file tool
func NewEinoReadFileTool(basePath string) *EinoReadFileTool {
	return &EinoReadFileTool{basePath: basePath}
}

// Info returns the tool metadata for LLM function calling
func (t *EinoReadFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "read_file",
		Desc: `Read the contents of a file from the codebase.
Use this to examine source code, configurations, documentation, or any text file.
The path should be relative to the project root.

Examples:
- read_file(path="README.md") - read project readme
- read_file(path="internal/api/handler.go", max_lines=100) - read first 100 lines
- read_file(path="package.json") - read package manifest`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {
				Type:     "string",
				Desc:     "Relative path to the file from project root",
				Required: true,
			},
			"max_lines": {
				Type:     "integer",
				Desc:     "Maximum number of lines to return (default: 500)",
				Required: false,
			},
		}),
	}, nil
}

// readFileArgs holds parsed arguments for read_file
type readFileArgs struct {
	Path     string `json:"path"`
	MaxLines int    `json:"max_lines,omitempty"`
}

// InvokableRun executes the read_file operation
func (t *EinoReadFileTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params readFileArgs
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if params.Path == "" {
		return "", fmt.Errorf("path argument is required")
	}

	// Security: prevent path traversal
	cleanPath := filepath.Clean(params.Path)
	if strings.HasPrefix(cleanPath, "..") {
		return "", fmt.Errorf("path traversal not allowed")
	}

	fullPath := filepath.Join(t.basePath, cleanPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", cleanPath, err)
	}

	maxLines := 500
	if params.MaxLines > 0 {
		maxLines = params.MaxLines
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, fmt.Sprintf("\n... [truncated: showing %d of %d lines]", maxLines, totalLines))
	}

	return strings.Join(lines, "\n"), nil
}

// Ensure interface compliance
var _ tool.InvokableTool = (*EinoReadFileTool)(nil)

// =============================================================================
// EinoGrepTool - Search for patterns across files
// =============================================================================

// EinoGrepTool implements tool.InvokableTool for searching patterns in files
type EinoGrepTool struct {
	basePath string
}

// NewEinoGrepTool creates a new Eino-compatible grep tool
func NewEinoGrepTool(basePath string) *EinoGrepTool {
	return &EinoGrepTool{basePath: basePath}
}

// Info returns the tool metadata for LLM function calling
func (t *EinoGrepTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "grep_search",
		Desc: `Search for a text pattern across files in the codebase.
Returns matching lines with file paths and line numbers.
Use this to find function definitions, imports, usages, or any text pattern.

Examples:
- grep_search(pattern="func main") - find main functions
- grep_search(pattern="TODO", include="*.go") - find TODOs in Go files
- grep_search(pattern="import.*react", path="src") - find React imports in src/`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {
				Type:     "string",
				Desc:     "Text pattern to search for (supports basic regex)",
				Required: true,
			},
			"path": {
				Type:     "string",
				Desc:     "Subdirectory to search in (default: entire project)",
				Required: false,
			},
			"include": {
				Type:     "string",
				Desc:     "File glob pattern to include (e.g., '*.go', '*.ts')",
				Required: false,
			},
		}),
	}, nil
}

// grepArgs holds parsed arguments for grep_search
type grepArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
	Include string `json:"include,omitempty"`
}

// InvokableRun executes the grep_search operation
func (t *EinoGrepTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params grepArgs
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if params.Pattern == "" {
		return "", fmt.Errorf("pattern argument is required")
	}

	searchPath := t.basePath
	if params.Path != "" {
		cleanPath := filepath.Clean(params.Path)
		if strings.HasPrefix(cleanPath, "..") {
			return "", fmt.Errorf("path traversal not allowed")
		}
		searchPath = filepath.Join(t.basePath, cleanPath)
	}

	// Build grep arguments
	grepArgs := []string{"-r", "-n", "-I", "--color=never"}

	// Add include patterns
	if params.Include != "" {
		grepArgs = append(grepArgs, "--include="+params.Include)
	} else {
		// Default: search common code files
		grepArgs = append(grepArgs,
			"--include=*.go", "--include=*.ts", "--include=*.tsx",
			"--include=*.js", "--include=*.jsx", "--include=*.json",
			"--include=*.yaml", "--include=*.yml", "--include=*.md",
			"--include=*.py", "--include=*.rs", "--include=*.toml",
		)
	}

	// Exclude common noise directories
	grepArgs = append(grepArgs,
		"--exclude-dir=node_modules", "--exclude-dir=vendor",
		"--exclude-dir=.git", "--exclude-dir=dist", "--exclude-dir=build",
	)

	grepArgs = append(grepArgs, params.Pattern, searchPath)

	cmd := exec.CommandContext(ctx, "grep", grepArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// grep returns exit code 1 for no matches, which is not an error
	_ = cmd.Run()

	output := stdout.String()
	if output == "" {
		return "No matches found.", nil
	}

	// Limit output to prevent overwhelming context
	lines := strings.Split(output, "\n")
	if len(lines) > 50 {
		lines = lines[:50]
		lines = append(lines, fmt.Sprintf("\n... [truncated: showing 50 of %d+ matches]", len(lines)))
	}

	// Make paths relative to basePath for cleaner output
	var result []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		relativeLine := strings.TrimPrefix(line, t.basePath+"/")
		result = append(result, relativeLine)
	}

	return strings.Join(result, "\n"), nil
}

// Ensure interface compliance
var _ tool.InvokableTool = (*EinoGrepTool)(nil)

// =============================================================================
// EinoListDirTool - List directory contents
// =============================================================================

// EinoListDirTool implements tool.InvokableTool for listing directory contents
type EinoListDirTool struct {
	basePath string
}

// NewEinoListDirTool creates a new Eino-compatible list directory tool
func NewEinoListDirTool(basePath string) *EinoListDirTool {
	return &EinoListDirTool{basePath: basePath}
}

// Info returns the tool metadata for LLM function calling
func (t *EinoListDirTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "list_dir",
		Desc: `List contents of a directory to understand project structure.
Shows files and subdirectories with their types.
Use this to explore the codebase and find relevant files.

Examples:
- list_dir() - list project root
- list_dir(path="internal") - list internal/ directory
- list_dir(path="src/components", max_depth=1) - shallow listing`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {
				Type:     "string",
				Desc:     "Subdirectory to list (default: project root)",
				Required: false,
			},
			"max_depth": {
				Type:     "integer",
				Desc:     "Maximum depth to traverse (default: 2)",
				Required: false,
			},
		}),
	}, nil
}

// listDirArgs holds parsed arguments for list_dir
type listDirArgs struct {
	Path     string `json:"path,omitempty"`
	MaxDepth int    `json:"max_depth,omitempty"`
}

// InvokableRun executes the list_dir operation
func (t *EinoListDirTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params listDirArgs
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	targetPath := t.basePath
	if params.Path != "" {
		cleanPath := filepath.Clean(params.Path)
		if strings.HasPrefix(cleanPath, "..") {
			return "", fmt.Errorf("path traversal not allowed")
		}
		targetPath = filepath.Join(t.basePath, cleanPath)
	}

	maxDepth := 2
	if params.MaxDepth > 0 {
		maxDepth = params.MaxDepth
	}

	// Directories to skip
	skipDirs := map[string]bool{
		"node_modules": true, "vendor": true, ".git": true,
		"dist": true, "build": true, "__pycache__": true,
		".next": true, ".nuxt": true, "coverage": true,
	}

	var result []string
	itemCount := 0
	maxItems := 150

	err := filepath.WalkDir(targetPath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if itemCount >= maxItems {
			return filepath.SkipAll
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

		// Skip hidden files/dirs (except .github, .env.example)
		if strings.HasPrefix(name, ".") && name != ".github" && name != ".env.example" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip noise directories
		if d.IsDir() && skipDirs[name] {
			return filepath.SkipDir
		}

		// Format output with indentation
		indent := strings.Repeat("  ", depth)
		if d.IsDir() {
			result = append(result, fmt.Sprintf("%s📁 %s/", indent, name))
		} else {
			// Show file size for context
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

// formatSize returns a human-readable file size
func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("(%dB)", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("(%.1fKB)", float64(bytes)/1024)
	}
	return fmt.Sprintf("(%.1fMB)", float64(bytes)/(1024*1024))
}

// Ensure interface compliance
var _ tool.InvokableTool = (*EinoListDirTool)(nil)

// =============================================================================
// EinoExecTool - Execute allowed shell commands
// =============================================================================

// EinoExecTool implements tool.InvokableTool for executing whitelisted commands
type EinoExecTool struct {
	basePath        string
	allowedCommands map[string]bool
}

// NewEinoExecTool creates a new Eino-compatible exec tool with default allowed commands
func NewEinoExecTool(basePath string) *EinoExecTool {
	return &EinoExecTool{
		basePath: basePath,
		allowedCommands: map[string]bool{
			"git":  true, // git log, git show, git diff
			"head": true, // head of files
			"tail": true, // tail of files
			"wc":   true, // word/line count
			"find": true, // find files
		},
	}
}

// Info returns the tool metadata for LLM function calling
func (t *EinoExecTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "exec_command",
		Desc: `Execute a whitelisted shell command for deeper analysis.
Allowed commands: git, head, tail, wc, find.
Use this for git history, file line counts, or finding specific files.

Examples:
- exec_command(command="git", args=["log", "--oneline", "-20"]) - recent commits
- exec_command(command="git", args=["show", "HEAD:go.mod"]) - view file at commit
- exec_command(command="wc", args=["-l", "internal/api/*.go"]) - count lines
- exec_command(command="find", args=[".", "-name", "*.test.ts"]) - find test files`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {
				Type:     "string",
				Desc:     "Command to execute (git, head, tail, wc, find)",
				Required: true,
			},
			"args": {
				Type:     "array",
				Desc:     "Arguments to pass to the command",
				Required: false,
			},
		}),
	}, nil
}

// execArgs holds parsed arguments for exec_command
type execArgs struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// InvokableRun executes the exec_command operation
func (t *EinoExecTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params execArgs
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if params.Command == "" {
		return "", fmt.Errorf("command argument is required")
	}

	// Security: only allow whitelisted commands
	if !t.allowedCommands[params.Command] {
		allowed := make([]string, 0, len(t.allowedCommands))
		for cmd := range t.allowedCommands {
			allowed = append(allowed, cmd)
		}
		return "", fmt.Errorf("command '%s' not allowed. Allowed: %v", params.Command, allowed)
	}

	cmd := exec.CommandContext(ctx, params.Command, params.Args...)
	cmd.Dir = t.basePath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Include stderr in error for debugging
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s: %s", err, stderr.String())
		}
		return "", err
	}

	output := stdout.String()

	// Limit output size
	if len(output) > 10000 {
		output = output[:10000] + "\n... [truncated: output too large]"
	}

	return output, nil
}

// Ensure interface compliance
var _ tool.InvokableTool = (*EinoExecTool)(nil)

// =============================================================================
// Helper: Create all Eino tools for an agent
// =============================================================================

// CreateEinoTools returns all Eino-compatible tools for a given base path
func CreateEinoTools(basePath string) []tool.InvokableTool {
	return []tool.InvokableTool{
		NewEinoReadFileTool(basePath),
		NewEinoGrepTool(basePath),
		NewEinoListDirTool(basePath),
		NewEinoExecTool(basePath),
	}
}
