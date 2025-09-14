package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gofrs/flock"
	"github.com/google/uuid"
	"github.com/josephgoksu/TaskWing/models"
	yaml "gopkg.in/yaml.v3"
)

const (
	defaultDataFile   = "tasks.json" // Default filename if only format implies extension
	dataFileKey       = "dataFile"
	dataFileFormatKey = "dataFileFormat"
	defaultDataFormat = "json"
	formatJSON        = "json"
	formatYAML        = "yaml"
	formatTOML        = "toml"
	checksumSuffix    = ".checksum"
)

// FileTaskStore implements the TaskStore interface using a file backend.
// It supports JSON, YAML, and TOML formats and uses file-level locking.
type FileTaskStore struct {
	filePath string
	tasks    map[string]models.Task
	// mu       sync.RWMutex // Replaced by flock
	flk    *flock.Flock
	format string // Stores the data format: "json", "yaml", or "toml"
}

// NewFileTaskStore creates a new instance of FileTaskStore.
// It does not initialize the store; Initialize must be called separately.
func NewFileTaskStore() *FileTaskStore {
	return &FileTaskStore{
		tasks: make(map[string]models.Task),
	}
}

// Initialize configures the FileTaskStore.
// It expects a 'dataFile' key in the config map specifying the path to the data file.
// If not provided, it defaults to 'tasks.json' in the current working directory.
// It loads existing tasks from the file if it exists and establishes a file lock.
func (s *FileTaskStore) Initialize(config map[string]string) error {
	if val, ok := config[dataFileKey]; ok && val != "" {
		s.filePath = val
	} else {
		// If filePath is not given, try to infer from format or use a default name with default format
		s.filePath = defaultDataFile // This might need adjustment based on format below
	}

	if val, ok := config[dataFileFormatKey]; ok && val != "" {
		formatLower := strings.ToLower(val)
		switch formatLower {
		case formatJSON, formatYAML, formatTOML:
			s.format = formatLower
		default:
			return fmt.Errorf("unsupported dataFileFormat: %s. Supported formats are json, yaml, toml", val)
		}
	} else {
		s.format = defaultDataFormat
	}

	// If filePath was the default and format is not JSON, adjust default filePath extension
	// This is a simple heuristic. Users providing a full filePath are responsible for its extension.
	if s.filePath == defaultDataFile && s.format != formatJSON {
		ext := filepath.Ext(s.filePath)
		s.filePath = strings.TrimSuffix(s.filePath, ext) + "." + s.format
	}

	// Ensure the directory for the file path exists
	dir := filepath.Dir(s.filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Initialize flock for the data file. The lock file will be s.filePath + ".lock"
	// or on some systems, the lock is directly on the file.
	s.flk = flock.New(s.filePath) // flock uses the file path itself for locking

	// Attempt to acquire an exclusive lock for initialization (e.g. creating file if not exists)
	// then downgrade or release based on operation. For initial load, a shared lock is enough.
	locked, err := s.flk.TryLock() // Try to get an exclusive lock for initial setup/check
	if err != nil {
		return fmt.Errorf("failed to acquire initial lock for %s: %w", s.filePath, err)
	}
	if !locked {
		// Could wait here or return an error. For now, assume we should get it.
		// This might happen if another process has an exclusive lock.
		// Let's try a blocking lock to ensure initialization completes if the file is momentarily locked.
		if err := s.flk.Lock(); err != nil {
			return fmt.Errorf("failed to acquire blocking initial lock for %s: %w", s.filePath, err)
		}
	}
	defer func() { _ = s.flk.Unlock() }() // Unlock after initialization sequence

	s.tasks = make(map[string]models.Task) // Reset tasks map
	return s.loadTasksFromFileInternal()   // Use internal version that assumes lock is held
}

// calculateChecksum computes the SHA256 checksum of the given data.
func calculateChecksum(data []byte) string {
	hasher := sha256.New()
	hasher.Write(data) // Write never returns an error
	return hex.EncodeToString(hasher.Sum(nil))
}

// loadTasksFromFileInternal reads tasks from the file, verifies checksum, and unmarshals.
func (s *FileTaskStore) loadTasksFromFileInternal() error {
	checksumFilePath := s.filePath + checksumSuffix

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.tasks = make(map[string]models.Task)
			// If data file doesn't exist, checksum file shouldn't either. Clean up if it does.
			_ = os.Remove(checksumFilePath)
			if f, createErr := os.OpenFile(s.filePath, os.O_CREATE|os.O_RDWR, 0o644); createErr != nil {
				return fmt.Errorf("failed to create data file %s: %w", s.filePath, createErr)
			} else {
				_ = f.Close()
			}
			// Create an empty checksum file for a new empty data file
			if err := os.WriteFile(checksumFilePath, []byte(calculateChecksum([]byte{})), 0o644); err != nil {
				// Non-critical, log or ignore. The next save will attempt to create it.
				fmt.Printf("Warning: could not write initial checksum file %s: %v\n", checksumFilePath, err)
			}
			return nil
		}
		return fmt.Errorf("failed to read data file %s: %w", s.filePath, err)
	}

	// Verify checksum if checksum file exists
	if _, err := os.Stat(checksumFilePath); err == nil {
		expectedChecksumBytes, readErr := os.ReadFile(checksumFilePath)
		if readErr != nil {
			return fmt.Errorf("failed to read checksum file %s: %w - data file might be corrupt or tampered", checksumFilePath, readErr)
		}
		expectedChecksum := strings.TrimSpace(string(expectedChecksumBytes))
		actualChecksum := calculateChecksum(data)

		if actualChecksum != expectedChecksum {
			return fmt.Errorf("checksum mismatch for %s - expected %s, got %s - file is corrupt or tampered", s.filePath, expectedChecksum, actualChecksum)
		}
	} else if !os.IsNotExist(err) {
		// Some other error trying to stat checksum file (e.g. permission denied)
		return fmt.Errorf("error checking checksum file %s: %w", checksumFilePath, err)
	}
	// If checksum file does not exist, and data file exists (and possibly non-empty),
	// this could be data from before checksums. We'll allow loading it,
	// and the next save will create a checksum file.

	if len(data) == 0 {
		// If data is empty, ensure checksum reflects this or is created.
		currentChecksum := calculateChecksum([]byte{})
		_ = os.WriteFile(checksumFilePath, []byte(currentChecksum), 0o644) // best effort
		s.tasks = make(map[string]models.Task)
		return nil
	}

	var taskList models.TaskList
	switch s.format {
	case formatJSON:
		if err := json.Unmarshal(data, &taskList); err != nil {
			return fmt.Errorf("failed to unmarshal JSON from %s (checksum may have passed): %w", s.filePath, err)
		}
	case formatYAML:
		if err := yaml.Unmarshal(data, &taskList); err != nil {
			return fmt.Errorf("failed to unmarshal YAML from %s (checksum may have passed): %w", s.filePath, err)
		}
	case formatTOML:
		if err := toml.Unmarshal(data, &taskList); err != nil {
			return fmt.Errorf("failed to unmarshal TOML from %s (checksum may have passed): %w", s.filePath, err)
		}
	default:
		return fmt.Errorf("unsupported data format for loading: %s", s.format)
	}

	s.tasks = make(map[string]models.Task, len(taskList.Tasks))
	for _, task := range taskList.Tasks {
		s.tasks[task.ID] = task
	}
	return nil
}

