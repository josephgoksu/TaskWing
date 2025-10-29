package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/taskutil"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterArchiveTools wires archive-related tools into the MCP server
func RegisterArchiveTools(server *mcpsdk.Server, taskStore store.TaskStore) error {
	// list
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "archive-list", Description: "List archived entries (id, date, title, tags)"}, archiveListHandler())
	// view
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "archive-view", Description: "View an archive entry by id (full or prefix)"}, archiveViewHandler())
	// search
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "archive-search", Description: "Search archives by query, date range, and tags"}, archiveSearchHandler())
	// restore
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "archive-restore", Description: "Restore an archive entry into active tasks"}, archiveRestoreHandler(taskStore))
	// add (archive existing task)
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "archive-add", Description: "Archive an existing task by id/title with optional AI"}, archiveAddHandler(taskStore))
	// export/import/purge
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "archive-export", Description: "Export archives to a bundle (optionally encrypted)"}, archiveExportHandler())
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "archive-import", Description: "Import archives from a bundle (optionally decrypt)"}, archiveImportHandler())
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "archive-purge", Description: "Purge archives by retention policy (dry-run default)"}, archivePurgeHandler())
	return nil
}

func withArchiveStore[T any](fn func(store.ArchiveStore) (*mcpsdk.CallToolResultFor[T], error)) (*mcpsdk.CallToolResultFor[T], error) {
	s, err := archiveStore()
	if err != nil {
		return nil, types.NewMCPError("ARCHIVE_INIT", fmt.Sprintf("init archive store: %v", err), nil)
	}
	defer func() { _ = s.Close() }()
	return fn(s)
}

func archiveListHandler() mcpsdk.ToolHandlerFor[types.ArchiveListParams, types.ArchiveListResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ArchiveListParams]) (*mcpsdk.CallToolResultFor[types.ArchiveListResponse], error) {
		return withArchiveStore[types.ArchiveListResponse](func(s store.ArchiveStore) (*mcpsdk.CallToolResultFor[types.ArchiveListResponse], error) {
			items, err := s.List()
			if err != nil {
				return nil, types.NewMCPError("ARCHIVE_LIST", err.Error(), nil)
			}
			resp := types.ArchiveListResponse{Items: make([]types.ArchiveIndexItem, 0, len(items)), Count: len(items)}
			for _, it := range items {
				resp.Items = append(resp.Items, types.ArchiveIndexItem{ID: it.ID, Date: it.Date, Title: it.Title, Tags: it.Tags, FilePath: it.FilePath})
			}
			return &mcpsdk.CallToolResultFor[types.ArchiveListResponse]{StructuredContent: resp, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("Found %d archive entries", len(items))}}}, nil
		})
	}
}

func archiveViewHandler() mcpsdk.ToolHandlerFor[types.ArchiveViewParams, types.ArchiveEntryResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ArchiveViewParams]) (*mcpsdk.CallToolResultFor[types.ArchiveEntryResponse], error) {
		id := params.Arguments.ID
		return withArchiveStore[types.ArchiveEntryResponse](func(s store.ArchiveStore) (*mcpsdk.CallToolResultFor[types.ArchiveEntryResponse], error) {
			e, fullPath, err := s.GetByID(id)
			if err != nil {
				return nil, types.NewMCPError("ARCHIVE_VIEW", err.Error(), map[string]interface{}{"id": id})
			}
			// Convert to relative path for API consistency
			relPath := fullPath
			if strings.Contains(fullPath, "/archive/") {
				parts := strings.Split(fullPath, "/archive/")
				if len(parts) > 1 {
					relPath = parts[1]
				}
			}
			resp := types.ArchiveEntryResponse{
				ID: e.ID, TaskID: e.TaskID, Title: e.Title, Description: e.Description, LessonsLearned: e.LessonsLearned, Tags: e.Tags, ArchivedAt: e.ArchivedAt.Format(time.RFC3339), FilePath: relPath,
			}
			return &mcpsdk.CallToolResultFor[types.ArchiveEntryResponse]{StructuredContent: resp, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("Archive %s: %s", taskutil.ShortID(e.ID), e.Title)}}}, nil
		})
	}
}

