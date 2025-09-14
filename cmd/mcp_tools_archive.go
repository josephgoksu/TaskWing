package cmd

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/josephgoksu/TaskWing/store"
    "github.com/josephgoksu/TaskWing/types"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterArchiveTools wires archive-related tools into the MCP server
func RegisterArchiveTools(server *mcp.Server, taskStore store.TaskStore) error {
    // list
    mcp.AddTool(server, &mcp.Tool{ Name: "archive-list", Description: "List archived entries (id, date, title, tags)" }, archiveListHandler())
    // view
    mcp.AddTool(server, &mcp.Tool{ Name: "archive-view", Description: "View an archive entry by id (full or prefix)" }, archiveViewHandler())
    // search
    mcp.AddTool(server, &mcp.Tool{ Name: "archive-search", Description: "Search archives by query, date range, and tags" }, archiveSearchHandler())
    // restore
    mcp.AddTool(server, &mcp.Tool{ Name: "archive-restore", Description: "Restore an archive entry into active tasks" }, archiveRestoreHandler(taskStore))
    // add (archive existing task)
    mcp.AddTool(server, &mcp.Tool{ Name: "archive-add", Description: "Archive an existing task by id/title with optional AI" }, archiveAddHandler(taskStore))
    // export/import/purge
    mcp.AddTool(server, &mcp.Tool{ Name: "archive-export", Description: "Export archives to a bundle (optionally encrypted)" }, archiveExportHandler())
    mcp.AddTool(server, &mcp.Tool{ Name: "archive-import", Description: "Import archives from a bundle (optionally decrypt)" }, archiveImportHandler())
    mcp.AddTool(server, &mcp.Tool{ Name: "archive-purge", Description: "Purge archives by retention policy (dry-run default)" }, archivePurgeHandler())
    return nil
}

func withArchiveStore[T any](fn func(store.ArchiveStore) (*mcp.CallToolResultFor[T], error)) (*mcp.CallToolResultFor[T], error) {
    s, err := getArchiveStore()
    if err != nil { return nil, types.NewMCPError("ARCHIVE_INIT", fmt.Sprintf("init archive store: %v", err), nil) }
    defer s.Close()
    return fn(s)
}

func archiveListHandler() mcp.ToolHandlerFor[types.ArchiveListParams, types.ArchiveListResponse] {
    return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ArchiveListParams]) (*mcp.CallToolResultFor[types.ArchiveListResponse], error) {
        return withArchiveStore(func(s store.ArchiveStore) (*mcp.CallToolResultFor[types.ArchiveListResponse], error) {
            items, err := s.List()
            if err != nil { return nil, types.NewMCPError("ARCHIVE_LIST", err.Error(), nil) }
            resp := types.ArchiveListResponse{Items: make([]types.ArchiveIndexItem, 0, len(items)), Count: len(items)}
            for _, it := range items {
                resp.Items = append(resp.Items, types.ArchiveIndexItem{ID: it.ID, Date: it.Date, Title: it.Title, Tags: it.Tags, FilePath: it.FilePath})
            }
            return &mcp.CallToolResultFor[types.ArchiveListResponse]{ StructuredContent: resp, Content: []mcp.Content{ &mcp.TextContent{ Text: fmt.Sprintf("Found %d archive entries", len(items)) } } }, nil
        })
    }
}

func archiveViewHandler() mcp.ToolHandlerFor[types.ArchiveViewParams, types.ArchiveEntryResponse] {
    return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ArchiveViewParams]) (*mcp.CallToolResultFor[types.ArchiveEntryResponse], error) {
        id := params.Arguments.ID
        return withArchiveStore(func(s store.ArchiveStore) (*mcp.CallToolResultFor[types.ArchiveEntryResponse], error) {
            e, fullPath, err := s.GetByID(id)
            if err != nil { return nil, types.NewMCPError("ARCHIVE_VIEW", err.Error(), map[string]interface{}{"id": id}) }
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
            return &mcp.CallToolResultFor[types.ArchiveEntryResponse]{ StructuredContent: resp, Content: []mcp.Content{ &mcp.TextContent{ Text: fmt.Sprintf("Archive %s: %s", shortID(e.ID), e.Title) } } }, nil
        })
    }
}