// saveTasksToFileInternal writes tasks to file, then writes its checksum.
func (s *FileTaskStore) saveTasksToFileInternal() error {
	taskList := models.TaskList{
		Tasks:      make([]models.Task, 0, len(s.tasks)),
		TotalCount: len(s.tasks),
	}
	for _, task := range s.tasks {
		taskList.Tasks = append(taskList.Tasks, task)
	}

	var marshaledData []byte
	var err error

	switch s.format {
	case formatJSON:
		marshaledData, err = json.MarshalIndent(taskList, "", "  ")
	case formatYAML:
		marshaledData, err = yaml.Marshal(taskList)
	case formatTOML:
		buf := new(bytes.Buffer)
		if encodeErr := toml.NewEncoder(buf).Encode(taskList); encodeErr == nil {
			marshaledData = buf.Bytes()
		} else {
			err = fmt.Errorf("failed to marshal TOML: %w", encodeErr)
		}
	default:
		return fmt.Errorf("unsupported data format for saving: %s", s.format)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal tasks to %s: %w", s.format, err)
	}

	tempFilePath := s.filePath + ".tmp"
	checksumFilePath := s.filePath + checksumSuffix
	tempChecksumFilePath := checksumFilePath + ".tmp"

	defer func() { _ = os.Remove(tempFilePath) }()
	defer func() { _ = os.Remove(tempChecksumFilePath) }()

	if err := os.WriteFile(tempFilePath, marshaledData, 0o644); err != nil {
		return fmt.Errorf("failed to write to temporary data file %s: %w", tempFilePath, err)
	}

	// Data file written to temp, now calculate its checksum
	actualChecksum := calculateChecksum(marshaledData)
	if err := os.WriteFile(tempChecksumFilePath, []byte(actualChecksum), 0o644); err != nil {
		return fmt.Errorf("failed to write to temporary checksum file %s: %w", tempChecksumFilePath, err)
	}

	// Atomically move data file and then checksum file
	if err := os.Rename(tempFilePath, s.filePath); err != nil {
		return fmt.Errorf("failed to rename temporary data file %s to %s: %w", tempFilePath, s.filePath, err)
	}
	// If renaming data file succeeded, then rename checksum file
	if err := os.Rename(tempChecksumFilePath, checksumFilePath); err != nil {
		// This is a problematic state: data file is updated, checksum file is not.
		// Attempt to remove the new data file to revert to a potentially more consistent state (old data, old checksum or no checksum)
		// Or, log prominently and alert. For now, return error and log this potential inconsistency.
		// A more robust solution might try to write the checksum again, or remove the main file if this fails.
		return fmt.Errorf("CRITICAL: data file %s updated, but failed to update checksum file %s from %s: %w - store may be inconsistent", s.filePath, checksumFilePath, tempChecksumFilePath, err)
	}

	return nil
}

