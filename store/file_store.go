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
	"github.com/josephgoksu/taskwing.app/models"
	"gopkg.in/yaml.v3"
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
		if err := os.MkdirAll(dir, 0755); err != nil {
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
	defer s.flk.Unlock() // Unlock after initialization sequence

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
			if f, createErr := os.OpenFile(s.filePath, os.O_CREATE|os.O_RDWR, 0644); createErr != nil {
				return fmt.Errorf("failed to create data file %s: %w", s.filePath, createErr)
			} else {
				f.Close()
			}
			// Create an empty checksum file for a new empty data file
			if err := os.WriteFile(checksumFilePath, []byte(calculateChecksum([]byte{})), 0644); err != nil {
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
			return fmt.Errorf("failed to read checksum file %s: %w. Data file might be corrupt or tampered.", checksumFilePath, readErr)
		}
		expectedChecksum := strings.TrimSpace(string(expectedChecksumBytes))
		actualChecksum := calculateChecksum(data)

		if actualChecksum != expectedChecksum {
			return fmt.Errorf("checksum mismatch for %s. Expected %s, got %s. File is corrupt or tampered.", s.filePath, expectedChecksum, actualChecksum)
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
		_ = os.WriteFile(checksumFilePath, []byte(currentChecksum), 0644) // best effort
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

	defer os.Remove(tempFilePath)
	defer os.Remove(tempChecksumFilePath)

	if err := os.WriteFile(tempFilePath, marshaledData, 0644); err != nil {
		return fmt.Errorf("failed to write to temporary data file %s: %w", tempFilePath, err)
	}

	// Data file written to temp, now calculate its checksum
	actualChecksum := calculateChecksum(marshaledData)
	if err := os.WriteFile(tempChecksumFilePath, []byte(actualChecksum), 0644); err != nil {
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
		return fmt.Errorf("CRITICAL: data file %s updated, but failed to update checksum file %s from %s: %w. Store may be inconsistent.", s.filePath, checksumFilePath, tempChecksumFilePath, err)
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
// It generates an ID, sets timestamps, default status/priority, and manages dependency and subtask linkage.
func (s *FileTaskStore) CreateTask(task models.Task) (models.Task, error) {
	if err := s.flk.Lock(); err != nil {
		return models.Task{}, fmt.Errorf("failed to acquire write lock for CreateTask: %w", err)
	}
	defer s.flk.Unlock()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return models.Task{}, fmt.Errorf("failed to load tasks before creating: %w", err)
	}

	if task.ID == "" {
		task.ID = generateID()
	}

	if _, exists := s.tasks[task.ID]; exists {
		return models.Task{}, fmt.Errorf("task with ID %s already exists", task.ID)
	}

	now := time.Now().UTC()
	task.CreatedAt = now
	task.UpdatedAt = now

	if task.Status == "" {
		task.Status = models.StatusPending
	}
	if task.Priority == "" {
		task.Priority = models.PriorityMedium
	}
	task.CompletedAt = nil
	if task.Dependents == nil { // Ensure Dependents is initialized
		task.Dependents = []string{}
	}
	if task.SubtaskIDs == nil { // Ensure SubtaskIDs is initialized
		task.SubtaskIDs = []string{}
	}

	// Validate and link dependencies
	for _, depID := range task.Dependencies {
		if depID == task.ID {
			return models.Task{}, fmt.Errorf("task %s cannot depend on itself", task.ID)
		}
		depTask, exists := s.tasks[depID]
		if !exists {
			return models.Task{}, fmt.Errorf("dependency task with ID %s not found", depID)
		}
		depTask.Dependents = addStringToSliceIfMissing(depTask.Dependents, task.ID)
		s.tasks[depID] = depTask // Update the dependency task in the map
	}

	// Handle ParentID linkage
	if task.ParentID != nil && *task.ParentID != "" {
		parentID := *task.ParentID
		if parentID == task.ID {
			return models.Task{}, fmt.Errorf("task %s cannot be its own parent", task.ID)
		}
		parentTask, exists := s.tasks[parentID]
		if !exists {
			return models.Task{}, fmt.Errorf("parent task with ID %s not found", parentID)
		}
		parentTask.SubtaskIDs = addStringToSliceIfMissing(parentTask.SubtaskIDs, task.ID)
		parentTask.UpdatedAt = now // Parent task is also updated
		s.tasks[parentID] = parentTask
	}

	if err := models.ValidateStruct(task); err != nil {
		// Attempt to rollback dependency and parent changes - best effort
		for _, depID := range task.Dependencies {
			if depTask, exists := s.tasks[depID]; exists {
				depTask.Dependents = removeStringFromSlice(depTask.Dependents, task.ID)
				s.tasks[depID] = depTask
			}
		}
		if task.ParentID != nil && *task.ParentID != "" {
			if parentTask, exists := s.tasks[*task.ParentID]; exists {
				parentTask.SubtaskIDs = removeStringFromSlice(parentTask.SubtaskIDs, task.ID)
				s.tasks[*task.ParentID] = parentTask
			}
		}
		return models.Task{}, fmt.Errorf("validation failed for new task: %w", err)
	}

	s.tasks[task.ID] = task
	if err := s.saveTasksToFileInternal(); err != nil {
		// Attempt to revert in-memory changes if save fails
		delete(s.tasks, task.ID)
		// Also revert dependency updates
		for _, depID := range task.Dependencies {
			if depTask, exists := s.tasks[depID]; exists {
				depTask.Dependents = removeStringFromSlice(depTask.Dependents, task.ID)
				s.tasks[depID] = depTask
			}
		}
		// Also revert parent task's SubtaskIDs update
		if task.ParentID != nil && *task.ParentID != "" {
			if parentTask, exists := s.tasks[*task.ParentID]; exists {
				parentTask.SubtaskIDs = removeStringFromSlice(parentTask.SubtaskIDs, task.ID)
				// No need to update parent's UpdatedAt here for revert, as original parent state will be restored
				s.tasks[*task.ParentID] = parentTask
			}
		}
		return models.Task{}, fmt.Errorf("failed to save new task (with dependencies/parent): %w", err)
	}

	return task, nil
}

// GetTask retrieves a task by its unique identifier.
// This function assumes that a read lock (s.flk.RLock()) is already held by the caller if needed for safety with other ops.
// However, for a single Get, an exclusive lock for load might be simpler if tasks aren't kept hot.
// For simplicity and consistency with other ops, using exclusive lock for load.
func (s *FileTaskStore) GetTask(id string) (models.Task, error) {
	if err := s.flk.Lock(); err != nil { // Using exclusive lock to ensure fresh load and safety
		return models.Task{}, fmt.Errorf("failed to acquire lock for GetTask: %w", err)
	}
	defer s.flk.Unlock()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return models.Task{}, fmt.Errorf("failed to load tasks for GetTask: %w", err)
	}

	task, ok := s.tasks[id]
	if !ok {
		return models.Task{}, fmt.Errorf("task with ID %s not found", id)
	}
	return task, nil
}

// UpdateTask modifies an existing task in the store identified by its ID, applying the given updates.
// This includes managing dependency links and parent-child relationships if relevant fields are updated.
func (s *FileTaskStore) UpdateTask(id string, updates map[string]interface{}) (models.Task, error) {
	if err := s.flk.Lock(); err != nil {
		return models.Task{}, fmt.Errorf("failed to acquire write lock for UpdateTask: %w", err)
	}
	defer s.flk.Unlock()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return models.Task{}, fmt.Errorf("failed to load tasks before updating: %w", err)
	}

	task, ok := s.tasks[id]
	if !ok {
		return models.Task{}, fmt.Errorf("task with ID %s not found for update", id)
	}

	now := time.Now().UTC()
	originalTask := task
	originalDependencies := slices.Clone(task.Dependencies)
	originalParentID := task.ParentID

	var originalOldParentTaskState models.Task
	var originalNewParentTaskState models.Task
	var oldParentExists, newParentExists bool
	var parentIDFieldProvided bool // Declare here for wider scope

	// Handle ParentID updates first if present
	var newParentIDRaw interface{}
	if newParentIDRaw, parentIDFieldProvided = updates["parentId"]; parentIDFieldProvided {
		var newParentIDPtr *string
		if newParentIDRaw == nil {
			newParentIDPtr = nil
		} else if newParentIDStr, isStr := newParentIDRaw.(string); isStr {
			if newParentIDStr == "" { // Treat empty string as unsetting the parent
				newParentIDPtr = nil
			} else {
				if newParentIDStr == id {
					return models.Task{}, fmt.Errorf("task %s cannot be its own parent", id)
				}
				if _, parentExists := s.tasks[newParentIDStr]; !parentExists {
					return models.Task{}, fmt.Errorf("new parent task with ID %s not found", newParentIDStr)
				}
				newParentIDPtr = &newParentIDStr
			}
		} else {
			return models.Task{}, fmt.Errorf("invalid type for 'parentId' field: expected string or nil, got %T", newParentIDRaw)
		}

		// If old parent existed, remove task from its SubtaskIDs
		if originalParentID != nil && *originalParentID != "" {
			if oldParentTask, exists := s.tasks[*originalParentID]; exists {
				originalOldParentTaskState = oldParentTask // Save state for rollback
				oldParentExists = true
				oldParentTask.SubtaskIDs = removeStringFromSlice(oldParentTask.SubtaskIDs, id)
				oldParentTask.UpdatedAt = now
				s.tasks[*originalParentID] = oldParentTask
			}
		}

		// If new parent exists, add task to its SubtaskIDs
		if newParentIDPtr != nil && *newParentIDPtr != "" {
			newParentIDVal := *newParentIDPtr
			if newParentTask, exists := s.tasks[newParentIDVal]; exists {
				originalNewParentTaskState = newParentTask // Save state for rollback
				newParentExists = true
				newParentTask.SubtaskIDs = addStringToSliceIfMissing(newParentTask.SubtaskIDs, id)
				newParentTask.UpdatedAt = now
				s.tasks[newParentIDVal] = newParentTask
			}
		}
		task.ParentID = newParentIDPtr
		delete(updates, "parentId") // Processed
	}

	// Handle dependency updates if present (copied from original logic, ensure it's still relevant)
	if newDepsRaw, depsFieldProvided := updates["dependencies"]; depsFieldProvided {
		var newDepIDs []string
		if newDepsStrSlice, isStrSlice := newDepsRaw.([]string); isStrSlice {
			newDepIDs = newDepsStrSlice
		} else if newDepsIfaceSlice, isIfaceSlice := newDepsRaw.([]interface{}); isIfaceSlice {
			newDepIDs = make([]string, len(newDepsIfaceSlice))
			for i, v := range newDepsIfaceSlice {
				if strV, isStr := v.(string); isStr {
					newDepIDs[i] = strV
				} else {
					return models.Task{}, fmt.Errorf("invalid type for dependency ID in list: expected string, got %T for item %v", v, v)
				}
			}
		} else if newDepsRaw == nil { // explicitly setting dependencies to empty list
			newDepIDs = []string{}
		} else {
			return models.Task{}, fmt.Errorf("invalid type for 'dependencies' field: expected []string or []interface{}, got %T", newDepsRaw)
		}

		// Validate new dependencies and prevent self-dependency
		for _, depID := range newDepIDs {
			if depID == id {
				return models.Task{}, fmt.Errorf("task %s cannot depend on itself", id)
			}
			if _, exists := s.tasks[depID]; !exists {
				return models.Task{}, fmt.Errorf("dependency task with ID %s not found", depID)
			}
		}

		// Determine added and removed dependencies
		oldDepSet := make(map[string]struct{})
		for _, depID := range originalDependencies {
			oldDepSet[depID] = struct{}{}
		}
		newDepSet := make(map[string]struct{})
		for _, depID := range newDepIDs {
			newDepSet[depID] = struct{}{}
		}

		// Update Dependents for removed dependencies
		for _, oldDepID := range originalDependencies {
			if _, stillExists := newDepSet[oldDepID]; !stillExists {
				if depTask, exists := s.tasks[oldDepID]; exists {
					depTask.Dependents = removeStringFromSlice(depTask.Dependents, id)
					s.tasks[oldDepID] = depTask
				}
			}
		}

		// Update Dependents for added dependencies
		for _, newDepID := range newDepIDs {
			if _, wasAlreadyThere := oldDepSet[newDepID]; !wasAlreadyThere {
				if depTask, exists := s.tasks[newDepID]; exists {
					depTask.Dependents = addStringToSliceIfMissing(depTask.Dependents, id)
					s.tasks[newDepID] = depTask
				}
			}
		}
		task.Dependencies = newDepIDs
		delete(updates, "dependencies")
	}

	taskValue := reflect.ValueOf(&task).Elem()
	for key, value := range updates { // Process remaining updates
		// Disallow direct updates to SubtaskIDs as it's managed internally
		if key == "subtaskIds" || key == "SubtaskIDs" {
			return models.Task{}, fmt.Errorf("direct update to 'SubtaskIDs' field is not allowed; it is managed via ParentID changes")
		}

		fieldName := strings.ToUpper(key[:1]) + key[1:]
		field := taskValue.FieldByName(fieldName)

		if !field.IsValid() {
			return models.Task{}, fmt.Errorf("invalid field for update: %s", fieldName)
		}
		if !field.CanSet() {
			return models.Task{}, fmt.Errorf("cannot set field: %s", fieldName)
		}

		updateValue := reflect.ValueOf(value)

		if field.Type() == reflect.TypeOf((*time.Time)(nil)) {
			if strVal, ok := value.(string); ok {
				if strVal == "" {
					field.Set(reflect.Zero(field.Type()))
					continue
				}
			}
		}

		if updateValue.IsValid() && updateValue.Type().AssignableTo(field.Type()) {
			field.Set(updateValue)
		} else if updateValue.IsValid() && field.Type().Kind() == reflect.String && updateValue.Type().ConvertibleTo(field.Type()) {
			field.Set(updateValue.Convert(field.Type()))
		} else if fieldName == "Status" && updateValue.Type().ConvertibleTo(reflect.TypeOf(models.StatusPending)) {
			statusVal, ok := value.(string)
			if !ok {
				return models.Task{}, fmt.Errorf("invalid type for Status: expected string, got %T", value)
			}
			task.Status = models.TaskStatus(statusVal)
		} else if fieldName == "Priority" && updateValue.Type().ConvertibleTo(reflect.TypeOf(models.PriorityLow)) {
			priorityVal, ok := value.(string)
			if !ok {
				return models.Task{}, fmt.Errorf("invalid type for Priority: expected string, got %T", value)
			}
			task.Priority = models.TaskPriority(priorityVal)
		} else if fieldName == "Dependencies" || fieldName == "Tags" {
			strSlice, ok := value.([]string)
			if !ok {
				if ifaceSlice, isIfaceSlice := value.([]interface{}); isIfaceSlice {
					strSlice = make([]string, len(ifaceSlice))
					for i, v := range ifaceSlice {
						strV, isStr := v.(string)
						if !isStr {
							return models.Task{}, fmt.Errorf("invalid type for element in %s: expected string, got %T", fieldName, v)
						}
						strSlice[i] = strV
					}
					ok = true
				}
			}
			if ok {
				field.Set(reflect.ValueOf(strSlice))
			} else {
				return models.Task{}, fmt.Errorf("invalid type for %s: expected []string, got %T", fieldName, value)
			}
		} else if updateValue.IsValid() {
			return models.Task{}, fmt.Errorf("type mismatch for field %s: expected %s, got %s", fieldName, field.Type(), updateValue.Type())
		} else if value == nil && field.Type().Kind() == reflect.Ptr {
			field.Set(reflect.Zero(field.Type()))
		} else {
			return models.Task{}, fmt.Errorf("invalid value for field %s", fieldName)
		}
	}

	task.UpdatedAt = now

	if err := models.ValidateStruct(task); err != nil {
		s.tasks[id] = originalTask
		if parentIDFieldProvided { // Check if parentID was part of the update attempt
			if oldParentExists {
				s.tasks[*originalParentID] = originalOldParentTaskState
			}
			if newParentExists && task.ParentID != nil && *task.ParentID != "" { // task.ParentID is the NEW parent ID here
				_, found := s.tasks[*task.ParentID]
				if found && originalNewParentTaskState.ID == *task.ParentID {
					s.tasks[*task.ParentID] = originalNewParentTaskState
				}
				// If not found or ID mismatch, the new parent might have been deleted or changed, rollback is complex.
				// The primary goal here is to revert the main task and attempt to revert immediate parent states.
			}
		}
		return models.Task{}, fmt.Errorf("validation failed for updated task %s: %w", id, err)
	}

	s.tasks[id] = task
	if err := s.saveTasksToFileInternal(); err != nil {
		s.tasks[id] = originalTask
		if parentIDFieldProvided { // Check if parentID was part of the update attempt
			if oldParentExists {
				s.tasks[*originalParentID] = originalOldParentTaskState
			}
			if newParentExists && task.ParentID != nil && *task.ParentID != "" { // task.ParentID is the NEW parent ID here
				_, found := s.tasks[*task.ParentID]
				if found && originalNewParentTaskState.ID == *task.ParentID {
					s.tasks[*task.ParentID] = originalNewParentTaskState
				}
				// If not found or ID mismatch, the new parent might have been deleted or changed, rollback is complex.
				// The primary goal here is to revert the main task and attempt to revert immediate parent states.
			}
		}
		return models.Task{}, fmt.Errorf("failed to save updated task %s: %w", id, err)
	}

	return task, nil
}

