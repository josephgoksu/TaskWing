// Package e2e contains end-to-end integration tests for the TaskWing CLI.
// These tests compile and run the binary as a black box, simulating real user scenarios.
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testEnv holds the configuration for a single test environment.
type testEnv struct {
	binaryPath    string // Path to the compiled TaskWing binary
	workspacePath string // Path to the isolated project workspace
	t             *testing.T
}

// newTestEnv creates a new isolated test environment.
// It compiles the binary and creates a temporary workspace.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// Create temp directory for the binary
	binDir, err := os.MkdirTemp("", "taskwing-e2e-bin-*")
	if err != nil {
		t.Fatalf("failed to create temp bin directory: %v", err)
	}

	// Create temp directory for the project workspace
	workspaceDir, err := os.MkdirTemp("", "taskwing-e2e-workspace-*")
	if err != nil {
		os.RemoveAll(binDir)
		t.Fatalf("failed to create temp workspace directory: %v", err)
	}

	binaryPath := filepath.Join(binDir, "taskwing")

	env := &testEnv{
		binaryPath:    binaryPath,
		workspacePath: workspaceDir,
		t:             t,
	}

	// Register cleanup
	t.Cleanup(func() {
		os.RemoveAll(binDir)
		os.RemoveAll(workspaceDir)
	})

	// Compile the binary
	env.compileBinary()

	return env
}

// compileBinary compiles the TaskWing binary from the project root.
func (e *testEnv) compileBinary() {
	e.t.Helper()

	// Find the project root (go.mod location)
	projectRoot, err := findProjectRoot()
	if err != nil {
		e.t.Fatalf("failed to find project root: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", e.binaryPath, ".")
	cmd.Dir = projectRoot
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("failed to compile TaskWing binary:\n%s\nerror: %v", string(output), err)
	}

	// Verify binary exists and is executable
	if _, err := os.Stat(e.binaryPath); err != nil {
		e.t.Fatalf("binary not found after compilation: %v", err)
	}
}

// findProjectRoot walks up from the current directory to find go.mod.
func findProjectRoot() (string, error) {
	// Start from the test file's directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}

// setupMockProject creates a mock project structure in the workspace.
func (e *testEnv) setupMockProject(files map[string]string) {
	e.t.Helper()

	for relPath, content := range files {
		fullPath := filepath.Join(e.workspacePath, relPath)

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			e.t.Fatalf("failed to create directory for %s: %v", relPath, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			e.t.Fatalf("failed to write file %s: %v", relPath, err)
		}
	}
}

// initGitRepo initializes a git repository in the workspace.
func (e *testEnv) initGitRepo() {
	e.t.Helper()

	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@taskwing.test"},
		{"git", "config", "user.name", "TaskWing E2E Test"},
		{"git", "add", "."},
		{"git", "commit", "-m", "Initial commit"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = e.workspacePath

		output, err := cmd.CombinedOutput()
		if err != nil {
			e.t.Fatalf("git command %v failed:\n%s\nerror: %v", args, string(output), err)
		}
	}
}

// runCLI executes the TaskWing CLI with the given arguments.
// Returns stdout, stderr, and error.
func (e *testEnv) runCLI(args ...string) (stdout, stderr string, err error) {
	e.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	cmd.Dir = e.workspacePath

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"TASKWING_QUIET=true", // Suppress interactive prompts
	)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Provide empty stdin to avoid blocking on prompts
	cmd.Stdin = strings.NewReader("\n\n\n")

	err = cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}

// requireEnvVar checks that an environment variable is set.
// Skips the test if not set.
func requireEnvVar(t *testing.T, name string) string {
	t.Helper()

	value := os.Getenv(name)
	if value == "" {
		t.Skipf("skipping: %s environment variable not set", name)
	}
	return value
}

// getArchitectureDoc returns the architecture documentation content.
func getArchitectureDoc() string {
	return `# Project Architecture

## Database Layer

Our application uses a **read-replica pattern** for scalability.

### CRITICAL Database Access Rules

> **CRITICAL: All high-volume read operations MUST use the ` + "`db.ReadReplica`" + ` instance.**

This is mandatory for:
- Product listings
- Search results
- Analytics queries
- Top-selling reports
- Any endpoint that may receive >100 requests/minute

### Caching Strategy

For high-volume endpoints, always implement:
- Redis cache layer with appropriate TTL
- Consider using ` + "`Cache.WithFallback()`" + ` for graceful degradation

### Code Examples

**CORRECT** (High-volume read):
` + "```go\n" + `products, err := db.ReadReplica.Query("SELECT * FROM products ORDER BY sales DESC")
` + "```" + `

**INCORRECT** (Will cause primary DB overload):
` + "```go\n" + `products, err := db.Primary.Query("SELECT * FROM products ORDER BY sales DESC")
` + "```" + `

## Performance Requirements

- ReadReplica handles read traffic
- Primary only for writes
- Cache heavy-read endpoints
`
}

