package policy

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/tester"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/spf13/afero"
)

// TestResult represents the result of running a single OPA test.
type TestResult struct {
	// Name is the full name of the test (e.g., "data.taskwing.policy.test_deny_env_file")
	Name string `json:"name"`

	// Package is the Rego package the test is in
	Package string `json:"package"`

	// Passed indicates whether the test passed
	Passed bool `json:"passed"`

	// Failed indicates whether the test failed
	Failed bool `json:"failed"`

	// Skipped indicates whether the test was skipped
	Skipped bool `json:"skipped"`

	// Error contains the error message if the test errored
	Error string `json:"error,omitempty"`

	// Duration is how long the test took to run
	Duration time.Duration `json:"duration"`

	// Output contains any trace/print output from the test
	Output []string `json:"output,omitempty"`
}

// TestSummary summarizes the results of running multiple tests.
type TestSummary struct {
	// Passed is the number of tests that passed
	Passed int `json:"passed"`

	// Failed is the number of tests that failed
	Failed int `json:"failed"`

	// Skipped is the number of tests that were skipped
	Skipped int `json:"skipped"`

	// Errored is the number of tests that errored
	Errored int `json:"errored"`

	// Total is the total number of tests
	Total int `json:"total"`

	// Duration is the total time to run all tests
	Duration time.Duration `json:"duration"`

	// Results contains individual test results
	Results []*TestResult `json:"results"`
}

// TestRunner runs OPA policy tests.
type TestRunner struct {
	// fs is the filesystem to use
	fs afero.Fs

	// policiesDir is the directory containing policies and tests
	policiesDir string

	// workDir is the working directory for custom built-ins
	workDir string
}

// NewTestRunner creates a new test runner.
func NewTestRunner(fs afero.Fs, policiesDir, workDir string) *TestRunner {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	return &TestRunner{
		fs:          fs,
		policiesDir: policiesDir,
		workDir:     workDir,
	}
}

// Run executes all tests in the policies directory.
// Tests are Rego files with function names starting with "test_".
func (r *TestRunner) Run(ctx context.Context) (*TestSummary, error) {
	start := time.Now()

	// Ensure custom built-ins are registered
	builtinCtx := &BuiltinContext{
		WorkDir: r.workDir,
		Fs:      r.fs,
	}
	RegisterBuiltins(builtinCtx)

	// Load all modules from the policies directory
	modules, err := r.loadModules()
	if err != nil {
		return nil, fmt.Errorf("load modules: %w", err)
	}

	if len(modules) == 0 {
		return &TestSummary{
			Duration: time.Since(start),
			Results:  []*TestResult{},
		}, nil
	}

	// Compile all modules
	compiler := ast.NewCompiler()
	compiler.Compile(modules)
	if compiler.Failed() {
		var errMsgs []string
		for _, err := range compiler.Errors {
			errMsgs = append(errMsgs, err.Error())
		}
		return nil, fmt.Errorf("compile policies: %s", strings.Join(errMsgs, "; "))
	}

	// Create a test runner
	runner := tester.NewRunner().
		SetCompiler(compiler).
		SetModules(modules).
		EnableTracing(true).
		SetTimeout(30 * time.Second)

	// Run the tests
	ch, err := runner.RunTests(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("run tests: %w", err)
	}

	// Collect results
	var results []*TestResult
	for tr := range ch {
		result := &TestResult{
			Name:     tr.Name,
			Package:  tr.Package,
			Duration: tr.Duration,
		}

		if tr.Skip {
			result.Skipped = true
		} else if tr.Error != nil {
			result.Error = tr.Error.Error()
		} else if tr.Fail {
			result.Failed = true
		} else {
			result.Passed = true
		}

		// Collect trace output (note/print statements)
		for _, evt := range tr.Trace {
			if evt.Op == topdown.NoteOp && evt.Message != "" {
				result.Output = append(result.Output, evt.Message)
			}
		}

		results = append(results, result)
	}

	// Build summary
	summary := &TestSummary{
		Duration: time.Since(start),
		Results:  results,
	}

	for _, r := range results {
		summary.Total++
		if r.Passed {
			summary.Passed++
		} else if r.Failed {
			summary.Failed++
		} else if r.Skipped {
			summary.Skipped++
		} else if r.Error != "" {
			summary.Errored++
		}
	}

	return summary, nil
}

