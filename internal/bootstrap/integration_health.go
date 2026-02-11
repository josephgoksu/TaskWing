package bootstrap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/mcpcfg"
)

// Ownership describes who owns a configuration artifact.
type Ownership string

const (
	OwnershipManaged   Ownership = "managed"
	OwnershipUnmanaged Ownership = "unmanaged"
	OwnershipNone      Ownership = "none"
)

// ComponentStatus describes artifact health at the component level.
type ComponentStatus string

const (
	ComponentStatusOK      ComponentStatus = "ok"
	ComponentStatusMissing ComponentStatus = "missing"
	ComponentStatusInvalid ComponentStatus = "invalid"
	ComponentStatusStale   ComponentStatus = "stale"
)

// AIComponent identifies a concrete integration component.
type AIComponent string

const (
	AIComponentCommands  AIComponent = "commands"
	AIComponentHooks     AIComponent = "hooks"
	AIComponentPlugin    AIComponent = "plugin"
	AIComponentMCPLocal  AIComponent = "mcp_local"
	AIComponentMCPGlobal AIComponent = "mcp_global"
)

// IntegrationIssue is a normalized drift signal used by bootstrap + doctor.
type IntegrationIssue struct {
	AI            string          `json:"ai"`
	Component     AIComponent     `json:"component"`
	Ownership     Ownership       `json:"ownership"`
	Status        ComponentStatus `json:"status"`
	Reason        string          `json:"reason"`
	AutoFixable   bool            `json:"auto_fixable"`
	MutatesGlobal bool            `json:"mutates_global"`
	AdoptRequired bool            `json:"adopt_required"`
}

// IntegrationReport is per-AI evaluation output from the shared evaluator.
type IntegrationReport struct {
	AI                    string                          `json:"ai"`
	Issues                []IntegrationIssue              `json:"issues,omitempty"`
	ComponentStatuses     map[AIComponent]ComponentStatus `json:"component_statuses,omitempty"`
	ComponentOwnership    map[AIComponent]Ownership       `json:"component_ownership,omitempty"`
	ManagedLocalDrift     bool                            `json:"managed_local_drift"`
	UnmanagedDrift        bool                            `json:"unmanaged_drift"`
	GlobalMCPDrift        bool                            `json:"global_mcp_drift"`
	CommandsDirExists     bool                            `json:"commands_dir_exists"`
	MarkerFileExists      bool                            `json:"marker_file_exists"`
	CommandFilesCount     int                             `json:"command_files_count"`
	HooksConfigExists     bool                            `json:"hooks_config_exists"`
	HooksConfigValid      bool                            `json:"hooks_config_valid"`
	GlobalMCPExists       bool                            `json:"global_mcp_exists"`
	TaskWingLikeUnmanaged bool                            `json:"taskwing_like_unmanaged"`
}

// RepairAction is an executable fix step derived from integration issues.
type RepairAction struct {
	AI               string      `json:"ai"`
	Component        AIComponent `json:"component"`
	Primitive        string      `json:"primitive"`
	Apply            bool        `json:"apply"`
	Reason           string      `json:"reason,omitempty"`
	MutatesGlobal    bool        `json:"mutates_global"`
	RequiresAdoption bool        `json:"requires_adoption"`
}

// RepairPlan is an ordered collection of remediation actions.
type RepairPlan struct {
	Actions []RepairAction `json:"actions"`
}

// RepairPlanOptions tunes what kind of actions become applicable.
type RepairPlanOptions struct {
	TargetAIs                []string
	IncludeGlobalMutations   bool
	IncludeUnmanagedAdoption bool
}

// EvaluateIntegrations runs a shared health evaluation across all supported AIs.
func EvaluateIntegrations(basePath string, globalMCP map[string]bool) map[string]IntegrationReport {
	reports := make(map[string]IntegrationReport, len(ValidAINames()))
	for _, ai := range ValidAINames() {
		reports[ai] = EvaluateIntegration(basePath, ai, globalMCP[ai])
	}
	return reports
}

