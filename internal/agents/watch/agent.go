/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com

Package watch provides the WatchAgent for continuous codebase monitoring.
It watches for file changes and triggers appropriate agents for incremental analysis.
*/
package watch

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/josephgoksu/TaskWing/internal/agents/analysis"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/patterns"
)

// FileCategory represents the type of file for routing purposes
type FileCategory string

const (
	FileCategoryDocs   FileCategory = "docs"   // *.md, docs/*
	FileCategoryCode   FileCategory = "code"   // *.go, *.ts, *.js, etc.
	FileCategoryDeps   FileCategory = "deps"   // go.mod, package.json
	FileCategoryConfig FileCategory = "config" // *.yaml, *.json configs
	FileCategoryGit    FileCategory = "git"    // .git/HEAD changes
	FileCategoryIgnore FileCategory = "ignore" // Files to skip
)

// FileChangeEvent represents a batched file change
type FileChangeEvent struct {
	Path      string
	Operation string
	Category  FileCategory
	Timestamp time.Time
}

// WatchAgent monitors the filesystem and triggers agents on changes
type WatchAgent struct {
	basePath    string
	llmConfig   llm.Config
	watcher     *fsnotify.Watcher
	debouncer   *ChangeDebouncer
	dispatcher  *AgentDispatcher
	stream      *core.StreamingOutput
	activityLog *ActivityLog
	hashTracker *ContentHashTracker
	verbose     bool

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// WatchConfig holds configuration for the watch agent
type WatchConfig struct {
	BasePath     string
	LLMConfig    llm.Config
	Verbose      bool
	IncludeGlobs []string // Only watch paths matching these globs
	ExcludeGlobs []string // Skip paths matching these globs
	Stream       *core.StreamingOutput
}

// NewWatchAgent creates a new file watching agent
func NewWatchAgent(cfg WatchConfig) (*WatchAgent, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &WatchAgent{
		basePath:  cfg.BasePath,
		llmConfig: cfg.LLMConfig,
		watcher:   watcher,
		verbose:   cfg.Verbose,
		stream:    cfg.Stream,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Initialize debouncer
	w.debouncer = NewChangeDebouncer(w.handleBatch)

	// Initialize dispatcher with activity log
	w.activityLog = NewActivityLog(cfg.BasePath)
	w.dispatcher = NewAgentDispatcherWithLog(cfg.LLMConfig, cfg.BasePath, w.activityLog)

	// Initialize content hash tracker for deduplication
	w.hashTracker = NewContentHashTracker()

	return w, nil
}

// Start begins watching for file changes
func (w *WatchAgent) Start() error {
	// Add base path recursively
	if err := w.addWatchRecursive(w.basePath); err != nil {
		return fmt.Errorf("add watch paths: %w", err)
	}

	if w.verbose {
		fmt.Printf("📁 Watching: %s\n", w.basePath)
	}

	// Start event loop
	w.wg.Add(1)
	go w.eventLoop()

	return nil
}

// Stop stops the watch agent
func (w *WatchAgent) Stop() {
	w.cancel()
	_ = w.watcher.Close()
	w.debouncer.Stop()
	w.wg.Wait()
}

// SetFindingsHandler sets the callback for handling agent findings.
// This MUST be set for proper deduplication via knowledge.Service.IngestFindings.
func (w *WatchAgent) SetFindingsHandler(handler FindingsHandler) {
	w.dispatcher.SetFindingsHandler(handler)
}

// eventLoop processes filesystem events
func (w *WatchAgent) eventLoop() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			if w.verbose {
				fmt.Printf("⚠️  Watch error: %v\n", err)
			}

		case <-w.ctx.Done():
			return
		}
	}
}

// handleEvent processes a single filesystem event
func (w *WatchAgent) handleEvent(event fsnotify.Event) {
	// Get relative path
	relPath, err := filepath.Rel(w.basePath, event.Name)
	if err != nil {
		return
	}

	// Categorize the file
	category := w.categorize(relPath)
	if category == FileCategoryIgnore {
		return
	}

	// Determine operation
	op := "modify"
	switch {
	case event.Op&fsnotify.Create != 0:
		op = "create"
		// If a new directory is created, watch it
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			_ = w.watcher.Add(event.Name)
		}
	case event.Op&fsnotify.Remove != 0:
		op = "delete"
		// Clear hash on delete
		w.hashTracker.Remove(event.Name)
	case event.Op&fsnotify.Rename != 0:
		op = "rename"
		w.hashTracker.Remove(event.Name)
	}

	// For modify operations, check if content actually changed
	if op == "modify" {
		if !w.hashTracker.HasChanged(event.Name) {
			if w.verbose {
				fmt.Printf("⏭️  skip (no change): %s\n", relPath)
			}
			return
		}
	}

	// Queue the event for debouncing
	change := FileChangeEvent{
		Path:      relPath,
		Operation: op,
		Category:  category,
		Timestamp: time.Now(),
	}

	w.debouncer.Add(change)

	// Log the file change
	if w.activityLog != nil {
		w.activityLog.LogFileChange(relPath, op, category)
	}

	if w.verbose {
		fmt.Printf("📝 %s: %s (%s)\n", op, relPath, category)
	}

	if w.stream != nil {
		w.stream.Emit(core.EventAgentStart, "watch", fmt.Sprintf("%s: %s", op, relPath), map[string]any{
			"category": string(category),
		})
	}
}