// DeleteTask removes a task from the store by its unique identifier.
// It prevents deletion if other tasks depend on it or if the task itself has subtasks.
// If successful, it also removes itself from the Dependents list of tasks it depended on,
// and from the SubtaskIDs list of its parent task.
func (s *FileTaskStore) DeleteTask(id string) error {
	if err := s.flk.Lock(); err != nil {
		return fmt.Errorf("failed to acquire write lock for DeleteTask: %w", err)
	}
	defer s.flk.Unlock()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return fmt.Errorf("failed to load tasks before deleting: %w", err)
	}

	taskToDelete, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task with ID %s not found for deletion", id)
	}

	// Prevent deletion if other tasks depend on this task
	if len(taskToDelete.Dependents) > 0 {
		return fmt.Errorf("task %s cannot be deleted because other tasks depend on it: %v", id, taskToDelete.Dependents)
	}

	// Prevent deletion if this task has subtasks
	if len(taskToDelete.SubtaskIDs) > 0 {
		return fmt.Errorf("task %s cannot be deleted because it has subtasks: %v. Please delete or reassign subtasks first.", id, taskToDelete.SubtaskIDs)
	}

	now := time.Now().UTC()
	originalTaskForRevert := taskToDelete
	originalDependenciesForRevert := make(map[string]models.Task)
	var originalParentTaskState models.Task
	parentModified := false

	// Remove this task from the Dependents list of tasks it depended on
	for _, depID := range taskToDelete.Dependencies {
		if depTask, exists := s.tasks[depID]; exists {
			originalDependenciesForRevert[depID] = depTask
			depTask.Dependents = removeStringFromSlice(depTask.Dependents, id)
			depTask.UpdatedAt = now // Also update timestamp of modified dependency
			s.tasks[depID] = depTask
		}
	}

	// If this task was a subtask, remove it from its parent's SubtaskIDs list
	if taskToDelete.ParentID != nil && *taskToDelete.ParentID != "" {
		parentID := *taskToDelete.ParentID
		if parentTask, exists := s.tasks[parentID]; exists {
			originalParentTaskState = parentTask // Save for potential rollback
			parentModified = true
			parentTask.SubtaskIDs = removeStringFromSlice(parentTask.SubtaskIDs, id)
			parentTask.UpdatedAt = now
			s.tasks[parentID] = parentTask
		}
	}

	delete(s.tasks, id)

	if err := s.saveTasksToFileInternal(); err != nil {
		// Revert deletion and all modifications
		s.tasks[id] = originalTaskForRevert
		for depID, originalDepTask := range originalDependenciesForRevert {
			s.tasks[depID] = originalDepTask
		}
		if parentModified && taskToDelete.ParentID != nil && *taskToDelete.ParentID != "" {
			// Ensure originalParentTaskState was actually captured and ID matches
			if originalParentTaskState.ID == *taskToDelete.ParentID {
				s.tasks[*taskToDelete.ParentID] = originalParentTaskState
			}
		}
		_ = s.saveTasksToFileInternal() // Best effort to save reverted state
		return fmt.Errorf("failed to save after deleting task %s and updating related tasks: %w", id, err)
	}
	return nil
}