// EvaluateIntegration evaluates one AI integration from filesystem + global MCP signal.
func EvaluateIntegration(basePath, aiName string, globalMCPExists bool) IntegrationReport {
	report := IntegrationReport{
		AI:                 aiName,
		ComponentStatuses:  make(map[AIComponent]ComponentStatus),
		ComponentOwnership: make(map[AIComponent]Ownership),
		GlobalMCPExists:    globalMCPExists,
	}

	cfg, ok := aiHelpers[aiName]
	if !ok {
		return report
	}

	commandsStatus, commandsOwner, cmdExists, cmdMarker, cmdCount, cmdTaskwingLike, cmdReason := evalCommandsComponent(basePath, aiName, cfg)
	report.CommandsDirExists = cmdExists
	report.MarkerFileExists = cmdMarker
	report.CommandFilesCount = cmdCount
	report.TaskWingLikeUnmanaged = cmdTaskwingLike && commandsOwner == OwnershipUnmanaged
	report.ComponentStatuses[AIComponentCommands] = commandsStatus
	report.ComponentOwnership[AIComponentCommands] = commandsOwner
	if commandsStatus != ComponentStatusOK &&
		(commandsStatus != ComponentStatusMissing || commandsOwner != OwnershipNone) {
		report.Issues = append(report.Issues, IntegrationIssue{
			AI:            aiName,
			Component:     AIComponentCommands,
			Ownership:     commandsOwner,
			Status:        commandsStatus,
			Reason:        cmdReason,
			AutoFixable:   commandsOwner == OwnershipManaged,
			MutatesGlobal: false,
			AdoptRequired: commandsOwner == OwnershipUnmanaged,
		})
	}
	localConfigured := commandsOwner != OwnershipNone || commandsStatus != ComponentStatusMissing || cmdExists

	if aiName == "claude" || aiName == "codex" {
		if commandsStatus != ComponentStatusMissing {
			hookStatus, hookOwner, hookExists, hookValid, hookReason := evalHooksComponent(basePath, aiName, commandsOwner)
			report.HooksConfigExists = hookExists
			report.HooksConfigValid = hookValid
			report.ComponentStatuses[AIComponentHooks] = hookStatus
			report.ComponentOwnership[AIComponentHooks] = hookOwner
			if hookStatus != ComponentStatusOK {
				report.Issues = append(report.Issues, IntegrationIssue{
					AI:            aiName,
					Component:     AIComponentHooks,
					Ownership:     hookOwner,
					Status:        hookStatus,
					Reason:        hookReason,
					AutoFixable:   hookOwner == OwnershipManaged,
					MutatesGlobal: false,
					AdoptRequired: hookOwner == OwnershipUnmanaged,
				})
			}
		}

		report.ComponentStatuses[AIComponentMCPGlobal] = boolToStatus(globalMCPExists)
		report.ComponentOwnership[AIComponentMCPGlobal] = OwnershipNone
		if localConfigured && !globalMCPExists {
			report.Issues = append(report.Issues, IntegrationIssue{
				AI:            aiName,
				Component:     AIComponentMCPGlobal,
				Ownership:     OwnershipNone,
				Status:        ComponentStatusMissing,
				Reason:        "global taskwing-mcp registration missing",
				AutoFixable:   true,
				MutatesGlobal: true,
				AdoptRequired: false,
			})
		}
	}

	if aiName == "opencode" {
		pluginPath := filepath.Join(basePath, ".opencode", "plugins", "taskwing-hooks.js")
		opencodeConfigPath := filepath.Join(basePath, "opencode.json")
		opencodeConfigured := localConfigured || pathExists(pluginPath) || pathExists(opencodeConfigPath)

		if opencodeConfigured {
			pluginStatus, pluginOwner, pluginReason := evalOpenCodePluginComponent(basePath, commandsOwner)
			report.ComponentStatuses[AIComponentPlugin] = pluginStatus
			report.ComponentOwnership[AIComponentPlugin] = pluginOwner
			if pluginStatus != ComponentStatusOK {
				report.Issues = append(report.Issues, IntegrationIssue{
					AI:            aiName,
					Component:     AIComponentPlugin,
					Ownership:     pluginOwner,
					Status:        pluginStatus,
					Reason:        pluginReason,
					AutoFixable:   pluginOwner == OwnershipManaged,
					MutatesGlobal: false,
					AdoptRequired: pluginOwner == OwnershipUnmanaged,
				})
			}

			localStatus, localReason := evalOpenCodeLocalMCP(basePath)
			report.ComponentStatuses[AIComponentMCPLocal] = localStatus
			report.ComponentOwnership[AIComponentMCPLocal] = OwnershipNone
			if localStatus != ComponentStatusOK {
				report.Issues = append(report.Issues, IntegrationIssue{
					AI:            aiName,
					Component:     AIComponentMCPLocal,
					Ownership:     OwnershipNone,
					Status:        localStatus,
					Reason:        localReason,
					AutoFixable:   true,
					MutatesGlobal: false,
					AdoptRequired: false,
				})
			}
		}
	}

	if aiName == "gemini" || aiName == "cursor" || aiName == "copilot" {
		localMCPPath := localMCPConfigPath(basePath, aiName)
		if localConfigured || pathExists(localMCPPath) {
			localStatus, localReason := evalLocalMCPComponent(basePath, aiName)
			report.ComponentStatuses[AIComponentMCPLocal] = localStatus
			report.ComponentOwnership[AIComponentMCPLocal] = OwnershipNone
			if localStatus != ComponentStatusOK {
				report.Issues = append(report.Issues, IntegrationIssue{
					AI:            aiName,
					Component:     AIComponentMCPLocal,
					Ownership:     OwnershipNone,
					Status:        localStatus,
					Reason:        localReason,
					AutoFixable:   true,
					MutatesGlobal: false,
					AdoptRequired: false,
				})
			}
		}
	}

	for _, issue := range report.Issues {
		switch {
		case issue.MutatesGlobal:
			report.GlobalMCPDrift = true
		case issue.Ownership == OwnershipManaged && !issue.MutatesGlobal:
			report.ManagedLocalDrift = true
		case issue.Ownership == OwnershipUnmanaged && !issue.MutatesGlobal:
			report.UnmanagedDrift = true
		}
	}

	return report
}