// getRepoBaseFile returns the base repository file content.
func getRepoBaseFile() string {
	return `package repo

import "database/sql"

// DB holds database connections for the application.
type DB struct {
	// Primary is the main database connection for write operations.
	Primary *sql.DB

	// ReadReplica is the read-replica connection for high-volume read operations.
	// CRITICAL: Use this for all high-traffic read endpoints!
	ReadReplica *sql.DB
}

// Cache provides caching capabilities.
type Cache struct {
	client interface{}
}

// WithFallback wraps a cache operation with fallback to database.
func (c *Cache) WithFallback(key string, ttl int, fallback func() (interface{}, error)) (interface{}, error) {
	// Implementation
	return nil, nil
}

// NewDB creates a new database connection pool.
func NewDB(primaryDSN, replicaDSN string) (*DB, error) {
	return &DB{}, nil
}
`
}

// getProductRepoFile returns the product repository file content.
func getProductRepoFile() string {
	return `package repo

// Product represents a product in the system.
type Product struct {
	ID    int64
	Name  string
	Sales int64
	Price float64
}

// ProductRepository handles product data access.
type ProductRepository struct {
	db *DB
}

// NewProductRepository creates a new product repository.
func NewProductRepository(db *DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// GetByID retrieves a single product by ID (low-volume, can use Primary).
func (r *ProductRepository) GetByID(id int64) (*Product, error) {
	// Low volume operation, Primary is fine
	return nil, nil
}
`
}

// getMainFile returns the main.go file content.
func getMainFile() string {
	return `package main

import "fmt"

func main() {
	fmt.Println("Product Catalog Service")
}
`
}

// getGoModFile returns the go.mod file content.
func getGoModFile() string {
	return `module github.com/example/product-catalog

go 1.21
`
}

// TestReadReplicaMandate verifies that TaskWing enforces architectural constraints
// when planning high-volume read endpoints.
//
// Scenario: "The Read-Replica Mandate"
// When a user asks for a high-volume read endpoint, the CLI should suggest
// using the ReadReplica database connection, adhering to the project's
// documented architecture.
func TestReadReplicaMandate(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Require OpenAI API key
	requireEnvVar(t, "OPENAI_API_KEY")

	t.Log("Setting up test environment...")
	env := newTestEnv(t)

	// Define mock project files
	mockFiles := map[string]string{
		"docs/architecture.md":     getArchitectureDoc(),
		"internal/repo/base.go":    getRepoBaseFile(),
		"internal/repo/product.go": getProductRepoFile(),
		"main.go":                  getMainFile(),
		"go.mod":                   getGoModFile(),
	}

	t.Log("Setting up mock project structure...")
	env.setupMockProject(mockFiles)

	t.Log("Initializing git repository...")
	env.initGitRepo()

	// Step 1: Run bootstrap
	t.Log("Running 'taskwing bootstrap'...")
	stdout, stderr, err := env.runCLI("bootstrap")
	if err != nil {
		t.Logf("bootstrap stdout:\n%s", stdout)
		t.Logf("bootstrap stderr:\n%s", stderr)
		t.Fatalf("bootstrap command failed: %v", err)
	}
	t.Log("Bootstrap completed successfully")

	// Step 2: Run plan new
	goal := "Create a new API endpoint to list high-volume top-selling products"
	t.Logf("Running 'taskwing plan new \"%s\"'...", goal)

	stdout, stderr, err = env.runCLI("plan", "new", goal)
	// Note: plan command may fail for various reasons, but we still analyze output
	combinedOutput := stdout + stderr

	t.Logf("Plan output:\n%s", combinedOutput)

	if err != nil {
		t.Logf("plan command returned error (may be expected): %v", err)
		// Don't fail immediately - check if we got meaningful output
	}

	// Step 3: Analyze output
	t.Log("Analyzing plan output for architectural compliance...")

	// PASS conditions: Output mentions ReadReplica AND Cache
	hasReadReplica := strings.Contains(combinedOutput, "ReadReplica") ||
		strings.Contains(combinedOutput, "read replica") ||
		strings.Contains(combinedOutput, "read-replica") ||
		strings.Contains(combinedOutput, "replica")

	hasCache := strings.Contains(combinedOutput, "Cache") ||
		strings.Contains(combinedOutput, "cache") ||
		strings.Contains(combinedOutput, "caching") ||
		strings.Contains(combinedOutput, "Redis")

	// FAIL conditions: Raw SQL without replica, or Primary database usage for reads
	hasBadPattern := (strings.Contains(combinedOutput, "db.Primary.Query") ||
		strings.Contains(combinedOutput, "Primary.Query")) &&
		!strings.Contains(combinedOutput, "ReadReplica")

	// Output analysis summary
	t.Logf("Analysis results:")
	t.Logf("  - Mentions ReadReplica: %v", hasReadReplica)
	t.Logf("  - Mentions Cache: %v", hasCache)
	t.Logf("  - Has bad patterns: %v", hasBadPattern)

	// Final assertions
	if hasBadPattern {
		t.Error("FAIL: Plan suggests using Primary database for high-volume reads, ignoring replica constraint")
	}

	if !hasReadReplica {
		t.Error("FAIL: Plan does not mention ReadReplica for high-volume read endpoint")
	}

	if !hasCache {
		t.Error("FAIL: Plan does not mention Cache/caching for high-volume endpoint")
	}

	if hasReadReplica && hasCache && !hasBadPattern {
		t.Log("PASS: Plan correctly recommends ReadReplica and Cache for high-volume reads")
	}
}