// categorize determines the FileCategory for a path
func (w *WatchAgent) categorize(relPath string) FileCategory {
	name := filepath.Base(relPath)
	ext := strings.ToLower(filepath.Ext(name))
	dir := filepath.Dir(relPath)

	// Ignore hidden files (except .env.example)
	if strings.HasPrefix(name, ".") && name != ".env.example" {
		return FileCategoryIgnore
	}

	// Check ignored directories using centralized patterns
	for ig := range patterns.IgnoredDirs {
		if strings.Contains(relPath, ig+string(os.PathSeparator)) || name == ig {
			return FileCategoryIgnore
		}
	}

	// Dependency files (high priority)
	if patterns.IsDependencyFile(name) {
		return FileCategoryDeps
	}

	// Documentation
	if ext == ".md" || strings.HasPrefix(dir, "docs") {
		return FileCategoryDocs
	}

	// Config files
	if patterns.IsConfigFile(name, ext) {
		return FileCategoryConfig
	}

	// Code files
	if patterns.IsCodeFile(ext) {
		return FileCategoryCode
	}

	return FileCategoryIgnore
}

// handleBatch processes a batch of debounced changes
func (w *WatchAgent) handleBatch(changes []FileChangeEvent) {
	if len(changes) == 0 {
		return
	}

	// Group by category
	byCategory := make(map[FileCategory][]FileChangeEvent)
	for _, c := range changes {
		byCategory[c.Category] = append(byCategory[c.Category], c)
	}

	if w.verbose {
		fmt.Printf("🔄 Processing batch: %d changes\n", len(changes))
	}

	// Dispatch to appropriate agents
	for category, categoryChanges := range byCategory {
		w.dispatcher.Dispatch(w.ctx, category, categoryChanges)
	}
}

// addWatchRecursive adds the directory and all subdirectories to the watcher
func (w *WatchAgent) addWatchRecursive(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		name := d.Name()

		// Skip ignored directories
		ignoreDirs := []string{"node_modules", "vendor", ".git", "dist", "build", "__pycache__", ".next"}
		for _, ig := range ignoreDirs {
			if name == ig {
				return filepath.SkipDir
			}
		}

		if strings.HasPrefix(name, ".") && name != ".github" {
			return filepath.SkipDir
		}

		return w.watcher.Add(path)
	})
}

// ChangeDebouncer batches rapid file changes
type ChangeDebouncer struct {
	pending []FileChangeEvent
	timer   *time.Timer
	mu      sync.Mutex
	onFlush func([]FileChangeEvent)
	delay   time.Duration
	stopped bool
}

// NewChangeDebouncer creates a new debouncer with the given flush callback
func NewChangeDebouncer(onFlush func([]FileChangeEvent)) *ChangeDebouncer {
	return &ChangeDebouncer{
		pending: make([]FileChangeEvent, 0),
		onFlush: onFlush,
		delay:   500 * time.Millisecond,
	}
}

// Add queues a change event
func (d *ChangeDebouncer) Add(change FileChangeEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	d.pending = append(d.pending, change)

	// Reset timer
	if d.timer != nil {
		d.timer.Stop()
	}

	// Determine delay based on category
	delay := d.delay
	switch change.Category {
	case FileCategoryDeps:
		delay = 2 * time.Second // Larger delay for deps
	case FileCategoryDocs:
		delay = 1 * time.Second
	}

	d.timer = time.AfterFunc(delay, d.flush)
}

// flush sends pending events to the handler
func (d *ChangeDebouncer) flush() {
	d.mu.Lock()
	events := d.pending
	d.pending = make([]FileChangeEvent, 0)
	d.mu.Unlock()

	if len(events) > 0 && d.onFlush != nil {
		d.onFlush(events)
	}
}

// Stop stops the debouncer
func (d *ChangeDebouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopped = true
	if d.timer != nil {
		d.timer.Stop()
	}
}

// FindingsHandler is called when an agent produces findings.
// This allows the caller to handle persistence (e.g., via knowledge.Service.IngestFindings).
type FindingsHandler func(ctx context.Context, findings []core.Finding) error

