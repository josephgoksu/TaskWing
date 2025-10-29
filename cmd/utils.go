package cmd

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/manifoldco/promptui"
)

// statusIcon provides a compact emoji indicator for task status
func statusIcon(s models.TaskStatus) string {
	switch s {
	case models.StatusTodo:
		return "⭕"
	case models.StatusDoing:
		return "🔄"
	case models.StatusReview:
		return "🔍"
	case models.StatusDone:
		return "✅"
	default:
		return string(s)
	}
}

// priorityIcon provides a compact emoji badge for task priority
func priorityIcon(p models.TaskPriority) string {
	switch p {
	case models.PriorityUrgent:
		return "🟥 urgent"
	case models.PriorityHigh:
		return "🟧 high"
	case models.PriorityMedium:
		return "🟨 medium"
	case models.PriorityLow:
		return "🟩 low"
	default:
		return string(p)
	}
}

// resolveTaskID resolves a partial task ID to a full task ID.
// Simplified version supporting only ID-based resolution.
// For fuzzy matching, CLI commands can fall back to prompting the user.
func resolveTaskID(st store.TaskStore, partialID string) (string, error) {
	partialID = strings.TrimSpace(strings.ToLower(partialID))
	if partialID == "" {
		return "", fmt.Errorf("task ID cannot be empty")
	}

	tasks, err := st.ListTasks(nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list tasks: %w", err)
	}

	// Exact match
	for _, task := range tasks {
		if strings.ToLower(task.ID) == partialID {
			return task.ID, nil
		}
	}

	// Prefix match (4+ chars)
	if len(partialID) >= 4 {
		var matches []string
		for _, task := range tasks {
			if strings.HasPrefix(strings.ToLower(task.ID), partialID) {
				matches = append(matches, task.ID)
			}
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
		if len(matches) > 1 {
			return "", fmt.Errorf("ambiguous ID '%s' matches %d tasks", partialID, len(matches))
		}
	}

	return "", fmt.Errorf("no task found with ID '%s'", partialID)
}

// Archive utility functions shared between archive.go, done.go, and mcp_tools_archive.go

func getArchiveDir() string {
	cfg := GetConfig()
	// Default under project root to match existing tests and CLI behavior
	return filepath.Join(cfg.Project.RootDir, "archive")
}

// archiveAndDeleteSubtree archives a task and all its descendants, then deletes them from the active store.
// - Parent task receives the provided lessons text; descendants are archived with empty lessons by default.
// - Tags are applied to all archived entries.
// - Returns the list of created archive entries (parent first when possible).
func archiveAndDeleteSubtree(taskStore store.TaskStore, arch store.ArchiveStore, root models.Task, parentLessons string, tags []string) ([]models.ArchiveEntry, error) {
	// Fetch subtree (root + all descendants)
	subtree, err := taskStore.GetTaskWithDescendants(root.ID)
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}

	// Make parent appear first in archive list if not already
	entries := make([]models.ArchiveEntry, 0, len(subtree))

	// Create archives for each task in the subtree
	for _, t := range subtree {
		lessons := ""
		if t.ID == root.ID {
			lessons = parentLessons
		}
		e, err := arch.CreateFromTask(t, lessons, tags)
		if err != nil {
			return entries, fmt.Errorf("archive '%s': %w", t.Title, err)
		}
		entries = append(entries, e)
	}

	// Delete the entire subtree from active tasks using batch delete
	ids := make([]string, 0, len(subtree))
	for _, t := range subtree {
		ids = append(ids, t.ID)
	}
	if _, err := taskStore.DeleteTasks(ids); err != nil {
		return entries, fmt.Errorf("delete subtree: %w", err)
	}

	return entries, nil
}

func getArchiveStore() (store.ArchiveStore, error) {
	s := store.NewFileArchiveStore()
	if err := s.Initialize(map[string]string{"archiveDir": getArchiveDir()}); err != nil {
		return nil, err
	}
	return s, nil
}

// Utility to prompt for simple input used by done integration as well
func promptInput(label string) (string, error) {
	p := promptui.Prompt{Label: label}
	return p.Run()
}