func archiveSearchHandler() mcp.ToolHandlerFor[types.ArchiveSearchParams, types.ArchiveListResponse] {
    return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ArchiveSearchParams]) (*mcp.CallToolResultFor[types.ArchiveListResponse], error) {
        args := params.Arguments
        return withArchiveStore(func(s store.ArchiveStore) (*mcp.CallToolResultFor[types.ArchiveListResponse], error) {
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
            items, err := s.Search(args.Query, store.ArchiveSearchFilters{ DateFrom: fromPtr, DateTo: toPtr, Tags: args.Tags })
            if err != nil { return nil, types.NewMCPError("ARCHIVE_SEARCH", err.Error(), nil) }
            resp := types.ArchiveListResponse{Items: make([]types.ArchiveIndexItem, 0, len(items)), Count: len(items)}
            for _, it := range items { resp.Items = append(resp.Items, types.ArchiveIndexItem{ID: it.ID, Date: it.Date, Title: it.Title, Tags: it.Tags, FilePath: it.FilePath}) }
            return &mcp.CallToolResultFor[types.ArchiveListResponse]{ StructuredContent: resp, Content: []mcp.Content{ &mcp.TextContent{ Text: fmt.Sprintf("Found %d results for '%s'", len(items), args.Query) } } }, nil
        })
    }
}

func archiveRestoreHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.ArchiveRestoreParams, types.ArchiveRestoreResponse] {
    return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ArchiveRestoreParams]) (*mcp.CallToolResultFor[types.ArchiveRestoreResponse], error) {
        id := params.Arguments.ID
        return withArchiveStore(func(s store.ArchiveStore) (*mcp.CallToolResultFor[types.ArchiveRestoreResponse], error) {
            t, err := s.RestoreToTaskStore(id, taskStore)
            if err != nil { return nil, types.NewMCPError("ARCHIVE_RESTORE", err.Error(), map[string]interface{}{"id": id}) }
            resp := types.ArchiveRestoreResponse{ Restored: taskToResponse(t) }
            return &mcp.CallToolResultFor[types.ArchiveRestoreResponse]{ StructuredContent: resp, Content: []mcp.Content{ &mcp.TextContent{ Text: fmt.Sprintf("Restored '%s' (%s)", t.Title, shortID(t.ID)) } } }, nil
        })
    }
}

func archiveAddHandler(taskStore store.TaskStore) mcp.ToolHandlerFor[types.ArchiveAddParams, types.ArchiveEntryResponse] {
    return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ArchiveAddParams]) (*mcp.CallToolResultFor[types.ArchiveEntryResponse], error) {
        args := params.Arguments
        // resolve reference
        resolved, err := resolveTaskReference(taskStore, args.Reference)
        if err != nil { return nil, types.NewMCPError("TASK_NOT_FOUND", fmt.Sprintf("could not resolve '%s'", args.Reference), nil) }
        t := *resolved
        lessons := args.Lessons
        if lessons == "" && args.AISuggest {
            if s, ok := aiSuggestLessons(t); ok && len(s) > 0 {
                lessons = s[0] // non-interactive; pick best
            }
        }
        if args.AIFix && lessons != "" {
            if p, ok := aiPolishLessons(lessons); ok { lessons = p }
        }
        return withArchiveStore(func(s store.ArchiveStore) (*mcp.CallToolResultFor[types.ArchiveEntryResponse], error) {
            // Archive parent + descendants together
            entries, err := archiveAndDeleteSubtree(taskStore, s, t, lessons, args.Tags)
            if err != nil { return nil, types.NewMCPError("ARCHIVE_ADD", err.Error(), nil) }
            if len(entries) == 0 { return nil, types.NewMCPError("ARCHIVE_ADD", "no entries created", nil) }
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
            resp := types.ArchiveEntryResponse{ ID: parent.ID, TaskID: parent.TaskID, Title: parent.Title, Description: parent.Description, LessonsLearned: parent.LessonsLearned, Tags: parent.Tags, ArchivedAt: parent.ArchivedAt.Format(time.RFC3339), FilePath: relPath }
            extra := ""
            if len(entries) > 1 { extra = fmt.Sprintf(" +%d subtasks", len(entries)-1) }
            return &mcp.CallToolResultFor[types.ArchiveEntryResponse]{
                StructuredContent: resp,
                Content: []mcp.Content{ &mcp.TextContent{ Text: fmt.Sprintf("Archived '%s' (%s)%s", parent.Title, shortID(parent.ID), extra) } },
            }, nil
        })
    }
}