// TestBootstrapCreatesMemory verifies that bootstrap creates the .taskwing directory.
func TestBootstrapCreatesMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	requireEnvVar(t, "OPENAI_API_KEY")

	env := newTestEnv(t)

	// Minimal project setup
	mockFiles := map[string]string{
		"main.go":   getMinimalMainFile(),
		"go.mod":    getMinimalGoModFile(),
		"README.md": "# Minimal Project\n\nA test project.\n",
	}

	env.setupMockProject(mockFiles)
	env.initGitRepo()

	// Run bootstrap
	stdout, stderr, err := env.runCLI("bootstrap")
	if err != nil {
		t.Logf("stdout: %s", stdout)
		t.Logf("stderr: %s", stderr)
		t.Fatalf("bootstrap failed: %v", err)
	}

	// Check that .taskwing directory was created
	taskwingDir := filepath.Join(env.workspacePath, ".taskwing")
	if _, err := os.Stat(taskwingDir); os.IsNotExist(err) {
		t.Error("bootstrap did not create .taskwing directory")
	}

	// Check for memory.db
	memoryDB := filepath.Join(taskwingDir, "memory", "memory.db")
	if _, err := os.Stat(memoryDB); os.IsNotExist(err) {
		t.Error("bootstrap did not create memory.db")
	}
}

// getMinimalMainFile returns a minimal main.go content.
func getMinimalMainFile() string {
	return `package main

func main() {}
`
}

// getMinimalGoModFile returns a minimal go.mod content.
func getMinimalGoModFile() string {
	return `module github.com/example/minimal

go 1.21
`
}

// TestCLIVersionFlag verifies the --version flag works.
func TestCLIVersionFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)

	stdout, _, err := env.runCLI("version")
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	// Should contain version information
	if !strings.Contains(stdout, "taskwing") && !strings.Contains(stdout, "version") {
		t.Errorf("version output unexpected: %s", stdout)
	}
}

// TestCLIHelpFlag verifies the --help flag works.
func TestCLIHelpFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)

	stdout, _, err := env.runCLI("--help")
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}

	// Should contain usage information
	if !strings.Contains(stdout, "Usage") && !strings.Contains(stdout, "bootstrap") {
		t.Errorf("help output unexpected: %s", stdout)
	}
}