func archiveSearchHandler() mcpsdk.ToolHandlerFor[types.ArchiveSearchParams, types.ArchiveListResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ArchiveSearchParams]) (*mcpsdk.CallToolResultFor[types.ArchiveListResponse], error) {
		args := params.Arguments
		return withArchiveStore[types.ArchiveListResponse](func(s store.ArchiveStore) (*mcpsdk.CallToolResultFor[types.ArchiveListResponse], error) {
			var fromPtr, toPtr *time.Time
			if args.From != "" {
				t, err := time.Parse("2006-01-02", args.From)
				if err != nil {
					return nil, types.NewMCPError("INVALID_DATE", fmt.Sprintf("invalid 'from' date format (expected YYYY-MM-DD): %v", err), map[string]interface{}{"from": args.From})
				}
				fromPtr = &t
			}
			if args.To != "" {
				t, err := time.Parse("2006-01-02", args.To)
				if err != nil {
					return nil, types.NewMCPError("INVALID_DATE", fmt.Sprintf("invalid 'to' date format (expected YYYY-MM-DD): %v", err), map[string]interface{}{"to": args.To})
				}
				tt := t.Add(24*time.Hour - time.Nanosecond)
				toPtr = &tt
			}
			items, err := s.Search(args.Query, store.ArchiveSearchFilters{DateFrom: fromPtr, DateTo: toPtr, Tags: args.Tags})
			if err != nil {
				return nil, types.NewMCPError("ARCHIVE_SEARCH", err.Error(), nil)
			}
			resp := types.ArchiveListResponse{Items: make([]types.ArchiveIndexItem, 0, len(items)), Count: len(items)}
			for _, it := range items {
				resp.Items = append(resp.Items, types.ArchiveIndexItem{ID: it.ID, Date: it.Date, Title: it.Title, Tags: it.Tags, FilePath: it.FilePath})
			}
			return &mcpsdk.CallToolResultFor[types.ArchiveListResponse]{StructuredContent: resp, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("Found %d results for '%s'", len(items), args.Query)}}}, nil
		})
	}
}

func archiveRestoreHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.ArchiveRestoreParams, types.ArchiveRestoreResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ArchiveRestoreParams]) (*mcpsdk.CallToolResultFor[types.ArchiveRestoreResponse], error) {
		id := params.Arguments.ID
		return withArchiveStore[types.ArchiveRestoreResponse](func(s store.ArchiveStore) (*mcpsdk.CallToolResultFor[types.ArchiveRestoreResponse], error) {
			t, err := s.RestoreToTaskStore(id, taskStore)
			if err != nil {
				return nil, types.NewMCPError("ARCHIVE_RESTORE", err.Error(), map[string]interface{}{"id": id})
			}
			resp := types.ArchiveRestoreResponse{Restored: taskToResponse(t)}
			return &mcpsdk.CallToolResultFor[types.ArchiveRestoreResponse]{StructuredContent: resp, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("Restored '%s' (%s)", t.Title, taskutil.ShortID(t.ID))}}}, nil
		})
	}
}

func archiveAddHandler(taskStore store.TaskStore) mcpsdk.ToolHandlerFor[types.ArchiveAddParams, types.ArchiveEntryResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ArchiveAddParams]) (*mcpsdk.CallToolResultFor[types.ArchiveEntryResponse], error) {
		args := params.Arguments
		// resolve reference
		resolved, err := resolveTask(taskStore, args.Reference)
		if err != nil {
			return nil, types.NewMCPError("TASK_NOT_FOUND", fmt.Sprintf("could not resolve '%s'", args.Reference), nil)
		}
		t := *resolved
		lessons := args.Lessons
		if lessons == "" && args.AISuggest {
			if s, ok := suggestLessons(t); ok && len(s) > 0 {
				lessons = s[0] // non-interactive; pick best
			}
		}
		if args.AIFix && lessons != "" {
			if p, ok := polishLessons(lessons); ok {
				lessons = p
			}
		}
		return withArchiveStore[types.ArchiveEntryResponse](func(s store.ArchiveStore) (*mcpsdk.CallToolResultFor[types.ArchiveEntryResponse], error) {
			// Archive parent + descendants together
			entries, err := archiveAndDelete(taskStore, s, t, lessons, args.Tags)
			if err != nil {
				return nil, types.NewMCPError("ARCHIVE_ADD", err.Error(), nil)
			}
			if len(entries) == 0 {
				return nil, types.NewMCPError("ARCHIVE_ADD", "no entries created", nil)
			}
			parent := entries[0]
			_, fullPath, _ := s.GetByID(parent.ID)
			// Convert to relative path for API consistency
			relPath := fullPath
			if strings.Contains(fullPath, "/archive/") {
				parts := strings.Split(fullPath, "/archive/")
				if len(parts) > 1 {
					relPath = parts[1]
				}
			}
			resp := types.ArchiveEntryResponse{ID: parent.ID, TaskID: parent.TaskID, Title: parent.Title, Description: parent.Description, LessonsLearned: parent.LessonsLearned, Tags: parent.Tags, ArchivedAt: parent.ArchivedAt.Format(time.RFC3339), FilePath: relPath}
			extra := ""
			if len(entries) > 1 {
				extra = fmt.Sprintf(" +%d subtasks", len(entries)-1)
			}
			return &mcpsdk.CallToolResultFor[types.ArchiveEntryResponse]{
				StructuredContent: resp,
				Content:           []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("Archived '%s' (%s)%s", parent.Title, taskutil.ShortID(parent.ID), extra)}},
			}, nil
		})
	}
}