func archiveExportHandler() mcp.ToolHandlerFor[types.ArchiveExportParams, types.BulkOperationResponse] {
    return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ArchiveExportParams]) (*mcp.CallToolResultFor[types.BulkOperationResponse], error) {
        args := params.Arguments
        return withArchiveStore(func(s store.ArchiveStore) (*mcp.CallToolResultFor[types.BulkOperationResponse], error) {
            out := args.File
            tmp := out + ".tmp"
            if err := s.Export(tmp); err != nil { return nil, types.NewMCPError("ARCHIVE_EXPORT", err.Error(), nil) }
            if args.Encrypt {
                if args.Key == "" { return nil, types.NewMCPError("MISSING_KEY", "--key required when encrypting", nil) }
                if err := encryptFile(tmp, out, args.Key); err != nil { return nil, types.NewMCPError("ARCHIVE_EXPORT", err.Error(), nil) }
                _ = os.Remove(tmp)
            } else {
                if err := os.Rename(tmp, out); err != nil { return nil, types.NewMCPError("ARCHIVE_EXPORT", err.Error(), nil) }
            }
            resp := types.BulkOperationResponse{ Succeeded: 1 }
            return &mcp.CallToolResultFor[types.BulkOperationResponse]{ StructuredContent: resp, Content: []mcp.Content{ &mcp.TextContent{ Text: fmt.Sprintf("Exported archive to %s", out) } } }, nil
        })
    }
}

func archiveImportHandler() mcp.ToolHandlerFor[types.ArchiveImportParams, types.BulkOperationResponse] {
    return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ArchiveImportParams]) (*mcp.CallToolResultFor[types.BulkOperationResponse], error) {
        args := params.Arguments
        return withArchiveStore(func(s store.ArchiveStore) (*mcp.CallToolResultFor[types.BulkOperationResponse], error) {
            src := args.File
            if args.Decrypt {
                if args.Key == "" { return nil, types.NewMCPError("MISSING_KEY", "--key required when decrypting", nil) }
                tmp := src + ".dec.tmp"
                if err := decryptFile(src, tmp, args.Key); err != nil { return nil, types.NewMCPError("ARCHIVE_IMPORT", err.Error(), nil) }
                src = tmp
                defer os.Remove(tmp)
            }
            if err := s.Import(src); err != nil { return nil, types.NewMCPError("ARCHIVE_IMPORT", err.Error(), nil) }
            resp := types.BulkOperationResponse{ Succeeded: 1 }
            return &mcp.CallToolResultFor[types.BulkOperationResponse]{ StructuredContent: resp, Content: []mcp.Content{ &mcp.TextContent{ Text: fmt.Sprintf("Imported archive from %s", args.File) } } }, nil
        })
    }
}

func archivePurgeHandler() mcp.ToolHandlerFor[types.ArchivePurgeParams, types.BulkOperationResponse] {
    return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[types.ArchivePurgeParams]) (*mcp.CallToolResultFor[types.BulkOperationResponse], error) {
        args := params.Arguments
        return withArchiveStore(func(s store.ArchiveStore) (*mcp.CallToolResultFor[types.BulkOperationResponse], error) {
            var older *time.Duration
            if args.OlderThan != "" { if d, err := time.ParseDuration(args.OlderThan); err == nil { older = &d } }
            res, err := s.Purge(store.PurgeOptions{ OlderThan: older, DryRun: args.DryRun })
            if err != nil { return nil, types.NewMCPError("ARCHIVE_PURGE", err.Error(), nil) }
            text := "DRY-RUN"
            if !res.DryRun { text = "APPLIED" }
            msg := fmt.Sprintf("%s: considered %d, deleted %d, freed %d bytes", text, res.FilesConsidered, res.FilesDeleted, res.BytesFreed)
            resp := types.BulkOperationResponse{ Succeeded: res.FilesDeleted }
            return &mcp.CallToolResultFor[types.BulkOperationResponse]{ StructuredContent: resp, Content: []mcp.Content{ &mcp.TextContent{ Text: msg } } }, nil
        })
    }
}