// gatherLessonsInteractive provides a richer UX to compose lessons learned
func gatherLessonsInteractive(task models.Task, allowAISuggest, autoPick, allowPolish bool) string {
	// Auto-pick AI suggestion if enabled
	if allowAISuggest && autoPick {
		if s, ok := aiSuggestLessons(task); ok && len(s) > 0 {
			chosen := s[0]
			if allowPolish {
				if polished, ok := aiPolishLessons(chosen); ok {
					chosen = polished
				}
			}
			return chosen
		}
	}

	// Offer simplified menu (removed guided questions)
	options := []string{"Type a short summary"}
	var aiSuggestions []string
	if allowAISuggest {
		if s, ok := aiSuggestLessons(task); ok && len(s) > 0 {
			aiSuggestions = s
			options = append([]string{"Pick from AI suggestions"}, options...)
		}
	}

	sel := promptui.Select{Label: "How would you like to add lessons learned?", Items: options}
	_, choice, err := sel.Run()
	if err != nil {
		return ""
	}

	switch choice {
	case "Pick from AI suggestions":
		pick := promptui.Select{Label: "AI suggestions (pick one)", Items: aiSuggestions}
		_, out, err := pick.Run()
		if err != nil {
			return ""
		}
		if allowPolish {
			if p, ok := aiPolishLessons(out); ok {
				out = p
			}
		}
		return out
	// Removed guided questions - was too tedious
	default:
		// Type a short summary
		ps := promptui.Prompt{Label: "Lessons learned (1-3 sentences)", AllowEdit: true}
		out, _ := ps.Run()
		if allowPolish {
			if p, ok := aiPolishLessons(out); ok {
				out = p
			}
		}
		return out
	}
}

// AI helpers
func aiSuggestLessons(task models.Task) ([]string, bool) {
	cfg := GetConfig()
	prov, err := createLLMProvider(&cfg.LLM)
	if err != nil {
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "AI disabled: %v\n", err)
		}
		return nil, false
	}
	ctx := context.Background()
	system := "You are an assistant that writes concise, actionable 'lessons learned' for completed tasks. Return 3 distinct suggestions as plain text lines separated by \n---\n, no numbering, each 1–2 sentences."
	content := fmt.Sprintf("Task Title: %s\nDescription: %s\nAcceptance: %s", task.Title, task.Description, task.AcceptanceCriteria)
	// Reuse ImprovePRD to get a freeform response
	out, err := prov.ImprovePRD(ctx, system, content, cfg.LLM.ModelName, cfg.LLM.APIKey, cfg.LLM.ProjectID, cfg.LLM.MaxOutputTokens, cfg.LLM.Temperature)
	if err != nil {
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "AI suggestion error: %v\n", err)
		}
		return nil, false
	}
	parts := strings.Split(out, "---")
	var suggestions []string
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			suggestions = append(suggestions, s)
		}
	}
	if len(suggestions) == 0 {
		return nil, false
	}
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}
	return suggestions, true
}

func aiPolishLessons(text string) (string, bool) {
	cfg := GetConfig()
	prov, err := createLLMProvider(&cfg.LLM)
	if err != nil {
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "AI disabled: %v\n", err)
		}
		return "", false
	}
	ctx := context.Background()
	system := "Rewrite this 'lessons learned' to be grammatically correct, concise, and active voice. Keep meaning; return only the revised text."
	out, err := prov.ImprovePRD(ctx, system, text, cfg.LLM.ModelName, cfg.LLM.APIKey, cfg.LLM.ProjectID, cfg.LLM.MaxOutputTokens, cfg.LLM.Temperature)
	if err != nil {
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "AI polish error: %v\n", err)
		}
		return "", false
	}
	return strings.TrimSpace(out), true
}

// Encryption helpers (AES-GCM with hex key)

func encryptFile(in, out, hexKey string) error {
	b, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	key, err := decodeHexKey(hexKey)
	if err != nil {
		return err
	}
	ct, err := aesGCMEncrypt(key, b)
	if err != nil {
		return err
	}
	return os.WriteFile(out, ct, 0o600)
}

func decryptFile(in, out, hexKey string) error {
	b, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	key, err := decodeHexKey(hexKey)
	if err != nil {
		return err
	}
	pt, err := aesGCMDecrypt(key, b)
	if err != nil {
		return err
	}
	return os.WriteFile(out, pt, 0o600)
}

func decodeHexKey(h string) ([]byte, error) {
	h = strings.TrimSpace(h)
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, fmt.Errorf("invalid hex key: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes (64 hex chars), got %d bytes", len(b))
	}
	return b, nil
}

// AES-GCM with random nonce prefixed to ciphertext
func aesGCMEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, out...), nil
}

func aesGCMDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	ct := ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}
