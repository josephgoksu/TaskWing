package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Manage archived tasks (list, view, search, restore, export, import, purge)",
	Long:  "Archive management commands for viewing, searching, restoring, exporting, importing, and purging archived tasks.",
	Run: func(cmd *cobra.Command, args []string) {
		// Show help when just `taskwing archive` is called
		_ = cmd.Help()
	},
}

var archiveListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List archived tasks",
	Run: func(cmd *cobra.Command, args []string) {
		s, err := getArchiveStore()
		if err != nil {
			HandleFatalError("Failed to init archive store", err)
		}
		defer func() { _ = s.Close() }()
		items, err := s.List()
		if err != nil {
			HandleFatalError("Failed to list archives", err)
		}
		if len(items) == 0 {
			fmt.Println("No archives found.")
			return
		}
		for _, it := range items {
			fmt.Printf("%s  %s  %s  %s\n", shortID(it.ID), it.Date, it.Title, strings.Join(it.Tags, ","))
		}
	},
}

var archiveViewCmd = &cobra.Command{
	Use:     "view <id>",
	Aliases: []string{"show"},
	Short:   "View archived entry",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		s, err := getArchiveStore()
		if err != nil {
			HandleFatalError("Failed to init archive store", err)
		}
		defer func() { _ = s.Close() }()
		e, path, err := s.GetByID(id)
		if err != nil {
			// Provide actionable guidance with specific steps
			fmt.Printf("❌ Archive entry not found for '%s'\n", id)
			fmt.Printf("\n💡 Tips:\n")
			fmt.Printf("   • Use 'taskwing archive list' to see all archived tasks\n")
			fmt.Printf("   • Archive IDs can be used as full UUID or 8-char prefix\n")
			fmt.Printf("   • This command expects an archive ID, not a task ID\n")
			os.Exit(1)
		}
		fmt.Printf("ID: %s\nTitle: %s\nArchivedAt: %s\nFile: %s\n\nDescription:\n%s\n\nLessons Learned:\n%s\n",
			e.ID, e.Title, e.ArchivedAt.Format(time.RFC3339), path, e.Description, e.LessonsLearned)
	},
}

var (
	searchQuery string
	filterFrom  string
	filterTo    string
	filterTags  []string
)

var archiveSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search archives (full-text + filters)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		searchQuery = args[0]
		s, err := getArchiveStore()
		if err != nil {
			HandleFatalError("Failed to init archive store", err)
		}
		defer func() { _ = s.Close() }()
		var fromPtr, toPtr *time.Time
		if filterFrom != "" {
			t, err := time.Parse("2006-01-02", filterFrom)
			if err != nil {
				HandleFatalError(fmt.Sprintf("Invalid --from date format (expected YYYY-MM-DD): %s", filterFrom), err)
			}
			fromPtr = &t
		}
		if filterTo != "" {
			t, err := time.Parse("2006-01-02", filterTo)
			if err != nil {
				HandleFatalError(fmt.Sprintf("Invalid --to date format (expected YYYY-MM-DD): %s", filterTo), err)
			}
			tt := t.Add(24*time.Hour - time.Nanosecond)
			toPtr = &tt
		}
		items, err := s.Search(searchQuery, store.ArchiveSearchFilters{DateFrom: fromPtr, DateTo: toPtr, Tags: filterTags})
		if err != nil {
			HandleFatalError("Search failed", err)
		}
		for _, it := range items {
			fmt.Printf("%s  %s  %s\n", shortID(it.ID), it.Date, it.Title)
		}
	},
}

var (
	addLessons   string
	addTags      string
	addAISuggest bool
	addAIFix     bool
	addAIAuto    bool
	addKeepTask  bool
)