// BuildRepairPlan translates integration issues into executable repair actions.
func BuildRepairPlan(reports map[string]IntegrationReport, opts RepairPlanOptions) RepairPlan {
	targetSet := make(map[string]struct{})
	for _, ai := range opts.TargetAIs {
		trimmed := strings.TrimSpace(ai)
		if trimmed != "" {
			targetSet[trimmed] = struct{}{}
		}
	}

	keys := make([]string, 0, len(reports))
	for ai := range reports {
		keys = append(keys, ai)
	}
	sort.Strings(keys)

	plan := RepairPlan{Actions: []RepairAction{}}
	for _, ai := range keys {
		if len(targetSet) > 0 {
			if _, ok := targetSet[ai]; !ok {
				continue
			}
		}
		report := reports[ai]
		for _, issue := range report.Issues {
			action := RepairAction{
				AI:               issue.AI,
				Component:        issue.Component,
				Primitive:        primitiveForComponent(issue.Component),
				Apply:            issue.AutoFixable,
				Reason:           issue.Reason,
				MutatesGlobal:    issue.MutatesGlobal,
				RequiresAdoption: issue.AdoptRequired,
			}
			if issue.MutatesGlobal && !opts.IncludeGlobalMutations {
				action.Apply = false
				action.Reason = "global mutation disabled"
			}
			if issue.AdoptRequired {
				action.Primitive = "adopt_and_" + action.Primitive
				action.Apply = opts.IncludeUnmanagedAdoption
				if !opts.IncludeUnmanagedAdoption {
					action.Reason = "adoption required (use --adopt-unmanaged)"
				}
			}
			plan.Actions = append(plan.Actions, action)
		}
	}

	return plan
}