// TestSecurityMiddlewareMandate verifies that TaskWing enforces security constraints
// when planning public-facing API endpoints.
//
// Scenario: "The Security Middleware Mandate"
// When a user asks for a public API endpoint that handles user data,
// the CLI should suggest using JWT authentication middleware and input validation,
// adhering to the project's documented security requirements.
func TestSecurityMiddlewareMandate(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Require OpenAI API key
	requireEnvVar(t, "OPENAI_API_KEY")

	t.Log("Setting up test environment...")
	env := newTestEnv(t)

	// Define mock project files with security constraints
	mockFiles := map[string]string{
		"docs/security.md":                 getSecurityDoc(),
		"internal/middleware/auth.go":      getAuthMiddlewareFile(),
		"internal/middleware/validator.go": getValidatorMiddlewareFile(),
		"main.go":                          getSecurityMainFile(),
		"go.mod":                           getSecurityGoModFile(),
	}

	t.Log("Setting up mock project structure...")
	env.setupMockProject(mockFiles)

	t.Log("Initializing git repository...")
	env.initGitRepo()

	// Step 1: Run bootstrap
	t.Log("Running 'taskwing bootstrap'...")
	stdout, stderr, err := env.runCLI("bootstrap")
	if err != nil {
		t.Logf("bootstrap stdout:\n%s", stdout)
		t.Logf("bootstrap stderr:\n%s", stderr)
		t.Fatalf("bootstrap command failed: %v", err)
	}
	t.Log("Bootstrap completed successfully")

	// Step 2: Run plan new with a security-sensitive goal
	goal := "Create a public REST API endpoint to update user profile information including email and password"
	t.Logf("Running 'taskwing plan new \"%s\"'...", goal)

	stdout, stderr, err = env.runCLI("plan", "new", goal)
	combinedOutput := stdout + stderr

	t.Logf("Plan output:\n%s", combinedOutput)

	if err != nil {
		t.Logf("plan command returned error (may be expected): %v", err)
	}

	// Step 3: Analyze output for security compliance
	t.Log("Analyzing plan output for security compliance...")

	// PASS conditions: Output mentions JWT/Auth AND Validation/Sanitization
	hasAuth := strings.Contains(combinedOutput, "JWT") ||
		strings.Contains(combinedOutput, "jwt") ||
		strings.Contains(combinedOutput, "AuthMiddleware") ||
		strings.Contains(combinedOutput, "auth middleware") ||
		strings.Contains(combinedOutput, "authentication") ||
		strings.Contains(combinedOutput, "Authorization")

	hasValidation := strings.Contains(combinedOutput, "Validation") ||
		strings.Contains(combinedOutput, "validation") ||
		strings.Contains(combinedOutput, "ValidateInput") ||
		strings.Contains(combinedOutput, "sanitize") ||
		strings.Contains(combinedOutput, "Sanitize") ||
		strings.Contains(combinedOutput, "input validation")

	// FAIL conditions: Suggests direct database writes without auth
	hasBadPattern := strings.Contains(combinedOutput, "directly update") &&
		!hasAuth

	// Output analysis summary
	t.Logf("Analysis results:")
	t.Logf("  - Mentions Authentication: %v", hasAuth)
	t.Logf("  - Mentions Input Validation: %v", hasValidation)
	t.Logf("  - Has bad patterns: %v", hasBadPattern)

	// Final assertions
	if hasBadPattern {
		t.Error("FAIL: Plan suggests direct data mutation without authentication")
	}

	if !hasAuth {
		t.Error("FAIL: Plan does not mention JWT/authentication for user-facing endpoint")
	}

	if !hasValidation {
		t.Error("FAIL: Plan does not mention input validation/sanitization for user data endpoint")
	}

	if hasAuth && hasValidation && !hasBadPattern {
		t.Log("PASS: Plan correctly recommends authentication and input validation for public API")
	}
}

// getSecurityDoc returns the security documentation content.
func getSecurityDoc() string {
	return `# Security Guidelines

## Authentication Requirements

> **CRITICAL: All public-facing API endpoints MUST use the ` + "`AuthMiddleware.JWT()`" + ` middleware.**

This applies to:
- User profile endpoints
- Payment endpoints
- Admin endpoints
- Any endpoint that reads or writes user data

### Implementation

` + "```go" + `
router.POST("/api/v1/users/:id", AuthMiddleware.JWT(), handler.UpdateUser)
` + "```" + `

## Input Validation Requirements

> **MANDATORY: All user input MUST be validated using ` + "`ValidateInput()`" + ` before processing.**

This is required for:
- Form submissions
- JSON payloads
- Query parameters
- URL path parameters

### Validation Rules

- Email: Must match RFC 5322 format
- Password: Minimum 12 characters, must include uppercase, lowercase, number, and symbol
- User IDs: Must be valid UUIDs

### Example

` + "```go" + `
func UpdateUser(c *gin.Context) {
    var input UpdateUserRequest
    if err := ValidateInput(c, &input); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    // Process validated input
}
` + "```" + `

## Security Checklist

- [ ] JWT middleware applied to route
- [ ] Input validation on all fields
- [ ] Rate limiting enabled
- [ ] Audit logging for mutations
`
}

