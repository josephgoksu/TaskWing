package store

import "github.com/josephgoksu/TaskWing/models"

// TaskStore defines the interface for task persistence.
// It outlines the contract for managing tasks, including CRUD operations,
// initialization, backup, restore, and resource cleanup.
type TaskStore interface {
	// Initialize configures the store with necessary parameters, such as
	// file path, data format, and any other backend-specific settings.
	// It should be called before any other store operations.
	Initialize(config map[string]string) error

	// CreateTask adds a new task to the store.
	// It returns the created task, potentially with store-generated fields
	// (e.g., updated timestamps) or an error if the operation fails.
	CreateTask(task models.Task) (models.Task, error)

	// GetTask retrieves a task by its unique identifier.
	// It returns the found task or an error if the task does not exist
	// or if the retrieval fails.
	GetTask(id string) (models.Task, error)

	// UpdateTask modifies an existing task in the store identified by its ID, applying the given updates.
	// The 'updates' map contains field names to their new values.
	// It returns the updated task or an error if the task is not found
	// or the update operation fails.
	UpdateTask(id string, updates map[string]interface{}) (models.Task, error)

	// DeleteTask removes a task from the store by its unique identifier.
	// It returns an error if the task is not found or the deletion fails.
	DeleteTask(id string) error

	// DeleteTasks removes a list of tasks from the store by their unique identifiers.
	// This is intended for batch operations, like recursive deletes.
	// It should be more performant than calling DeleteTask for each ID.
	// It returns the number of tasks successfully deleted, or an error.
	DeleteTasks(ids []string) (int, error)

	// DeleteAllTasks removes all tasks from the store.
	// This is a destructive operation.
	// It returns an error if the operation fails.
	DeleteAllTasks() error

	// MarkTaskDone marks a task as completed.
	// It sets the task's status to completed and updates relevant timestamps.
	// It returns the updated task or an error if the task is not found or the operation fails.
	MarkTaskDone(id string) (models.Task, error)

	// ListTasks retrieves a list of tasks.
	// It can optionally apply a filter function and a sort function to the tasks.
	// If filterFn is nil, all tasks are returned (subject to sorting).
	// If sortFn is nil, the tasks are returned in their natural order (e.g., as read from the store).
	// It returns a slice of tasks or an error if the operation fails.
	ListTasks(filterFn func(models.Task) bool, sortFn func([]models.Task) []models.Task) ([]models.Task, error)

	// GetTaskWithDescendants retrieves a root task and all of its descendants (subtasks, sub-subtasks, etc.).
	// The returned slice includes the root task itself.
	GetTaskWithDescendants(rootID string) ([]models.Task, error)

	// Backup creates a backup of the current task data to the specified destination path.
	// The format and method of backup are implementation-specific.
	// It returns an error if the backup operation fails.
	Backup(destinationPath string) error

	// Restore replaces the current task data with data from the specified source path.
	// This operation may be destructive to current data.
	// It returns an error if the restoration fails.
	Restore(sourcePath string) error

	// Close releases any resources held by the store, such as file locks or
	// database connections. It should be called when the store is no longer needed.
	Close() error
}