func primitiveForComponent(component AIComponent) string {
	switch component {
	case AIComponentCommands:
		return "repairCommands"
	case AIComponentHooks:
		return "repairHooks"
	case AIComponentPlugin:
		return "repairPlugin"
	case AIComponentMCPLocal:
		return "repairLocalMCP"
	case AIComponentMCPGlobal:
		return "repairGlobalMCP"
	default:
		return "repairUnknown"
	}
}

func boolToStatus(ok bool) ComponentStatus {
	if ok {
		return ComponentStatusOK
	}
	return ComponentStatusMissing
}

func evalCommandsComponent(basePath, aiName string, cfg aiHelperConfig) (ComponentStatus, Ownership, bool, bool, int, bool, string) {
	if cfg.singleFile {
		filePath := filepath.Join(basePath, cfg.commandsDir, cfg.singleFileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return ComponentStatusMissing, OwnershipNone, false, false, 0, false, fmt.Sprintf("%s missing", cfg.singleFileName)
		}
		text := string(content)
		managed := strings.Contains(text, "<!-- TASKWING_MANAGED -->")
		taskwingLike := managed || strings.Contains(strings.ToLower(text), "taskwing")
		if managed {
			version := parseEmbeddedVersion(text)
			if version != "" && version != AIToolConfigVersion(aiName) {
				return ComponentStatusStale, OwnershipManaged, true, true, 1, true, "managed instructions version mismatch"
			}
			return ComponentStatusOK, OwnershipManaged, true, true, 1, true, ""
		}
		if taskwingLike {
			return ComponentStatusStale, OwnershipUnmanaged, true, false, 1, true, "taskwing-like unmanaged instructions detected"
		}
		return ComponentStatusOK, OwnershipUnmanaged, true, false, 1, false, ""
	}

	commandsDir := filepath.Join(basePath, cfg.commandsDir)
	info, err := os.Stat(commandsDir)
	if err != nil || !info.IsDir() {
		return ComponentStatusMissing, OwnershipNone, false, false, 0, false, "commands directory missing"
	}

	markerPath := filepath.Join(commandsDir, TaskWingManagedFile)
	_, markerErr := os.Stat(markerPath)
	managed := markerErr == nil
	ownership := OwnershipUnmanaged
	if managed {
		ownership = OwnershipManaged
	}

	expected := expectedSlashCommandFiles(cfg.fileExt)
	entries, _ := os.ReadDir(commandsDir)
	actual := map[string]struct{}{}
	commandFileCount := 0
	taskwingLike := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, cfg.fileExt) {
			commandFileCount++
			actual[name] = struct{}{}
			if strings.HasPrefix(strings.TrimSuffix(name, cfg.fileExt), "tw-") {
				taskwingLike = true
			}
		}
	}

	missing := make([]string, 0)
	for name := range expected {
		if _, ok := actual[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)

	if !taskwingLike {
		matches, _ := filepath.Glob(filepath.Join(commandsDir, "*."+strings.TrimPrefix(cfg.fileExt, ".")))
		for _, match := range matches {
			b, readErr := os.ReadFile(match)
			if readErr != nil {
				continue
			}
			if strings.Contains(strings.ToLower(string(b)), "taskwing slash") || strings.Contains(strings.ToLower(string(b)), "!taskwing") {
				taskwingLike = true
				break
			}
		}
	}

	if ownership == OwnershipManaged {
		if len(missing) > 0 {
			return ComponentStatusMissing, ownership, true, true, commandFileCount, taskwingLike, fmt.Sprintf("missing expected command files: %s", strings.Join(missing, ", "))
		}
		markerVersion := parseManagedMarkerVersion(markerPath)
		if markerVersion != "" && markerVersion != AIToolConfigVersion(aiName) {
			return ComponentStatusStale, ownership, true, true, commandFileCount, taskwingLike, "managed marker version mismatch"
		}
		if markerVersion == "" {
			return ComponentStatusStale, ownership, true, true, commandFileCount, taskwingLike, "managed marker missing version"
		}
		return ComponentStatusOK, ownership, true, true, commandFileCount, taskwingLike, ""
	}

	if commandFileCount == 0 {
		return ComponentStatusStale, ownership, true, false, commandFileCount, false, "commands directory exists but no command files"
	}

	if taskwingLike {
		if len(missing) > 0 {
			return ComponentStatusStale, ownership, true, false, commandFileCount, true, fmt.Sprintf("taskwing-like unmanaged directory missing expected files: %s", strings.Join(missing, ", "))
		}
		return ComponentStatusStale, ownership, true, false, commandFileCount, true, "taskwing-like unmanaged directory (adoption recommended)"
	}

	return ComponentStatusOK, ownership, true, false, commandFileCount, false, ""
}