// getAuthMiddlewareFile returns the auth middleware file content.
func getAuthMiddlewareFile() string {
	return `package middleware

import "github.com/gin-gonic/gin"

// AuthMiddleware provides authentication middleware functions.
type AuthMiddleware struct{}

// JWT returns a middleware that validates JWT tokens.
// CRITICAL: This MUST be used on all public-facing endpoints.
func (a *AuthMiddleware) JWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Token validation logic
		c.Next()
	}
}

// APIKey returns a middleware for API key authentication.
func (a *AuthMiddleware) APIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		// API key validation logic
		c.Next()
	}
}
`
}

// getValidatorMiddlewareFile returns the validator middleware file content.
func getValidatorMiddlewareFile() string {
	return `package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateInput validates the request body against struct tags.
// MANDATORY: Use this for all user input processing.
func ValidateInput(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindJSON(obj); err != nil {
		return err
	}
	return validate.Struct(obj)
}

// SanitizeString removes potentially dangerous characters from input.
func SanitizeString(input string) string {
	// Sanitization logic
	return input
}
`
}

// getSecurityMainFile returns the main.go file content for security test.
func getSecurityMainFile() string {
	return `package main

import "fmt"

func main() {
	fmt.Println("User Profile Service")
}
`
}

// getSecurityGoModFile returns the go.mod file content for security test.
func getSecurityGoModFile() string {
	return `module github.com/example/user-profile-service

go 1.21

require github.com/gin-gonic/gin v1.9.1
`
}

// =============================================================================
// MARKWISE-INSPIRED TESTS
// These tests are based on real architectural constraints from markwise.app
// =============================================================================

// TestEmbeddedVectorMandate verifies that TaskWing enforces the use of
// embedded LanceDB for vector search, not external services like Pinecone.
//
// Scenario: "The Embedded Vector Mandate"
// When a user asks for semantic search functionality, the CLI should
// recommend using the internal/vectorsearch package with LanceDB,
// not external vector database services.
func TestEmbeddedVectorMandate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	requireEnvVar(t, "OPENAI_API_KEY")

	t.Log("Setting up test environment...")
	env := newTestEnv(t)

	// Define mock project files based on real Markwise architecture
	mockFiles := map[string]string{
		"docs/ARCHITECTURE.md":             getMarkwiseArchitectureDoc(),
		"internal/vectorsearch/README.md":  getVectorSearchReadme(),
		"internal/vectorsearch/service.go": getVectorSearchServiceFile(),
		"internal/search/handler.go":       getSearchHandlerFile(),
		"main.go":                          getMarkwiseMainFile(),
		"go.mod":                           getMarkwiseGoModFile(),
	}

	t.Log("Setting up mock project structure...")
	env.setupMockProject(mockFiles)

	t.Log("Initializing git repository...")
	env.initGitRepo()

	// Step 1: Run bootstrap
	t.Log("Running 'taskwing bootstrap'...")
	stdout, stderr, err := env.runCLI("bootstrap")
	if err != nil {
		t.Logf("bootstrap stdout:\n%s", stdout)
		t.Logf("bootstrap stderr:\n%s", stderr)
		t.Fatalf("bootstrap command failed: %v", err)
	}
	t.Log("Bootstrap completed successfully")

	// Step 2: Run plan with a vector search goal
	goal := "Add semantic search for user collections to find similar bookmarks"
	t.Logf("Running 'taskwing plan new \"%s\"'...", goal)

	stdout, stderr, err = env.runCLI("plan", "new", goal)
	combinedOutput := stdout + stderr

	t.Logf("Plan output:\n%s", combinedOutput)

	if err != nil {
		t.Logf("plan command returned error (may be expected): %v", err)
	}

	// Step 3: Analyze output for LanceDB compliance
	t.Log("Analyzing plan output for embedded vector mandate compliance...")

	// PASS conditions: Mentions LanceDB or internal vectorsearch
	hasLanceDB := strings.Contains(combinedOutput, "LanceDB") ||
		strings.Contains(combinedOutput, "lancedb") ||
		strings.Contains(combinedOutput, "lance")

	hasInternalVectorSearch := strings.Contains(combinedOutput, "vectorsearch") ||
		strings.Contains(combinedOutput, "internal/vectorsearch") ||
		strings.Contains(combinedOutput, "embedded vector")

	// FAIL conditions: Suggests using external vector services (positive recommendation)
	// Note: We check for patterns that indicate USING the service, not just mentioning it
	// (e.g., "use Pinecone" vs "do NOT use Elasticsearch" - the latter is correct advice)
	lowerOutput := strings.ToLower(combinedOutput)
	hasPinecone := strings.Contains(lowerOutput, "use pinecone") ||
		strings.Contains(lowerOutput, "integrate pinecone") ||
		strings.Contains(lowerOutput, "pinecone.io")
	hasWeaviate := strings.Contains(lowerOutput, "use weaviate") ||
		strings.Contains(lowerOutput, "integrate weaviate")
	hasQdrant := strings.Contains(lowerOutput, "use qdrant") ||
		strings.Contains(lowerOutput, "integrate qdrant")
	hasMilvus := strings.Contains(lowerOutput, "use milvus") ||
		strings.Contains(lowerOutput, "integrate milvus")

	// For Elasticsearch, only fail if it's recommended for VECTOR operations
	// (Elasticsearch is sometimes used for keyword search which is OK)
	hasElasticForVectors := (strings.Contains(lowerOutput, "elasticsearch") ||
		strings.Contains(lowerOutput, "opensearch")) &&
		(strings.Contains(lowerOutput, "vector") || strings.Contains(lowerOutput, "embedding")) &&
		!strings.Contains(lowerOutput, "not") && !strings.Contains(lowerOutput, "don't")

	hasExternalService := hasPinecone || hasWeaviate || hasElasticForVectors || hasQdrant || hasMilvus

	// Output analysis summary
	t.Logf("Analysis results:")
	t.Logf("  - Mentions LanceDB: %v", hasLanceDB)
	t.Logf("  - Mentions internal/vectorsearch: %v", hasInternalVectorSearch)
	t.Logf("  - Recommends external service: %v", hasExternalService)

	// Final assertions
	if hasExternalService {
		t.Errorf("FAIL: Plan recommends external vector service instead of embedded LanceDB")
	}

	if !hasLanceDB && !hasInternalVectorSearch {
		t.Error("FAIL: Plan does not mention LanceDB or internal vectorsearch package")
	}

	if (hasLanceDB || hasInternalVectorSearch) && !hasExternalService {
		t.Log("PASS: Plan correctly recommends embedded LanceDB for vector search")
	}
}