// generateID creates a new universally unique identifier string.
func generateID() string {
	return uuid.NewString()
}

// addStringToSliceIfMissing adds a string to a slice if it's not already present.
// Returns the new slice.
func addStringToSliceIfMissing(slice []string, item string) []string {
	if !slices.Contains(slice, item) {
		return append(slice, item)
	}
	return slice
}

// removeStringFromSlice removes all occurrences of a string from a slice.
// Returns the new slice.
func removeStringFromSlice(slice []string, item string) []string {
	newSlice := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			newSlice = append(newSlice, s)
		}
	}
	return newSlice
}

// CreateTask adds a new task to the store.
// It sets the ID, timestamps, and manages relationship consistency.
func (s *FileTaskStore) CreateTask(task models.Task) (models.Task, error) {
	if err := s.flk.Lock(); err != nil {
		return models.Task{}, fmt.Errorf("could not lock file for create: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	// Reload state from disk to ensure we are working with the latest version
	// in case of concurrent access, though the lock should serialize operations.
	if err := s.loadTasksFromFileInternal(); err != nil {
		return models.Task{}, fmt.Errorf("failed to reload tasks before create: %w", err)
	}

	// If ID is empty, generate one. Otherwise, validate the provided one.
	if task.ID == "" {
		task.ID = generateID()
	} else {
		if _, exists := s.tasks[task.ID]; exists {
			return models.Task{}, fmt.Errorf("task with ID '%s' already exists", task.ID)
		}
	}

	now := time.Now().UTC()
	task.CreatedAt = now
	task.UpdatedAt = now
	task.Dependents = []string{} // Ensure dependents is initialized and empty

	// Validate the task struct before proceeding
	if err := models.ValidateStruct(task); err != nil {
		return models.Task{}, fmt.Errorf("validation failed for new task: %w", err)
	}

	// --- Relationship Management ---

	// 1. Handle Parent-Child relationship
	if task.ParentID != nil && *task.ParentID != "" {
		parentTask, exists := s.tasks[*task.ParentID]
		if !exists {
			return models.Task{}, fmt.Errorf("parent task with ID '%s' not found", *task.ParentID)
		}
		parentTask.SubtaskIDs = addStringToSliceIfMissing(parentTask.SubtaskIDs, task.ID)
		parentTask.UpdatedAt = now
		s.tasks[*task.ParentID] = parentTask
	}

	// 2. Handle Dependencies
	if len(task.Dependencies) > 0 {
		for _, depID := range task.Dependencies {
			if depID == task.ID {
				return models.Task{}, fmt.Errorf("task cannot depend on itself")
			}
			dependencyTask, exists := s.tasks[depID]
			if !exists {
				return models.Task{}, fmt.Errorf("dependency task with ID '%s' not found", depID)
			}
			// Add this new task's ID to the dependent list of the task it depends on.
			dependencyTask.Dependents = addStringToSliceIfMissing(dependencyTask.Dependents, task.ID)
			dependencyTask.UpdatedAt = now
			s.tasks[depID] = dependencyTask
		}
	}

	s.tasks[task.ID] = task

	if err := s.saveTasksToFileInternal(); err != nil {
		// Attempt to revert in-memory changes on save failure.
		// This is best-effort and a transactional approach would be more robust.
		// For now, reloading from the unchanged file is the simplest "rollback".
		_ = s.loadTasksFromFileInternal()
		return models.Task{}, fmt.Errorf("failed to save new task: %w", err)
	}

	return task, nil
}

// GetTask retrieves a task by its unique identifier.
// This function assumes that a read lock (s.flk.RLock()) is already held by the caller if needed for safety with other ops.
// However, for a single Get, an exclusive lock for load might be simpler if tasks aren't kept hot.
// For simplicity and consistency with other ops, using a read lock.
func (s *FileTaskStore) GetTask(id string) (models.Task, error) {
	if err := s.flk.Lock(); err != nil { // Using exclusive lock to ensure fresh load and safety
		return models.Task{}, fmt.Errorf("failed to acquire lock for GetTask: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return models.Task{}, fmt.Errorf("failed to load tasks for GetTask: %w", err)
	}

	task, ok := s.tasks[id]
	if !ok {
		return models.Task{}, fmt.Errorf("task with ID %s not found", id)
	}
	return task, nil
}

// fieldNameMapping maps JSON field names to struct field names
var fieldNameMapping = map[string]string{
	"id":                 "ID",
	"title":              "Title",
	"description":        "Description",
	"acceptanceCriteria": "AcceptanceCriteria",
	"status":             "Status",
	"parentId":           "ParentID",
	"subtaskIds":         "SubtaskIDs",
	"dependencies":       "Dependencies",
	"dependents":         "Dependents",
	"priority":           "Priority",
	"createdAt":          "CreatedAt",
	"updatedAt":          "UpdatedAt",
	"completedAt":        "CompletedAt",
}

// UpdateTask modifies an existing task.
// It uses a map of updates and ensures relationship consistency (parents, dependencies).
func (s *FileTaskStore) UpdateTask(id string, updates map[string]interface{}) (models.Task, error) {
	if err := s.flk.Lock(); err != nil {
		return models.Task{}, fmt.Errorf("could not lock file for update: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return models.Task{}, fmt.Errorf("failed to reload tasks before update: %w", err)
	}

	task, exists := s.tasks[id]
	if !exists {
		return models.Task{}, fmt.Errorf("task with ID '%s' not found", id)
	}
	originalTask := task // Keep a copy for potential rollback

	now := time.Now().UTC()
	task.UpdatedAt = now

	// Apply updates reflectively.
	for key, value := range updates {
		// Handle relationship fields separately.
		if key == "parentId" || key == "dependencies" {
			continue
		}
		// Use field name mapping to get correct struct field name
		fieldName, ok := fieldNameMapping[key]
		if !ok {
			// Use exact field name from struct
			// Simple title case conversion for ASCII field names
			if len(key) > 0 {
				fieldName = strings.ToUpper(key[:1]) + key[1:]
			}
		}

		// Use reflection to set field value.
		field := reflect.ValueOf(&task).Elem().FieldByName(fieldName)
		if field.IsValid() && field.CanSet() {
			val := reflect.ValueOf(value)
			// Handle type conversion for fields that need it (e.g., string to custom type)
			if field.Type() != val.Type() {
				if converted, err := convertType(val.Interface(), field.Type()); err == nil {
					val = converted
				} else {
					return models.Task{}, fmt.Errorf("type conversion error for field %s: %w", key, err)
				}
			}
			field.Set(val)
		}
	}

	// Handle parent change
	if newParentID, ok := updates["parentId"]; ok {
		if err := s.updateParentLink(&task, newParentID, now); err != nil {
			return models.Task{}, err
		}
	}

	// Handle dependencies change
	if newDeps, ok := updates["dependencies"]; ok {
		if err := s.updateDependenciesLink(&task, newDeps, now); err != nil {
			return models.Task{}, err
		}
	}

	// Validate the updated task struct.
	if err := models.ValidateStruct(task); err != nil {
		return models.Task{}, fmt.Errorf("validation failed for updated task: %w", err)
	}

	s.tasks[id] = task

	if err := s.saveTasksToFileInternal(); err != nil {
		s.tasks[id] = originalTask // Rollback in-memory change
		// More complex rollback for parent/dependency changes would be needed for full atomicity.
		// For now, this is a best-effort rollback of the primary task.
		return models.Task{}, fmt.Errorf("failed to save updated task: %w", err)
	}

	return task, nil
}

// updateParentLink handles the logic for changing a task's parent.
func (s *FileTaskStore) updateParentLink(task *models.Task, newParentIDValue interface{}, now time.Time) error {
	var newParentID *string
	if newParentIDValue != nil {
		idStr, ok := newParentIDValue.(string)
		if !ok {
			return fmt.Errorf("invalid type for parentId; must be a string or nil")
		}
		newParentID = &idStr
	}

	// Prevent self-parenting
	if newParentID != nil && *newParentID == task.ID {
		return fmt.Errorf("task cannot be its own parent")
	}

	oldParentID := task.ParentID

	// No change, do nothing.
	if (oldParentID == nil && newParentID == nil) || (oldParentID != nil && newParentID != nil && *oldParentID == *newParentID) {
		return nil
	}

	// Remove from old parent's SubtaskIDs
	if oldParentID != nil && *oldParentID != "" {
		if oldParent, ok := s.tasks[*oldParentID]; ok {
			oldParent.SubtaskIDs = removeStringFromSlice(oldParent.SubtaskIDs, task.ID)
			oldParent.UpdatedAt = now
			s.tasks[*oldParentID] = oldParent
		}
	}

	// Add to new parent's SubtaskIDs
	if newParentID != nil && *newParentID != "" {
		if newParent, ok := s.tasks[*newParentID]; ok {
			// Check for circular dependency (new parent cannot be a subtask of the current task)
			if s.isSubtask(newParent, task.ID) {
				return fmt.Errorf("cannot set parent: '%s' is a subtask of '%s'", newParent.Title, task.Title)
			}
			newParent.SubtaskIDs = addStringToSliceIfMissing(newParent.SubtaskIDs, task.ID)
			newParent.UpdatedAt = now
			s.tasks[*newParentID] = newParent
		} else {
			return fmt.Errorf("new parent task with ID '%s' not found", *newParentID)
		}
	}

	task.ParentID = newParentID
	return nil
}

// isSubtask checks if a potential parent task is actually a subtask of the given task.
func (s *FileTaskStore) isSubtask(potentialParent models.Task, originalTaskID string) bool {
	if potentialParent.ParentID == nil {
		return false
	}
	if *potentialParent.ParentID == originalTaskID {
		return true
	}
	// Recurse up the chain
	if grandParent, ok := s.tasks[*potentialParent.ParentID]; ok {
		return s.isSubtask(grandParent, originalTaskID)
	}
	return false
}

// updateDependenciesLink handles the logic for changing a task's dependencies.
func (s *FileTaskStore) updateDependenciesLink(task *models.Task, newDepsValue interface{}, now time.Time) error {
	newDeps, ok := newDepsValue.([]string)
	if !ok {
		return fmt.Errorf("invalid type for dependencies; must be a []string")
	}

	oldDeps := task.Dependencies
	task.Dependencies = newDeps // Set new dependencies on the task struct

	// Determine which dependencies were added and which were removed.
	depsToAdd := []string{}
	depsToRemove := []string{}

	oldDepSet := make(map[string]bool)
	for _, id := range oldDeps {
		oldDepSet[id] = true
	}
	newDepSet := make(map[string]bool)
	for _, id := range newDeps {
		newDepSet[id] = true
	}

	for id := range newDepSet {
		if !oldDepSet[id] {
			depsToAdd = append(depsToAdd, id)
		}
	}
	for id := range oldDepSet {
		if !newDepSet[id] {
			depsToRemove = append(depsToRemove, id)
		}
	}

	// Remove this task from the Dependents list of tasks that are no longer dependencies.
	for _, depID := range depsToRemove {
		if depTask, ok := s.tasks[depID]; ok {
			depTask.Dependents = removeStringFromSlice(depTask.Dependents, task.ID)
			depTask.UpdatedAt = now
			s.tasks[depID] = depTask
		}
	}

	// Add this task to the Dependents list of new dependencies.
	for _, depID := range depsToAdd {
		if depID == task.ID {
			return fmt.Errorf("task cannot depend on itself")
		}
		if depTask, ok := s.tasks[depID]; ok {
			depTask.Dependents = addStringToSliceIfMissing(depTask.Dependents, task.ID)
			depTask.UpdatedAt = now
			s.tasks[depID] = depTask
		} else {
			return fmt.Errorf("new dependency task with ID '%s' not found", depID)
		}
	}

	return nil
}

// convertType attempts to convert an interface value to a target reflect.Type.
// This is a simplified converter for specific types used in Task.
func convertType(value interface{}, targetType reflect.Type) (reflect.Value, error) {
	valueType := reflect.TypeOf(value)
	if valueType == targetType {
		return reflect.ValueOf(value), nil
	}

	// Handle string to custom types like TaskStatus and TaskPriority
	if valueStr, ok := value.(string); ok {
		switch targetType {
		case reflect.TypeOf(models.TaskStatus("")):
			return reflect.ValueOf(models.TaskStatus(valueStr)), nil
		case reflect.TypeOf(models.TaskPriority("")):
			return reflect.ValueOf(models.TaskPriority(valueStr)), nil
		}
	}

	return reflect.Value{}, fmt.Errorf("unsupported type conversion from %v to %v", valueType, targetType)
}

// DeleteTask removes a task from the store by its unique identifier.
// It prevents deletion if other tasks depend on it or if the task itself has subtasks.
// If successful, it also removes itself from the Dependents list of tasks it depended on,
// and from the SubtaskIDs list of its parent task.
func (s *FileTaskStore) DeleteTask(id string) error {
	if err := s.flk.Lock(); err != nil {
		return fmt.Errorf("could not lock file for delete: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return fmt.Errorf("failed to reload tasks before delete: %w", err)
	}

	taskToDelete, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task with ID '%s' not found", id)
	}

	// --- Relationship Management ---
	now := time.Now().UTC()

	// 1. Update parent task if this was a subtask
	if taskToDelete.ParentID != nil && *taskToDelete.ParentID != "" {
		if parentTask, ok := s.tasks[*taskToDelete.ParentID]; ok {
			parentTask.SubtaskIDs = removeStringFromSlice(parentTask.SubtaskIDs, id)
			parentTask.UpdatedAt = now
			s.tasks[*taskToDelete.ParentID] = parentTask
		}
	}

	// 2. Remove this task from the Dependents list of its dependencies
	for _, depID := range taskToDelete.Dependencies {
		if depTask, ok := s.tasks[depID]; ok {
			depTask.Dependents = removeStringFromSlice(depTask.Dependents, id)
			depTask.UpdatedAt = now
			s.tasks[depID] = depTask
		}
	}

	// 3. Handle dependents of the task being deleted. For now, we disallow deletion of tasks that have dependents.
	// A more advanced implementation could offer to re-link dependents or delete them recursively.
	if len(taskToDelete.Dependents) > 0 {
		return fmt.Errorf("cannot delete task '%s': it is a dependency for other tasks (e.g., %s)", taskToDelete.Title, strings.Join(taskToDelete.Dependents, ", "))
	}

	// 4. Handle subtasks of the task being deleted. For now, disallow deletion of tasks with subtasks.
	if len(taskToDelete.SubtaskIDs) > 0 {
		return fmt.Errorf("cannot delete task '%s' - it has subtasks - please delete or re-assign subtasks first", taskToDelete.Title)
	}

	// Finally, delete the task itself
	delete(s.tasks, id)

	if err := s.saveTasksToFileInternal(); err != nil {
		// Best-effort rollback
		_ = s.loadTasksFromFileInternal()
		return fmt.Errorf("failed to save after deleting task: %w", err)
	}

	return nil
}

// DeleteTasks removes a list of tasks in a single, atomic operation.
// It also cleans up all relationships related to the deleted tasks.
func (s *FileTaskStore) DeleteTasks(ids []string) (int, error) {
	if err := s.flk.Lock(); err != nil {
		return 0, fmt.Errorf("could not lock file for batch delete: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return 0, fmt.Errorf("failed to reload tasks before batch delete: %w", err)
	}

	deleteSet := make(map[string]bool)
	for _, id := range ids {
		deleteSet[id] = true
	}

	deletedCount := 0
	now := time.Now().UTC()

	// Create a new map for tasks to keep, to avoid modifying while iterating.
	keptTasks := make(map[string]models.Task)
	for id, task := range s.tasks {
		if !deleteSet[id] {
			keptTasks[id] = task
		}
	}

	// Now iterate over the tasks we are keeping and clean up their relationships
	// to any tasks that were deleted.
	for id, task := range keptTasks {
		modified := false

		// Clean up parent link if parent was deleted.
		if task.ParentID != nil && deleteSet[*task.ParentID] {
			task.ParentID = nil
			modified = true
		}

		// Clean up dependencies list.
		newDeps := []string{}
		for _, depID := range task.Dependencies {
			if !deleteSet[depID] {
				newDeps = append(newDeps, depID)
			} else {
				modified = true
			}
		}
		if modified {
			task.Dependencies = newDeps
			task.UpdatedAt = now
			keptTasks[id] = task
		}
	}

	deletedCount = len(s.tasks) - len(keptTasks)
	s.tasks = keptTasks // Replace the old map with the cleaned one.

	if err := s.saveTasksToFileInternal(); err != nil {
		// Rollback is complex here; a simple reload is the safest option.
		_ = s.loadTasksFromFileInternal()
		return 0, fmt.Errorf("failed to save after batch deleting tasks: %w", err)
	}

	return deletedCount, nil
}

// DeleteAllTasks removes all tasks from the store.
// This is a destructive operation that wipes the entire task map.
func (s *FileTaskStore) DeleteAllTasks() error {
	if err := s.flk.Lock(); err != nil {
		return fmt.Errorf("failed to acquire write lock for DeleteAllTasks: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	// This is a destructive operation. The command layer should have already confirmed with the user.
	// Here we just perform the action by clearing the in-memory map.
	s.tasks = make(map[string]models.Task)

	// Save the now-empty task list to the file, overwriting the previous state.
	if err := s.saveTasksToFileInternal(); err != nil {
		// If saving fails, the file on disk is not touched, but the in-memory store is now empty.
		// Reloading would be necessary to get back to the previous state.
		// The error signals that the operation was not successful.
		return fmt.Errorf("failed to clear data file by saving empty task list: %w", err)
	}
	return nil
}

// MarkTaskDone marks a task as completed.
// Automatically cascades completion to all subtasks when a parent task is marked done.
func (s *FileTaskStore) MarkTaskDone(id string) (models.Task, error) {
	if err := s.flk.Lock(); err != nil {
		return models.Task{}, fmt.Errorf("failed to acquire write lock for MarkTaskDone: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return models.Task{}, fmt.Errorf("failed to load tasks before marking done: %w", err)
	}

	task, ok := s.tasks[id]
	if !ok {
		return models.Task{}, fmt.Errorf("task with ID %s not found to mark as done", id)
	}

	originalTask := task // For potential revert

	now := time.Now().UTC()
	task.Status = models.StatusDone
	task.CompletedAt = &now
	task.UpdatedAt = now

	if err := models.ValidateStruct(task); err != nil {
		s.tasks[id] = originalTask // Revert
		return models.Task{}, fmt.Errorf("validation failed for task %s after marking done: %w", id, err)
	}

	s.tasks[id] = task

	// Automatically complete all subtasks when parent task is marked done
	for _, subtaskID := range task.SubtaskIDs {
		if subtask, exists := s.tasks[subtaskID]; exists && subtask.Status != models.StatusDone {
			subtask.Status = models.StatusDone
			subtask.CompletedAt = &now
			subtask.UpdatedAt = now
			s.tasks[subtaskID] = subtask
		}
	}

	if err := s.saveTasksToFileInternal(); err != nil {
		s.tasks[id] = originalTask // Revert
		return models.Task{}, fmt.Errorf("failed to save task %s after marking done: %w", id, err)
	}

	return task, nil
}

// ListTasks retrieves a list of tasks.
// It can optionally apply a filter function and a sort function to the tasks.
func (s *FileTaskStore) ListTasks(filterFn func(models.Task) bool, sortFn func([]models.Task) []models.Task) ([]models.Task, error) {
	// Use a read lock if loadTasksFromFileInternal is thread-safe for reads or if tasks are loaded once and kept in memory.
	// Given that loadTasksFromFileInternal re-reads, an exclusive lock is safer here to prevent reading partial writes from other ops.
	if err := s.flk.Lock(); err != nil { // Using exclusive lock for safety during load. Could optimize with RLock if load is safe.
		return nil, fmt.Errorf("failed to acquire lock for ListTasks: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return nil, fmt.Errorf("failed to load tasks for ListTasks: %w", err)
	}

	if len(s.tasks) == 0 {
		return []models.Task{}, nil
	}

	taskList := make([]models.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		taskList = append(taskList, task)
	}

	if filterFn != nil {
		filteredTasks := make([]models.Task, 0)
		for _, task := range taskList {
			if filterFn(task) {
				filteredTasks = append(filteredTasks, task)
			}
		}
		taskList = filteredTasks
	}

	if sortFn != nil {
		sortFn(taskList) // Sorts in-place
	}

	return taskList, nil
}

// Backup creates a backup of the current task data to the specified destination path.
func (s *FileTaskStore) Backup(destinationPath string) error {
	if err := s.flk.Lock(); err != nil { // Shared lock for reading the data file
		return fmt.Errorf("failed to acquire read lock for backup: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	input, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read source file %s for backup: %w", s.filePath, err)
	}

	if err = os.WriteFile(destinationPath, input, 0o644); err != nil {
		return fmt.Errorf("failed to write backup file to %s: %w", destinationPath, err)
	}
	// Note: Backup does not copy the .checksum file. The backed up data file
	// would need its checksum recalculated if restored and checksums are enforced strictly.
	// Or, the backup process could be enhanced to also copy the checksum file if it exists.
	return nil
}

// Restore replaces the current task data with data from the specified source path.
// It also removes any existing checksum file for the main data path, as the new data's
// checksum will be generated on the next save if the source isn't checksummed itself.
func (s *FileTaskStore) Restore(sourcePath string) error {
	if err := s.flk.Lock(); err != nil { // Exclusive lock for writing the data file
		return fmt.Errorf("failed to acquire lock for restore: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source backup file %s: %w", sourcePath, err)
	}

	tempFilePath := s.filePath + ".tmp_restore"
	defer func() { _ = os.Remove(tempFilePath) }()

	if err = os.WriteFile(tempFilePath, sourceData, 0o644); err != nil {
		return fmt.Errorf("failed to write restored data to temporary file %s: %w", tempFilePath, err)
	}

	if err = os.Rename(tempFilePath, s.filePath); err != nil {
		return fmt.Errorf("failed to atomically replace file %s with restored data from %s: %w", s.filePath, sourcePath, err)
	}

	// Remove old checksum file as the restored data might not match it, or source may not have one.
	// A new checksum will be generated on the next successful save of tasks.
	checksumFilePath := s.filePath + checksumSuffix
	_ = os.Remove(checksumFilePath) // Best effort removal

	return s.loadTasksFromFileInternal()
}

// GetTaskWithDescendants fetches a task by its ID along with all its children, grandchildren, etc.
func (s *FileTaskStore) GetTaskWithDescendants(rootID string) ([]models.Task, error) {
	if err := s.flk.Lock(); err != nil {
		return nil, fmt.Errorf("could not acquire read lock for GetTaskWithDescendants: %w", err)
	}
	defer func() { _ = s.flk.Unlock() }()

	if _, exists := s.tasks[rootID]; !exists {
		return nil, fmt.Errorf("task with root ID '%s' not found", rootID)
	}

	allTasksInTree := make(map[string]models.Task)
	var findChildren func(taskID string)

	findChildren = func(taskID string) {
		if task, ok := s.tasks[taskID]; ok {
			// Add the current task to the map
			allTasksInTree[task.ID] = task
			// Recurse for all its children
			for _, subID := range task.SubtaskIDs {
				if _, alreadyProcessed := allTasksInTree[subID]; !alreadyProcessed {
					findChildren(subID)
				}
			}
		}
	}

	findChildren(rootID)

	// Convert map to slice
	result := make([]models.Task, 0, len(allTasksInTree))
	for _, task := range allTasksInTree {
		result = append(result, task)
	}

	return result, nil
}

// Close releases any resources held by the store, such as file locks.
// It attempts to unlock the file lock associated with the FileTaskStore.
// flock.Unlock() is idempotent and can be called even if the lock is not held by this process.
func (s *FileTaskStore) Close() error {
	if s.flk != nil {
		return s.flk.Unlock()
	}
	return nil
}