func evalHooksComponent(basePath, aiName string, commandsOwnership Ownership) (ComponentStatus, Ownership, bool, bool, string) {
	settingsPath := filepath.Join(basePath, "."+aiName, "settings.json")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		owner := commandsOwnership
		if owner == OwnershipNone {
			owner = OwnershipManaged
		}
		return ComponentStatusMissing, owner, false, false, "hooks config missing"
	}

	owner := commandsOwnership
	if owner == OwnershipNone {
		if strings.Contains(strings.ToLower(string(content)), "taskwing hook") {
			owner = OwnershipUnmanaged
		} else {
			owner = OwnershipManaged
		}
	}

	var parsed map[string]any
	if err := json.Unmarshal(content, &parsed); err != nil {
		return ComponentStatusInvalid, owner, true, false, "hooks config invalid JSON"
	}

	hooksRaw, ok := parsed["hooks"]
	if !ok {
		return ComponentStatusInvalid, owner, true, true, "hooks key missing"
	}
	hooksMap, ok := hooksRaw.(map[string]any)
	if !ok {
		return ComponentStatusInvalid, owner, true, true, "hooks key has invalid type"
	}

	if _, hasStop := hooksMap["Stop"]; !hasStop {
		return ComponentStatusInvalid, owner, true, true, "required Stop hook missing"
	}
	if !HookEventContainsCommand(hooksMap, "Stop", "hook continue-check") {
		return ComponentStatusInvalid, owner, true, true, "required Stop hook command missing taskwing continue-check"
	}
	missingRecommended := make([]string, 0)
	if _, hasSessionStart := hooksMap["SessionStart"]; !hasSessionStart {
		missingRecommended = append(missingRecommended, "SessionStart")
	} else if !HookEventContainsCommand(hooksMap, "SessionStart", "hook session-init") {
		missingRecommended = append(missingRecommended, "SessionStart(command)")
	}
	if _, hasSessionEnd := hooksMap["SessionEnd"]; !hasSessionEnd {
		missingRecommended = append(missingRecommended, "SessionEnd")
	} else if !HookEventContainsCommand(hooksMap, "SessionEnd", "hook session-end") {
		missingRecommended = append(missingRecommended, "SessionEnd(command)")
	}
	if len(missingRecommended) > 0 {
		return ComponentStatusStale, owner, true, true, fmt.Sprintf("recommended hooks missing: %s", strings.Join(missingRecommended, ", "))
	}

	return ComponentStatusOK, owner, true, true, ""
}

// HookEventContainsCommand returns true when the hook event contains a command
// entry whose command field includes requiredSubstr (case-insensitive).
func HookEventContainsCommand(hooksMap map[string]any, eventName, requiredSubstr string) bool {
	rawEvent, ok := hooksMap[eventName]
	if !ok {
		return false
	}
	eventEntries, ok := rawEvent.([]any)
	if !ok {
		return false
	}
	required := strings.ToLower(strings.TrimSpace(requiredSubstr))
	if required == "" {
		return false
	}

	for _, entry := range eventEntries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		rawHooks, ok := entryMap["hooks"]
		if !ok {
			continue
		}
		hookCommands, ok := rawHooks.([]any)
		if !ok {
			continue
		}
		for _, cmdEntry := range hookCommands {
			cmdMap, ok := cmdEntry.(map[string]any)
			if !ok {
				continue
			}
			cmdStr, _ := cmdMap["command"].(string)
			if strings.Contains(strings.ToLower(cmdStr), required) {
				return true
			}
		}
	}

	return false
}