// TestOpenAPIContractSyncMandate verifies that TaskWing enforces the
// OpenAPI-first development workflow where types are generated from
// specs/openapi.yaml to ensure frontend/backend sync.
//
// Scenario: "The OpenAPI Contract Sync Mandate"
// When adding a new API endpoint, the CLI should recommend updating
// openapi.yaml first, then running code generation, not creating
// manual type definitions.
func TestOpenAPIContractSyncMandate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	requireEnvVar(t, "OPENAI_API_KEY")

	t.Log("Setting up test environment...")
	env := newTestEnv(t)

	// Define mock project files based on real Markwise architecture
	mockFiles := map[string]string{
		"docs/API_CONTRACT.md":          getAPIContractDoc(),
		"specs/openapi.yaml":            getOpenAPISpecFile(),
		"internal/api/types.gen.go":     getGeneratedTypesFile(),
		"internal/bookmarks/handler.go": getBookmarksHandlerFile(),
		"web/src/services/api.gen.ts":   getGeneratedTSTypesFile(),
		"Makefile":                      getMakefileWithGenerate(),
		"main.go":                       getAPIMainFile(),
		"go.mod":                        getAPIGoModFile(),
	}

	t.Log("Setting up mock project structure...")
	env.setupMockProject(mockFiles)

	t.Log("Initializing git repository...")
	env.initGitRepo()

	// Step 1: Run bootstrap
	t.Log("Running 'taskwing bootstrap'...")
	stdout, stderr, err := env.runCLI("bootstrap")
	if err != nil {
		t.Logf("bootstrap stdout:\n%s", stdout)
		t.Logf("bootstrap stderr:\n%s", stderr)
		t.Fatalf("bootstrap command failed: %v", err)
	}
	t.Log("Bootstrap completed successfully")

	// Step 2: Run plan with an API endpoint goal
	goal := "Add a new API endpoint to export bookmarks as CSV format"
	t.Logf("Running 'taskwing plan new \"%s\"'...", goal)

	stdout, stderr, err = env.runCLI("plan", "new", goal)
	combinedOutput := stdout + stderr

	t.Logf("Plan output:\n%s", combinedOutput)

	if err != nil {
		t.Logf("plan command returned error (may be expected): %v", err)
	}

	// Step 3: Analyze output for OpenAPI workflow compliance
	t.Log("Analyzing plan output for OpenAPI contract sync compliance...")

	// PASS conditions: Mentions OpenAPI spec and code generation
	hasOpenAPIUpdate := strings.Contains(combinedOutput, "openapi.yaml") ||
		strings.Contains(combinedOutput, "OpenAPI") ||
		strings.Contains(combinedOutput, "openapi")

	hasCodeGeneration := strings.Contains(combinedOutput, "generate-api") ||
		strings.Contains(combinedOutput, "generate:api") ||
		strings.Contains(combinedOutput, "code generation") ||
		strings.Contains(combinedOutput, "generated types") ||
		strings.Contains(combinedOutput, "types.gen")

	hasTypeSync := strings.Contains(combinedOutput, "frontend") ||
		strings.Contains(combinedOutput, "TypeScript") ||
		strings.Contains(combinedOutput, "sync") ||
		strings.Contains(combinedOutput, "contract")

	// FAIL conditions: Manual type definitions without mentioning OpenAPI
	hasManualTypes := (strings.Contains(combinedOutput, "create struct") ||
		strings.Contains(combinedOutput, "define struct") ||
		strings.Contains(combinedOutput, "new struct")) && !hasOpenAPIUpdate

	// Output analysis summary
	t.Logf("Analysis results:")
	t.Logf("  - Mentions OpenAPI spec: %v", hasOpenAPIUpdate)
	t.Logf("  - Mentions code generation: %v", hasCodeGeneration)
	t.Logf("  - Mentions type sync: %v", hasTypeSync)
	t.Logf("  - Suggests manual types without OpenAPI: %v", hasManualTypes)

	// Final assertions
	if hasManualTypes {
		t.Error("FAIL: Plan suggests manual type definitions without OpenAPI workflow")
	}

	if !hasOpenAPIUpdate {
		t.Error("FAIL: Plan does not mention updating openapi.yaml")
	}

	// Code generation is ideal but not strictly required if OpenAPI is mentioned
	if hasOpenAPIUpdate && !hasManualTypes {
		t.Log("PASS: Plan correctly recommends OpenAPI-first workflow")
	}
}