// AgentDispatcher routes file changes to appropriate agents
type AgentDispatcher struct {
	llmConfig       llm.Config
	basePath        string
	activityLog     *ActivityLog
	findingsHandler FindingsHandler
	mu              sync.Mutex
}

// NewAgentDispatcher creates a new agent dispatcher
func NewAgentDispatcher(cfg llm.Config, basePath string) *AgentDispatcher {
	return &AgentDispatcher{
		llmConfig: cfg,
		basePath:  basePath,
	}
}

// NewAgentDispatcherWithLog creates a dispatcher with activity logging
func NewAgentDispatcherWithLog(cfg llm.Config, basePath string, log *ActivityLog) *AgentDispatcher {
	return &AgentDispatcher{
		llmConfig:   cfg,
		basePath:    basePath,
		activityLog: log,
	}
}

// SetFindingsHandler sets the callback for handling agent findings.
// This MUST be set for proper deduplication - without it, findings are logged but not persisted.
func (d *AgentDispatcher) SetFindingsHandler(handler FindingsHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.findingsHandler = handler
}

// Dispatch routes changes to the appropriate agent
func (d *AgentDispatcher) Dispatch(ctx context.Context, category FileCategory, changes []FileChangeEvent) {
	d.mu.Lock()
	handler := d.findingsHandler
	d.mu.Unlock()

	// Determine which agent to use
	var agent core.Agent
	switch category {
	case FileCategoryCode:
		// Use deterministic CodeAgent for code analysis
		agent = analysis.NewCodeAgent(d.llmConfig, d.basePath)
	case FileCategoryDocs:
		agent = analysis.NewDocAgent(d.llmConfig)
	case FileCategoryDeps:
		agent = analysis.NewDepsAgent(d.llmConfig)
	default:
		return
	}

	// Build input with changed files context
	changedPaths := make([]string, len(changes))
	for i, c := range changes {
		changedPaths[i] = c.Path
	}

	input := core.Input{
		BasePath:     d.basePath,
		ProjectName:  filepath.Base(d.basePath),
		Mode:         core.ModeWatch,
		ChangedFiles: changedPaths,
	}

	// Run agent in background
	actLog := d.activityLog
	go func() {
		// Close agent when goroutine exits to release LLM resources
		if closeable, ok := agent.(core.CloseableAgent); ok {
			defer func() { _ = closeable.Close() }()
		}

		fmt.Printf("  🤖 Running %s agent for %d changed files...\n", agent.Name(), len(changes))
		output, err := agent.Run(ctx, input)
		if err != nil {
			fmt.Printf("  ⚠️  %s agent error: %v\n", agent.Name(), err)
			if actLog != nil {
				actLog.LogAgentRun(agent.Name(), 0, 0, err)
			}
			return
		}
		fmt.Printf("  ✓ %s agent found %d findings (%.1fs)\n", agent.Name(), len(output.Findings), output.Duration.Seconds())

		// Log successful run and findings
		if actLog != nil {
			actLog.LogAgentRun(agent.Name(), len(output.Findings), output.Duration, nil)
			for _, f := range output.Findings {
				actLog.LogFinding(agent.Name(), f.Title, string(f.Type))
			}
		}

		// Persist findings via handler (uses knowledge.Service.IngestFindings for proper deduplication)
		if handler != nil && len(output.Findings) > 0 {
			if err := handler(ctx, output.Findings); err != nil {
				fmt.Printf("  ⚠️  persist findings error: %v\n", err)
			}
		} else if handler == nil && len(output.Findings) > 0 {
			fmt.Printf("  ⚠️  no findings handler configured - findings not persisted\n")
		}
	}()
}

// ContentHashTracker tracks file content hashes to detect actual changes
type ContentHashTracker struct {
	hashes map[string]string
	mu     sync.RWMutex
}

// NewContentHashTracker creates a new content hash tracker
func NewContentHashTracker() *ContentHashTracker {
	return &ContentHashTracker{
		hashes: make(map[string]string),
	}
}

// HasChanged checks if a file's content has changed since last check
// Returns true if file is new or content changed, false if unchanged
func (t *ContentHashTracker) HasChanged(path string) bool {
	hash, err := t.computeHash(path)
	if err != nil {
		// If we can't read the file, assume it changed
		return true
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	oldHash, exists := t.hashes[path]
	t.hashes[path] = hash

	if !exists {
		// First time seeing this file
		return true
	}

	return hash != oldHash
}

// Remove removes a file from the tracker
func (t *ContentHashTracker) Remove(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.hashes, path)
}

// computeHash calculates MD5 hash of file content
func (t *ContentHashTracker) computeHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