func evalOpenCodePluginComponent(basePath string, commandsOwnership Ownership) (ComponentStatus, Ownership, string) {
	pluginPath := filepath.Join(basePath, ".opencode", "plugins", "taskwing-hooks.js")
	content, err := os.ReadFile(pluginPath)
	if err != nil {
		owner := commandsOwnership
		if owner == OwnershipNone {
			owner = OwnershipManaged
		}
		return ComponentStatusMissing, owner, "OpenCode plugin missing"
	}

	text := string(content)
	managed := strings.Contains(text, "TASKWING_MANAGED_PLUGIN")
	owner := OwnershipUnmanaged
	if managed {
		owner = OwnershipManaged
	}
	taskwingLike := managed || strings.Contains(strings.ToLower(text), "taskwing hook")

	requiredFragments := []string{"session.created", "session.idle", "taskwing hook session-init", "taskwing hook continue-check"}
	for _, fragment := range requiredFragments {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(fragment)) {
			if owner == OwnershipManaged {
				return ComponentStatusStale, owner, fmt.Sprintf("plugin missing required fragment: %s", fragment)
			}
			if taskwingLike {
				return ComponentStatusStale, owner, fmt.Sprintf("taskwing-like unmanaged plugin missing required fragment: %s", fragment)
			}
			return ComponentStatusOK, owner, ""
		}
	}

	if owner == OwnershipUnmanaged && taskwingLike {
		return ComponentStatusStale, owner, "taskwing-like unmanaged OpenCode plugin"
	}

	return ComponentStatusOK, owner, ""
}

func evalLocalMCPComponent(basePath, aiName string) (ComponentStatus, string) {
	switch aiName {
	case "gemini":
		path := filepath.Join(basePath, ".gemini", "settings.json")
		return hasTaskWingInMapJSON(path, "mcpServers")
	case "cursor":
		path := filepath.Join(basePath, ".cursor", "mcp.json")
		return hasTaskWingInMapJSON(path, "mcpServers")
	case "copilot":
		path := filepath.Join(basePath, ".vscode", "mcp.json")
		return hasTaskWingInMapJSON(path, "servers")
	default:
		return ComponentStatusOK, ""
	}
}

func localMCPConfigPath(basePath, aiName string) string {
	switch aiName {
	case "gemini":
		return filepath.Join(basePath, ".gemini", "settings.json")
	case "cursor":
		return filepath.Join(basePath, ".cursor", "mcp.json")
	case "copilot":
		return filepath.Join(basePath, ".vscode", "mcp.json")
	default:
		return ""
	}
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func evalOpenCodeLocalMCP(basePath string) (ComponentStatus, string) {
	path := filepath.Join(basePath, "opencode.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return ComponentStatusMissing, "opencode.json missing"
	}
	var parsed map[string]any
	if err := json.Unmarshal(content, &parsed); err != nil {
		return ComponentStatusInvalid, "opencode.json invalid JSON"
	}
	mcpRaw, ok := parsed["mcp"]
	if !ok {
		return ComponentStatusMissing, "opencode.json missing mcp section"
	}
	mcpMap, ok := mcpRaw.(map[string]any)
	if !ok {
		return ComponentStatusInvalid, "opencode.json mcp section has invalid type"
	}
	for name := range mcpMap {
		if mcpcfg.IsLegacyServerName(name) {
			return ComponentStatusInvalid, fmt.Sprintf("non-canonical MCP server key %q found (expected %q)", name, mcpcfg.CanonicalServerName)
		}
	}
	raw, ok := mcpMap[mcpcfg.CanonicalServerName]
	if !ok {
		return ComponentStatusMissing, mcpcfg.CanonicalServerName + " entry missing in opencode.json"
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return ComponentStatusInvalid, mcpcfg.CanonicalServerName + " entry has invalid format"
	}
	typeStr, _ := entry["type"].(string)
	if typeStr != "local" {
		return ComponentStatusInvalid, mcpcfg.CanonicalServerName + " entry type must be local"
	}
	cmdRaw, ok := entry["command"].([]any)
	if !ok || len(cmdRaw) < 2 {
		return ComponentStatusInvalid, mcpcfg.CanonicalServerName + " command must be array with binary and mcp"
	}
	last, _ := cmdRaw[len(cmdRaw)-1].(string)
	if strings.TrimSpace(last) != "mcp" {
		return ComponentStatusInvalid, mcpcfg.CanonicalServerName + " command must end with 'mcp'"
	}
	return ComponentStatusOK, ""
}

func hasTaskWingInMapJSON(path, rootKey string) (ComponentStatus, string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return ComponentStatusMissing, fmt.Sprintf("%s missing", path)
	}
	var parsed map[string]any
	if err := json.Unmarshal(content, &parsed); err != nil {
		return ComponentStatusInvalid, fmt.Sprintf("invalid JSON in %s", path)
	}
	raw, ok := parsed[rootKey]
	if !ok {
		return ComponentStatusMissing, fmt.Sprintf("%s missing %s", path, rootKey)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return ComponentStatusInvalid, fmt.Sprintf("%s has invalid %s format", path, rootKey)
	}
	if _, ok := m[mcpcfg.CanonicalServerName]; ok {
		return ComponentStatusOK, ""
	}
	for name := range m {
		if mcpcfg.IsLegacyServerName(name) {
			return ComponentStatusInvalid, fmt.Sprintf("non-canonical MCP server key %q found (expected %q)", name, mcpcfg.CanonicalServerName)
		}
	}
	return ComponentStatusMissing, fmt.Sprintf("%s missing in %s", mcpcfg.CanonicalServerName, path)
}