// MarkTaskDone marks a task as completed.
// Note: This does not currently automatically update statuses of dependent tasks.
func (s *FileTaskStore) MarkTaskDone(id string) (models.Task, error) {
	if err := s.flk.Lock(); err != nil {
		return models.Task{}, fmt.Errorf("failed to acquire write lock for MarkTaskDone: %w", err)
	}
	defer s.flk.Unlock()

	if err := s.loadTasksFromFileInternal(); err != nil {
		return models.Task{}, fmt.Errorf("failed to load tasks before marking done: %w", err)
	}

	task, ok := s.tasks[id]
	if !ok {
		return models.Task{}, fmt.Errorf("task with ID %s not found to mark as done", id)
	}

	originalTask := task // For potential revert

	now := time.Now().UTC()
	task.Status = models.StatusCompleted
	task.CompletedAt = &now
	task.UpdatedAt = now

	if err := models.ValidateStruct(task); err != nil {
		s.tasks[id] = originalTask // Revert
		return models.Task{}, fmt.Errorf("validation failed for task %s after marking done: %w", id, err)
	}

	s.tasks[id] = task
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
	defer s.flk.Unlock()

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
	if err := s.flk.RLock(); err != nil { // Shared lock for reading the data file
		return fmt.Errorf("failed to acquire read lock for backup: %w", err)
	}
	defer s.flk.Unlock()

	input, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read source file %s for backup: %w", s.filePath, err)
	}

	if err = os.WriteFile(destinationPath, input, 0644); err != nil {
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
	defer s.flk.Unlock()

	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source backup file %s: %w", sourcePath, err)
	}

	tempFilePath := s.filePath + ".tmp_restore"
	defer os.Remove(tempFilePath)

	if err = os.WriteFile(tempFilePath, sourceData, 0644); err != nil {
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

// Close releases any resources held by the store, such as file locks.
// It attempts to unlock the file lock associated with the FileTaskStore.
// flock.Unlock() is idempotent and can be called even if the lock is not held by this process.
func (s *FileTaskStore) Close() error {
	if s.flk != nil {
		return s.flk.Unlock()
	}
	return nil
}
