# TaskWing.app

TaskWing.app is an AI-assisted task manager built for modern development workflows. Designed for engineers, indie hackers, and teams, it aims to automate project planning, task breakdown, prioritization, and workflow clarity—so you can focus on building, not managing.

This README describes the current Proof of Concept (POC) CLI capabilities.

## Core Features (POC)

- **Project Initialization:** Set up a new TaskWing project environment.
- **Task Management (CLI):**
  - Create, list, update, and delete tasks.
  - Tasks include: ID, Title, Description, Status, Priority, Tags, Dependencies, and Dependents.
- **Dependency Tracking:** Define and view relationships between tasks.
- **Local Data Storage:** Tasks are stored locally in a file (JSON, YAML, or TOML) with checksum verification for data integrity.
- **Interactive Prompts:** User-friendly prompts for adding and updating tasks.
- **Flexible Filtering & Sorting:** Powerful options for viewing tasks.
- **Configuration:** Project-specific configuration via a local file.

## Prerequisites

- Go (if building from source, version 1.21+ recommended).
- A compiled `taskwing` binary (if not building from source).

## Installation

Currently, you can build the CLI from source:

```bash
go build -o taskwing main.go
```

Then, you can run it as `./taskwing` or move it to a directory in your PATH (e.g., `/usr/local/bin/taskwing`).

## Configuration

TaskWing uses a project-specific configuration. When you initialize a project, it expects settings to be potentially found in a configuration file.

1.  **Project Directory:** By default, all TaskWing specific files for a project are stored in a `.taskwing` directory created in your project's root.

    - **Default Root:** `./.taskwing/`

2.  **Configuration File:**

    - TaskWing looks for a configuration file named `.taskwing.yaml` (or `.taskwing.json`, `.taskwing.toml`) primarily inside the `project.rootDir` (e.g., `./.taskwing/.taskwing.yaml`).
    - If not found there, it falls back to `$HOME/.taskwing.yaml` and then `./.taskwing.yaml` (in the current directory).
    - Environment variables prefixed with `TASKWING_` can also be used (e.g., `TASKWING_PROJECT_ROOTDIR`).

3.  **Key Configuration Options (`AppConfig` in `cmd/config.go`):**

    - `project.rootDir`: (Default: `.taskwing`) The base directory for all TaskWing project files.
    - `project.tasksDir`: (Default: `tasks`) Directory for task data files, relative to `project.rootDir`.
    - `project.templatesDir`: (Default: `templates`) Directory for templates, relative to `project.rootDir`.
    - `project.outputLogPath`: (Default: `.taskwing/logs/taskwing.log`) Path for log files.
    - `data.file`: (Default: `tasks.json`) The name of the task data file.
    - `data.format`: (Default: `json`) The format of the task data file (`json`, `yaml`, `toml`).
    - `api.*`: Placeholder for future AI model API key configurations.
    - `model.*`: Placeholder for future AI model selection and parameters.

    Example structure within your project:

    ```
    your-project/
    ├── .taskwing/
    │   ├── .taskwing.yaml  # Optional project-specific config
    │   ├── tasks/
    │   │   └── tasks.json        # Default task data file
    │   │   └── tasks.json.checksum # Checksum for tasks.json
    │   └── logs/
    │       └── taskwing.log    # Log file
    └── ... your other project files
    ```

## Getting Started

1.  **Initialize a new TaskWing project:**
    Navigate to your project's root directory in the terminal and run:
    ```bash
    taskwing init
    ```
    This will:
    - Create the `.taskwing` directory (or your configured `project.rootDir`).
    - Create the tasks directory within it (e.g., `.taskwing/tasks`).
    - Ensure the task data file (e.g., `.taskwing/tasks/tasks.json`) can be initialized.

## Core Commands

### `taskwing add`

Add a new task. You will be prompted for the following information:

- **Title:** (Required, min 3 chars) The main title of the task.
- **Description:** (Optional) A more detailed description.
- **Priority:** (Required) Select from Low, Medium, High, Urgent.
- **Tags:** (Optional) Comma-separated tags (e.g., `backend,api,refactor`).
- **Dependencies:** (Optional) Comma-separated IDs of tasks this task depends on.

Example:

```bash
taskwing add
```

### `taskwing list`

List tasks with various filtering and sorting options.

- **Output Columns:** ID, Title, Status, Priority, Tags, Dependencies, Dependents.

- **Filtering Flags:**

  - `--status <statuses>`: Filter by status (comma-separated, e.g., `pending,in-progress`).
  - `--priority <priorities>`: Filter by priority (comma-separated, e.g., `high,urgent`).
  - `--title-contains <text>`: Filter by text in title (case-insensitive).
  - `--description-contains <text>`: Filter by text in description (case-insensitive).
  - `--tags <tags>`: Filter by tags (ALL tags must match, comma-separated).
  - `--tags-any <tags>`: Filter by tags (ANY tag must match, comma-separated).
  - `--search <query>`: Generic search across ID, title, description (case-insensitive).

- **Sorting Flags:**
  - `--sort-by <field>`: Sort tasks by field (id, title, status, priority, createdAt, updatedAt). Default: `createdAt`.
  - `--sort-order <asc|desc>`: Sort order. Default: `asc`.

Example:

```bash
taskwing list --status pending --priority high --sort-by createdAt
```

### `taskwing update [task_id]`

Update an existing task.

- If `[task_id]` is provided, it attempts to update that task directly.
- If no `[task_id]` is provided, an interactive list is shown to select a task.

You can update: Title, Description, Priority, Status, Tags (use 'none' to clear), and Dependencies (use 'none' to clear).

Example (interactive):

```bash
taskwing update
```

Example (direct):

```bash
taskwing update <your_task_id_here>
```

### `taskwing delete [task_id]`

Delete a task.

- If `[task_id]` is provided, it attempts to delete that task directly after confirmation.
- If no `[task_id]` is provided, an interactive list is shown to select a task, followed by a confirmation.
- **Note:** A task cannot be deleted if other tasks depend on it.

Example:

```bash
taskwing delete <your_task_id_here>
```

### `taskwing done [task_id]`

Mark a task as completed.

- If `[task_id]` is provided, it attempts to mark that task as done.
- If no `[task_id]` is provided, an interactive list of non-completed tasks is shown.

Example:

```bash
taskwing done <your_task_id_here>
```

### `taskwing deps [task_id]`

Display dependency information for a specific task. Shows which tasks the given task depends on, and which tasks depend on it.

Example:

```bash
taskwing deps <your_task_id_here>
```

### `taskwing generate`

(Placeholder for Future AI Feature) This command is intended for AI-assisted task generation from natural language input. Currently, it only prints a message.

## Data Storage

- Tasks are stored locally in a file defined by your configuration (default: `.taskwing/tasks/tasks.json`).
- The data format can be `json`, `yaml`, or `toml`.
- A checksum file (e.g., `tasks.json.checksum`) is maintained alongside the data file to ensure data integrity. The store will refuse to load data if a checksum mismatch is detected.

## Building from Source

1.  Ensure you have Go installed (version 1.21 or newer is recommended).
2.  Clone the repository (if applicable).
3.  Navigate to the project directory.
4.  Run: `go build -o taskwing main.go`
5.  The `taskwing` executable will be created in the current directory.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests. (Further details to be added).

## License

This project is licensed under the MIT License - see the LICENSE file for details. (Assuming MIT, please update if different).