var archiveAddCmd = &cobra.Command{
	Use:   "add <task-id-or-ref>",
	Short: "Archive an existing task on-demand",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ref := args[0]
		tasks, err := GetStore()
		if err != nil {
			HandleFatalError("Failed to init task store", err)
		}
		defer func() { _ = tasks.Close() }()

		t, err := resolveTaskReference(tasks, ref)
		if err != nil {
			HandleFatalError("Could not resolve task reference", err)
		}

		// Warn if task is not completed
		if t.Status != models.StatusDone {
			sel := promptui.Select{Label: "Task is not done. Archive anyway?", Items: []string{"No", "Yes"}}
			_, choice, err := sel.Run()
			if err != nil || choice == "No" {
				fmt.Println("Archive cancelled.")
				return
			}
		}

		lessons := strings.TrimSpace(addLessons)
		if lessons == "" {
			// Default to AI suggestions for better UX
			useAI := addAISuggest || (!cmd.Flags().Changed("ai-suggest"))
			lessons = gatherLessonsInteractive(*t, useAI, addAIAuto, addAIFix)
		}

		tags := []string{}
		if addTags == "" {
			addTags, _ = promptInput("Tags (comma-separated, optional)")
		}
		if addTags != "" {
			for _, part := range strings.Split(addTags, ",") {
				if s := strings.TrimSpace(part); s != "" {
					tags = append(tags, s)
				}
			}
		}

		arch := store.NewFileArchiveStore()
		if err := arch.Initialize(map[string]string{"archiveDir": getArchiveDir()}); err != nil {
			HandleFatalError("Failed to init archive store", err)
		}
		defer func() { _ = arch.Close() }()

		if addAIFix && strings.TrimSpace(lessons) != "" {
			if polished, ok := aiPolishLessons(lessons); ok {
				lessons = polished
			}
		}

		var entries []models.ArchiveEntry

		if addKeepTask {
			// Archive without deleting - only archive the parent task for simplicity
			entry, archiveErr := arch.CreateFromTask(*t, lessons, tags)
			if archiveErr != nil {
				HandleFatalError("Failed to archive task", archiveErr)
			}
			entries = []models.ArchiveEntry{entry}
		} else {
			// Archive and delete the entire subtree (parent + descendants)
			entries, err = archiveAndDeleteSubtree(tasks, arch, *t, lessons, tags)
			if err != nil {
				HandleFatalError("Failed to archive task subtree", err)
			}
		}

		// Output summary based on the parent entry (first) plus count
		if len(entries) > 0 {
			parent := entries[0]
			_, path, err := arch.GetByID(parent.ID)
			if err != nil {
				fmt.Printf("Warning: could not get archive path: %v\n", err)
				path = ""
			}
			short := shortID(parent.ID)
			if !addKeepTask {
				fmt.Printf("✅ Task removed from active list\n")
			} else {
				fmt.Printf("📌 Task kept on active list (--keep flag used)\n")
			}
			fmt.Printf("🗄️  Archived: %s (archive-id: %s)\n", parent.Title, short)
			if path != "" {
				fmt.Printf("     ↳ %s\n", path)
			}
			if len(entries) > 1 {
				fmt.Printf("     + %d related subtask(s) archived\n", len(entries)-1)
			}
			fmt.Printf("     View:   taskwing archive view %s\n", short)
			fmt.Printf("     Search: taskwing archive search \"%s\"\n", parent.Title)
		}
	},
}

var archiveRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore an archived task to active list",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		arch, err := getArchiveStore()
		if err != nil {
			HandleFatalError("Failed to init archive store", err)
		}
		defer func() { _ = arch.Close() }()
		tasks, err := GetStore()
		if err != nil {
			HandleFatalError("Failed to init task store", err)
		}
		defer func() { _ = tasks.Close() }()
		t, err := arch.RestoreToTaskStore(id, tasks)
		if err != nil {
			HandleFatalError("Restore failed", err)
		}
		fmt.Printf("✅ Restored as new task: %s (%s)\n", t.Title, t.ID)
	},
}

var archiveExportCmd = &cobra.Command{
	Use:   "export <file>",
	Short: "Export all archives to a portable JSON bundle",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		out := args[0]
		s, err := getArchiveStore()
		if err != nil {
			HandleFatalError("Failed to init archive store", err)
		}
		defer func() { _ = s.Close() }()
		// Export to a temp file first
		tmp := out + ".tmp"
		_ = os.Remove(tmp)
		if err := s.Export(tmp); err != nil {
			HandleFatalError("Export failed", err)
		}
		if exportEncrypt {
			if exportKey == "" {
				HandleFatalError("--key required when --encrypt is set", fmt.Errorf("missing key"))
			}
			if err := encryptFile(tmp, out, exportKey); err != nil {
				HandleFatalError("Encryption failed", err)
			}
			_ = os.Remove(tmp)
			fmt.Printf("🔐 Encrypted export written to %s\n", out)
		} else {
			if err := os.Rename(tmp, out); err != nil {
				HandleFatalError("Failed to write export", err)
			}
			fmt.Printf("📦 Exported archive to %s\n", out)
		}
	},
}

var archiveImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import archives from a JSON bundle",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		in := args[0]
		if _, err := os.Stat(in); err != nil {
			HandleFatalError("Import file not found", err)
		}
		s, err := getArchiveStore()
		if err != nil {
			HandleFatalError("Failed to init archive store", err)
		}
		defer func() { _ = s.Close() }()
		src := in
		tmp := ""
		if importDecrypt {
			if importKey == "" {
				HandleFatalError("--key required when --decrypt is set", fmt.Errorf("missing key"))
			}
			tmp = in + ".dec.tmp"
			if err := decryptFile(in, tmp, importKey); err != nil {
				HandleFatalError("Decryption failed", err)
			}
			src = tmp
		}
		if err := s.Import(src); err != nil {
			HandleFatalError("Import failed", err)
		}
		if tmp != "" {
			_ = os.Remove(tmp)
		}
		fmt.Println("✅ Import complete")
	},
}

var (
	purgeOlderThan string
	purgeDryRun    bool
)

var archivePurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Purge archives by retention policy (dry-run by default)",
	Run: func(cmd *cobra.Command, args []string) {
		s, err := getArchiveStore()
		if err != nil {
			HandleFatalError("Failed to init archive store", err)
		}
		defer func() { _ = s.Close() }()
		var older *time.Duration
		if purgeOlderThan != "" {
			d, err := time.ParseDuration(purgeOlderThan)
			if err != nil {
				HandleFatalError("Invalid --older-than duration (e.g., 720h)", err)
			}
			older = &d
		}
		res, err := s.Purge(store.PurgeOptions{DryRun: purgeDryRun, OlderThan: older})
		if err != nil {
			HandleFatalError("Purge failed", err)
		}
		mode := "DRY-RUN"
		if !res.DryRun {
			mode = "APPLIED"
		}
		fmt.Printf("%s: considered %d, deleted %d, freed %d bytes\n", mode, res.FilesConsidered, res.FilesDeleted, res.BytesFreed)
	},
}

// Encryption flags for export/import commands
var (
	exportEncrypt bool
	exportKey     string
	importDecrypt bool
	importKey     string
)

func init() {
	rootCmd.AddCommand(archiveCmd)
	archiveCmd.AddCommand(archiveListCmd)
	archiveCmd.AddCommand(archiveViewCmd)
	archiveCmd.AddCommand(archiveSearchCmd)
	archiveCmd.AddCommand(archiveAddCmd)
	archiveCmd.AddCommand(archiveRestoreCmd)
	archiveCmd.AddCommand(archiveExportCmd)
	archiveCmd.AddCommand(archiveImportCmd)
	archiveCmd.AddCommand(archivePurgeCmd)

	archiveSearchCmd.Flags().StringVar(&filterFrom, "from", "", "Start date (YYYY-MM-DD)")
	archiveSearchCmd.Flags().StringVar(&filterTo, "to", "", "End date (YYYY-MM-DD)")
	archiveSearchCmd.Flags().StringSliceVar(&filterTags, "tag", nil, "Filter by tag (repeatable)")

	archivePurgeCmd.Flags().StringVar(&purgeOlderThan, "older-than", "", "Purge entries older than duration (e.g., 720h for 30 days)")
	archivePurgeCmd.Flags().BoolVar(&purgeDryRun, "dry-run", true, "Preview deletions without applying")

	archiveExportCmd.Flags().BoolVar(&exportEncrypt, "encrypt", false, "Encrypt exported bundle with AES-GCM")
	archiveExportCmd.Flags().StringVar(&exportKey, "key", "", "Hex-encoded 32-byte key for encryption")
	archiveImportCmd.Flags().BoolVar(&importDecrypt, "decrypt", false, "Decrypt bundle before import (AES-GCM)")
	archiveImportCmd.Flags().StringVar(&importKey, "key", "", "Hex-encoded 32-byte key for decryption")

	archiveAddCmd.Flags().StringVar(&addLessons, "lessons", "", "Lessons learned text (optional)")
	archiveAddCmd.Flags().StringVar(&addTags, "tags", "", "Comma-separated tags (optional)")
	// Default to no-AI to minimize token usage; users can opt-in per run
	archiveAddCmd.Flags().BoolVar(&addAISuggest, "ai-suggest", false, "Use AI to propose lessons learned suggestions (default: false)")
	archiveAddCmd.Flags().BoolVar(&addAIFix, "ai-fix", false, "Use AI to polish/grammar-fix lessons text (default: false)")
	archiveAddCmd.Flags().BoolVar(&addAIAuto, "ai-auto", false, "Auto-pick the first AI suggestion without prompting")
	archiveAddCmd.Flags().BoolVar(&addKeepTask, "keep", false, "Keep task on active list after archiving (default: remove)")
	// Provide a clean, archive-specific help template so root-level examples don't appear here
	archiveCmd.SetHelpTemplate(`Usage:
  {{.UseLine}}

Archive Commands:
  list                          List archived tasks
  view <id>                     View full details of an archived entry (alias: show)
  add <task-id> [--keep]        Archive an existing task (default: remove from active list)
  search <query> [flags]        Search archives (full-text + date/tag filters)
  restore <id>                  Restore an archived task to the active list
  export <file> [--encrypt]     Export all archives to a portable JSON bundle
  import <file> [--decrypt]     Import archives from a bundle
  purge [--older-than]          Purge archives by retention policy (dry-run default)

Examples:
  taskwing archive list
  taskwing archive view 1234abcd
  taskwing archive add 1234abcd              # Archive and remove from active list (default)
  taskwing archive add 1234abcd --keep       # Archive but keep on active list
  taskwing archive search "token refresh" --from 2025-01-01 --to 2025-12-31 --tag auth
  taskwing archive restore 1234abcd
  taskwing archive export archive.json
  taskwing archive export archive.enc --encrypt --key <64-hex>
  taskwing archive import archive.json
  taskwing archive import archive.enc --decrypt --key <64-hex>
  taskwing archive purge --older-than 720h --dry-run=false

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}
