# TaskWing

TaskWing is an AI-assisted CLI task manager for developers. This proof-of-concept (POC) helps you manage tasks, track dependencies, and organize your workflow directly from the terminal.

## Installation

You can build the CLI from source.

**Prerequisites:**

- Go (version 1.21+ recommended)

**Build steps:**

```bash
# Build the binary
go build -o taskwing main.go

# Move it to a directory in your PATH (optional)
mv ./taskwing /usr/local/bin/
```

## Getting Started

Navigate to your project's root directory and initialize TaskWing:

```bash
taskwing init
```

This command creates a `.taskwing` directory in your project to store all related data and configuration.

## Command Reference

### `taskwing add`

Add a new task. You will be prompted for a title, description, priority, tags, and dependencies.

```bash
taskwing add
```

### `taskwing list`

List tasks with filters and sorting.

- **Filter by:** `--status`, `--priority`, `--title-contains`, `--description-contains`, `--tags`, `--tags-any`, `--search`.
- **Sort by:** `--sort-by <field>` (e.g., `id`, `priority`, `createdAt`)
- **Sort order:** `--sort-order <asc|desc>`

```bash
# Example: List all high-priority, pending tasks, sorted by creation date.
taskwing list --status pending --priority high --sort-by createdAt
```

### `taskwing update [task_id]`

Update a task. If no `task_id` is provided, an interactive selector will be shown.

```bash
taskwing update <your_task_id>
```

### `taskwing delete [task_id]`

Delete a task. If no `task_id` is provided, an interactive selector will be shown. A task cannot be deleted if other tasks depend on it.

```bash
taskwing delete <your_task_id>
```

### `taskwing done [task_id]`

Mark a task as completed. If no `task_id` is provided, an interactive selector will be shown.

```bash
taskwing done <your_task_id>
```

### `taskwing deps [task_id]`

Display dependency relationships for a task.

```bash
taskwing deps <your_task_id>
```

### `taskwing generate`

**[Placeholder]** This command is reserved for future AI-assisted task generation.

## Configuration

TaskWing is configured via a `.taskwing.yaml` file (or `.json`/`.toml`). The configuration is loaded in the following order:

1.  Project-specific: `<project.rootDir>/.taskwing.yaml` (e.g. `./.taskwing/.taskwing.yaml`)
2.  Current directory: `./.taskwing.yaml`
3.  Home directory: `$HOME/.taskwing.yaml`

Environment variables prefixed with `TASKWING_` can also be used (e.g., `TASKWING_PROJECT_ROOTDIR`).

**Key Options:**

- `project.rootDir`: Base directory for all TaskWing files. (Default: `.taskwing`)
- `project.tasksDir`: Directory for task data files. (Default: `tasks`)
- `data.file`: Name of the task data file. (Default: `tasks.json`)
- `data.format`: Data file format (`json`, `yaml`, `toml`). (Default: `json`)
- `project.outputLogPath`: Path for log files. (Default: `.taskwing/logs/taskwing.log`)

## Data Storage

- Tasks are stored locally in the file specified by `data.file` (e.g., `.taskwing/tasks/tasks.json`).
- A `.checksum` file is maintained alongside the data file to ensure data integrity. The application will not load data if a checksum mismatch is detected.

## Contributing

Contributions are welcome. Please submit an issue or pull request.

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.