// =============================================================================
// MARKWISE MOCK FILE HELPERS
// =============================================================================

func getMarkwiseArchitectureDoc() string {
	return `# Markwise Architecture

## Vector Search

> **CRITICAL: All vector search operations MUST use the embedded LanceDB via ` + "`internal/vectorsearch`" + `.**

We use LanceDB as an embedded vector database with CGO bindings. This is a deliberate architectural choice:

### Why Embedded LanceDB (NOT Pinecone/Weaviate/etc.)

1. **Cost Efficiency** - No external service costs, runs on EFS
2. **Latency** - Sub-150ms queries without network hops
3. **Simplicity** - No separate service to deploy/maintain
4. **Data Locality** - Vectors stored alongside MongoDB data

### Usage

` + "```go" + `
import "github.com/company/app/internal/vectorsearch"

// Use the vectorsearch.Service for all semantic search
results, err := vectorSearchSvc.Search(ctx, query, userID, limit)
` + "```" + `

### What NOT to do

- ❌ Do NOT add Pinecone, Weaviate, Milvus, or other external vector DBs
- ❌ Do NOT create a separate vector search microservice
- ❌ Do NOT use Elasticsearch for vector similarity

### Embedding Model

We use OpenAI's ` + "`text-embedding-3-small`" + ` model exclusively.
`
}

func getVectorSearchReadme() string {
	return `# vectorsearch

> Embedded LanceDB vector search with OpenAI embeddings

## Purpose

The vectorsearch package provides semantic search using LanceDB (embedded) and OpenAI embeddings.

## CRITICAL: This is the ONLY vector search solution

All semantic search in the application MUST go through this package. Do NOT add external vector databases.

## Service Interface

` + "```go" + `
type Service interface {
    Search(ctx, query, userID, limit) ([]SearchResult, error)
    Index(ctx, doc) error
    Delete(ctx, docID) error
}
` + "```" + `
`
}

func getVectorSearchServiceFile() string {
	return `package vectorsearch

import (
	"context"
)

// Service provides vector search capabilities using embedded LanceDB.
// CRITICAL: This is the ONLY approved vector search solution.
type Service interface {
	Search(ctx context.Context, query string, userID string, limit int) ([]SearchResult, error)
	Index(ctx context.Context, doc *Document) error
	Delete(ctx context.Context, docID string) error
	Close() error
}

type SearchResult struct {
	ID       string
	Score    float64
	Document *Document
}

type Document struct {
	ID      string
	UserID  string
	Title   string
	Content string
}

type lanceDBService struct {
	// LanceDB connection via CGO
}

func New(ctx context.Context, config Config) (Service, error) {
	// Initialize embedded LanceDB
	return &lanceDBService{}, nil
}
`
}