func archiveExportHandler() mcpsdk.ToolHandlerFor[types.ArchiveExportParams, types.BulkOperationResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ArchiveExportParams]) (*mcpsdk.CallToolResultFor[types.BulkOperationResponse], error) {
		args := params.Arguments
		return withArchiveStore[types.BulkOperationResponse](func(s store.ArchiveStore) (*mcpsdk.CallToolResultFor[types.BulkOperationResponse], error) {
			out := args.File
			tmp := out + ".tmp"
			if err := s.Export(tmp); err != nil {
				return nil, types.NewMCPError("ARCHIVE_EXPORT", err.Error(), nil)
			}
			if args.Encrypt {
				if args.Key == "" {
					return nil, types.NewMCPError("MISSING_KEY", "--key required when encrypting", nil)
				}
				if err := encryptFile(tmp, out, args.Key); err != nil {
					return nil, types.NewMCPError("ARCHIVE_EXPORT", err.Error(), nil)
				}
				_ = os.Remove(tmp)
			} else {
				if err := os.Rename(tmp, out); err != nil {
					return nil, types.NewMCPError("ARCHIVE_EXPORT", err.Error(), nil)
				}
			}
			resp := types.BulkOperationResponse{Succeeded: 1}
			return &mcpsdk.CallToolResultFor[types.BulkOperationResponse]{StructuredContent: resp, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("Exported archive to %s", out)}}}, nil
		})
	}
}

func archiveImportHandler() mcpsdk.ToolHandlerFor[types.ArchiveImportParams, types.BulkOperationResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ArchiveImportParams]) (*mcpsdk.CallToolResultFor[types.BulkOperationResponse], error) {
		args := params.Arguments
		return withArchiveStore[types.BulkOperationResponse](func(s store.ArchiveStore) (*mcpsdk.CallToolResultFor[types.BulkOperationResponse], error) {
			src := args.File
			if args.Decrypt {
				if args.Key == "" {
					return nil, types.NewMCPError("MISSING_KEY", "--key required when decrypting", nil)
				}
				tmp := src + ".dec.tmp"
				if err := decryptFile(src, tmp, args.Key); err != nil {
					return nil, types.NewMCPError("ARCHIVE_IMPORT", err.Error(), nil)
				}
				src = tmp
				defer func() { _ = os.Remove(tmp) }()
			}
			if err := s.Import(src); err != nil {
				return nil, types.NewMCPError("ARCHIVE_IMPORT", err.Error(), nil)
			}
			resp := types.BulkOperationResponse{Succeeded: 1}
			return &mcpsdk.CallToolResultFor[types.BulkOperationResponse]{StructuredContent: resp, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("Imported archive from %s", args.File)}}}, nil
		})
	}
}

func archivePurgeHandler() mcpsdk.ToolHandlerFor[types.ArchivePurgeParams, types.BulkOperationResponse] {
	return func(ctx context.Context, ss *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[types.ArchivePurgeParams]) (*mcpsdk.CallToolResultFor[types.BulkOperationResponse], error) {
		args := params.Arguments
		return withArchiveStore[types.BulkOperationResponse](func(s store.ArchiveStore) (*mcpsdk.CallToolResultFor[types.BulkOperationResponse], error) {
			var older *time.Duration
			if args.OlderThan != "" {
				if d, err := time.ParseDuration(args.OlderThan); err == nil {
					older = &d
				}
			}
			res, err := s.Purge(store.PurgeOptions{OlderThan: older, DryRun: args.DryRun})
			if err != nil {
				return nil, types.NewMCPError("ARCHIVE_PURGE", err.Error(), nil)
			}
			text := "DRY-RUN"
			if !res.DryRun {
				text = "APPLIED"
			}
			msg := fmt.Sprintf("%s: considered %d, deleted %d, freed %d bytes", text, res.FilesConsidered, res.FilesDeleted, res.BytesFreed)
			resp := types.BulkOperationResponse{Succeeded: res.FilesDeleted}
			return &mcpsdk.CallToolResultFor[types.BulkOperationResponse]{StructuredContent: resp, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: msg}}}, nil
		})
	}
}