// loadModules loads all .rego files from the policies directory.
func (r *TestRunner) loadModules() (map[string]*ast.Module, error) {
	modules := make(map[string]*ast.Module)

	exists, err := afero.DirExists(r.fs, r.policiesDir)
	if err != nil {
		return nil, err
	}
	if !exists {
		return modules, nil
	}

	err = afero.Walk(r.fs, r.policiesDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".rego") {
			return nil
		}

		content, err := afero.ReadFile(r.fs, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		// Parse the module
		module, err := ast.ParseModule(path, string(content))
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		// Use relative path as module name for cleaner output
		relPath, _ := filepath.Rel(r.policiesDir, path)
		if relPath == "" {
			relPath = path
		}

		modules[relPath] = module
		return nil
	})

	return modules, err
}

// RunBundle runs tests from a bundle.
// This is useful for testing policies packaged in OPA bundles.
func (r *TestRunner) RunBundle(ctx context.Context, b *bundle.Bundle) (*TestSummary, error) {
	start := time.Now()

	// Ensure custom built-ins are registered
	builtinCtx := &BuiltinContext{
		WorkDir: r.workDir,
		Fs:      r.fs,
	}
	RegisterBuiltins(builtinCtx)

	// Extract modules from bundle
	modules := make(map[string]*ast.Module)
	for _, mf := range b.Modules {
		modules[mf.Path] = mf.Parsed
	}

	if len(modules) == 0 {
		return &TestSummary{
			Duration: time.Since(start),
			Results:  []*TestResult{},
		}, nil
	}

	// Compile all modules
	compiler := ast.NewCompiler()
	compiler.Compile(modules)
	if compiler.Failed() {
		var errMsgs []string
		for _, err := range compiler.Errors {
			errMsgs = append(errMsgs, err.Error())
		}
		return nil, fmt.Errorf("compile policies: %s", strings.Join(errMsgs, "; "))
	}

	// Create a test runner
	runner := tester.NewRunner().
		SetCompiler(compiler).
		SetModules(modules).
		EnableTracing(true).
		SetTimeout(30 * time.Second)

	// Run the tests
	ch, err := runner.RunTests(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("run tests: %w", err)
	}

	// Collect results
	var results []*TestResult
	for tr := range ch {
		result := &TestResult{
			Name:     tr.Name,
			Package:  tr.Package,
			Duration: tr.Duration,
		}

		if tr.Skip {
			result.Skipped = true
		} else if tr.Error != nil {
			result.Error = tr.Error.Error()
		} else if tr.Fail {
			result.Failed = true
		} else {
			result.Passed = true
		}

		results = append(results, result)
	}

	// Build summary
	summary := &TestSummary{
		Duration: time.Since(start),
		Results:  results,
	}

	for _, r := range results {
		summary.Total++
		if r.Passed {
			summary.Passed++
		} else if r.Failed {
			summary.Failed++
		} else if r.Skipped {
			summary.Skipped++
		} else if r.Error != "" {
			summary.Errored++
		}
	}

	return summary, nil
}

// HasTests returns true if there are any test files in the policies directory.
func (r *TestRunner) HasTests() (bool, error) {
	exists, err := afero.DirExists(r.fs, r.policiesDir)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	hasTests := false
	err = afero.Walk(r.fs, r.policiesDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), "_test.rego") {
			hasTests = true
			return filepath.SkipDir // Stop walking by skipping the rest
		}
		return nil
	})

	// filepath.SkipDir is expected when we find a test file
	if err == filepath.SkipDir {
		err = nil
	}

	return hasTests, err
}

// FormatSummary returns a human-readable summary of test results.
func (s *TestSummary) FormatSummary() string {
	var sb strings.Builder

	if s.Total == 0 {
		sb.WriteString("No tests found.\n")
		return sb.String()
	}

	// Summary line
	sb.WriteString(fmt.Sprintf("\n%d tests, %d passed", s.Total, s.Passed))
	if s.Failed > 0 {
		sb.WriteString(fmt.Sprintf(", %d failed", s.Failed))
	}
	if s.Errored > 0 {
		sb.WriteString(fmt.Sprintf(", %d errored", s.Errored))
	}
	if s.Skipped > 0 {
		sb.WriteString(fmt.Sprintf(", %d skipped", s.Skipped))
	}
	sb.WriteString(fmt.Sprintf(" in %s\n", s.Duration.Round(time.Millisecond)))

	return sb.String()
}

// AllPassed returns true if all tests passed (no failures or errors).
func (s *TestSummary) AllPassed() bool {
	return s.Failed == 0 && s.Errored == 0
}