func getSearchHandlerFile() string {
	return `package search

import (
	"net/http"
)

// Handler uses internal/vectorsearch for semantic search
type Handler struct {
	// vectorSearchSvc vectorsearch.Service
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	// Uses embedded LanceDB via vectorsearch package
}
`
}

func getMarkwiseMainFile() string {
	return `package main

import "fmt"

func main() {
	fmt.Println("Bookmark Manager with Embedded Vector Search")
}
`
}

func getMarkwiseGoModFile() string {
	return `module github.com/company/bookmark-manager

go 1.24
`
}

func getAPIContractDoc() string {
	return `# API Contract Guidelines

## OpenAPI is the Source of Truth

> **CRITICAL: The OpenAPI spec (` + "`specs/openapi.yaml`" + `) is the Source of Truth for ALL API endpoints.**

### Workflow for Adding/Modifying Endpoints

1. **First**: Update ` + "`specs/openapi.yaml`" + ` with the new endpoint and schemas
2. **Then**: Run ` + "`make generate-api`" + ` to update Go types in ` + "`internal/api/types.gen.go`" + `
3. **Then**: Run ` + "`bun run generate:api`" + ` to update TypeScript types
4. **Finally**: Implement the handler using generated types

### Why This Matters

- Frontend and backend share generated types (no drift)
- API documentation is always current
- Type-safe request/response handling

### Generated Types (MANDATORY)

All HTTP handlers MUST use types from ` + "`internal/api/types.gen.go`" + `:

` + "```go" + `
// DO:
var req api.CreateBookmarkRequest  // ✓ Generated type

// DON'T:
var req CreateBookmarkRequest      // ✗ Local model.go type
` + "```" + `

**NEVER skip this workflow.** Missing OpenAPI definitions cause type mismatches.
`
}

func getOpenAPISpecFile() string {
	return `openapi: 3.1.0
info:
  title: Bookmark Manager API
  version: 1.0.0

paths:
  /bookmarks:
    get:
      summary: List bookmarks
      responses:
        '200':
          description: List of bookmarks
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/BookmarkList'

components:
  schemas:
    BookmarkList:
      type: object
      properties:
        bookmarks:
          type: array
          items:
            $ref: '#/components/schemas/Bookmark'
    Bookmark:
      type: object
      properties:
        id:
          type: string
        url:
          type: string
        title:
          type: string
`
}

func getGeneratedTypesFile() string {
	return `// Package api provides generated types from OpenAPI spec.
// DO NOT EDIT - Generated by oapi-codegen
package api

// BookmarkList - Response for listing bookmarks
type BookmarkList struct {
	Bookmarks []Bookmark ` + "`json:\"bookmarks\"`" + `
}

// Bookmark - A user bookmark
type Bookmark struct {
	ID    string ` + "`json:\"id\"`" + `
	URL   string ` + "`json:\"url\"`" + `
	Title string ` + "`json:\"title\"`" + `
}
`
}

func getBookmarksHandlerFile() string {
	return `package bookmarks

import (
	"net/http"

	"github.com/company/app/internal/api"
)

// Handler handles bookmark HTTP requests using generated types
type Handler struct{}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	// Uses api.BookmarkList - generated from OpenAPI
	var response api.BookmarkList
	_ = response
}
`
}

func getGeneratedTSTypesFile() string {
	return `// Generated from openapi.yaml - DO NOT EDIT

export interface BookmarkList {
  bookmarks: Bookmark[];
}

export interface Bookmark {
  id: string;
  url: string;
  title: string;
}
`
}

func getMakefileWithGenerate() string {
	return `# Makefile

.PHONY: generate-api

# Generate Go types from OpenAPI spec
# CRITICAL: Run this after updating specs/openapi.yaml
generate-api:
	oapi-codegen -generate types -o internal/api/types.gen.go specs/openapi.yaml
	@echo "Go types generated. Now run 'bun run generate:api' in web/"
`
}

func getAPIMainFile() string {
	return `package main

import "fmt"

func main() {
	fmt.Println("API Server with OpenAPI Contract")
}
`
}

func getAPIGoModFile() string {
	return `module github.com/company/api-service

go 1.24
`
}