func parseManagedMarkerVersion(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(strings.ToLower(line), "# version:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# Version:"))
		}
	}
	return ""
}

func parseEmbeddedVersion(content string) string {
	const prefix = "<!-- Version:"
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			trimmed := strings.TrimPrefix(line, prefix)
			trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "-->"))
			return trimmed
		}
	}
	return ""
}

// ManagedLocalDriftAIs returns all AIs with managed local drift.
func ManagedLocalDriftAIs(reports map[string]IntegrationReport) []string {
	out := make([]string, 0)
	for ai, report := range reports {
		if report.ManagedLocalDrift {
			out = append(out, ai)
		}
	}
	sort.Strings(out)
	return out
}

// UnmanagedDriftAIs returns all AIs with unmanaged drift.
func UnmanagedDriftAIs(reports map[string]IntegrationReport) []string {
	out := make([]string, 0)
	for ai, report := range reports {
		if report.UnmanagedDrift {
			out = append(out, ai)
		}
	}
	sort.Strings(out)
	return out
}

// GlobalMCPDriftAIs returns all AIs with global MCP drift.
func GlobalMCPDriftAIs(reports map[string]IntegrationReport) []string {
	out := make([]string, 0)
	for ai, report := range reports {
		if report.GlobalMCPDrift {
			out = append(out, ai)
		}
	}
	sort.Strings(out)
	return out
}

// HasManagedLocalDrift checks whether any AI has managed local drift.
func HasManagedLocalDrift(reports map[string]IntegrationReport) bool {
	for _, report := range reports {
		if report.ManagedLocalDrift {
			return true
		}
	}
	return false
}

// AIHasIssueComponent reports whether the AI has an issue for a component.
func AIHasIssueComponent(report IntegrationReport, component AIComponent) bool {
	for _, issue := range report.Issues {
		if issue.Component == component {
			return true
		}
	}
	return false
}

// IsTaskWingLikeUnmanaged determines whether an unmanaged config resembles TaskWing output.
func IsTaskWingLikeUnmanaged(report IntegrationReport) bool {
	return report.TaskWingLikeUnmanaged || (report.UnmanagedDrift && slices.ContainsFunc(report.Issues, func(issue IntegrationIssue) bool {
		return issue.Ownership == OwnershipUnmanaged && issue.AdoptRequired
	}))
}
