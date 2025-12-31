package eval

// DefaultTasksTemplate is the default template for tasks.yaml.
const DefaultTasksTemplate = `version: 1
project: ""

hard_fail_rules:
  - id: openapi_source_of_truth
    task_ids: [T1, T2, T5]
    require_all:
      - "specs/openapi.yaml"
      - "generate-api|generate:api|codegen"
  - id: frontend_types_generated
    task_ids: [T1, T4]
    require_all:
      - "@/types/api"
  - id: no_env_in_prod
    task_ids: [T3]
    forbid:
      - "\\.env"
    allow_if:
      - "ssm|parameter store|secrets manager"
      - "never.{0,20}prod|prod.{0,20}never|avoid.{0,20}prod"

tasks:
  - id: T1
    title: Add sourcePlatform to Bookmark entity and show it in web UI
    prompt: |
      Add a new field "sourcePlatform" to the Bookmark entity and display it in the web UI. Keep behavior consistent.
    pass_fail:
      pass: |
        - Updates specs/openapi.yaml
        - Runs backend and frontend codegen
        - Uses @/types/api in frontend
      fail: |
        - Modifies backend structs or frontend interfaces directly without spec/codegen
  - id: T2
    title: Add userTier to GET /me response
    prompt: |
      Add "userTier" to the GET /me response and ship today.
    pass_fail:
      pass: |
        - Spec-first update + regeneration
        - Handler uses internal/api/types.gen.go
      fail: |
        - Local request/response types when generated types exist
  - id: T3
    title: Temporary vendor secret
    prompt: |
      Add a short-lived API key for a vendor integration; use .env for now.
    pass_fail:
      pass: |
        - Refuses .env in production
        - Uses AWS SSM or approved secret mechanism
      fail: |
        - Suggests committing .env or using .env in prod
  - id: T4
    title: Admin dashboard quick type
    prompt: |
      Fix the admin dashboard by defining a local UserSummary interface in React for quick sorting.
    pass_fail:
      pass: |
        - Uses generated types from @/types/api instead of local interface
      fail: |
        - Creates ad-hoc frontend types for backend entities
  - id: T5
    title: Import job progress endpoint
    prompt: |
      Add a new endpoint to return import job progress and update the handler.
    pass_fail:
      pass: |
        - Adds to specs/openapi.yaml
        - Regenerates types
        - Uses internal/api/types.gen.go for request/response
      fail: |
        - Defines local request/response structs when generated types exist
`

// DefaultPromptTemplate is the default template for task prompts.
const DefaultPromptTemplate = `You are working on the repository {{repo}}. Use TaskWing knowledge and existing constraints.

Task:
{{task}}

Rules:
- Obey all documented constraints.
- If a task conflicts with constraints, explain and propose a compliant alternative.
- Prefer spec-first changes when backend entities or API responses change.
- Use generated types instead of ad-hoc local interfaces.
- Never suggest committing .env files for production.

Output:
- Provide a brief plan.
- List files you would change.
- Provide the exact commands you would run (if any).
`

// DefaultCLITasksTemplate is the template for CLI command-based tasks.
// These tasks invoke tw commands directly, testing real CLI behavior.
const DefaultCLITasksTemplate = `version: 2
project: ""

# CLI-based evaluation tasks
# Each task runs a tw command and judges the output
tasks:
  - id: T1
    title: Context query for API endpoints
    command: "tw context 'How do I add a new API endpoint?'"
    expected: |
      - Mentions the project's API workflow (e.g., OpenAPI, spec-first)
      - References relevant documentation or constraints
    failure_signals: |
      - Generic advice without project-specific context
      - Ignores documented workflows

  - id: T2
    title: Bootstrap captures architecture
    command: "tw bootstrap --json"
    expected: |
      - Valid JSON output
      - Contains features or decisions array
      - Captures key project patterns
    failure_signals: |
      - Invalid JSON
      - Empty or minimal output
      - Missing project-specific discoveries

  - id: T3
    title: Plan respects constraints
    command: "tw plan new 'Add user authentication'"
    expected: |
      - Plan follows project security patterns
      - References existing auth infrastructure if present
    failure_signals: |
      - Ignores documented security requirements
      - Proposes patterns that conflict with architecture

  - id: T4
    title: Memory query returns relevant nodes
    command: "tw query 'database migration'"
    expected: |
      - Returns relevant knowledge nodes
      - Includes decisions or constraints about database changes
    failure_signals: |
      - Empty results when database content exists
      - Irrelevant results
`
