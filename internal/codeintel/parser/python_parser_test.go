package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPythonParser_ParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "example.py")

	// Note: No leading newline - class definitions must start at column 0
	content := `MAX_RETRIES = 3
DEFAULT_TIMEOUT = 30

class User:
    """Represents a user in the system."""

    def __init__(self, name: str, email: str):
        """Initialize a new user."""
        self.name = name
        self.email = email

    def get_display_name(self) -> str:
        """Returns the display name for the user."""
        return self.name

    def _private_method(self):
        """A private method."""
        pass


class AdminUser(User):
    """Admin user with elevated privileges."""

    def __init__(self, name: str, email: str, role: str):
        super().__init__(name, email)
        self.role = role

    def get_permissions(self) -> list:
        """Get admin permissions."""
        return ["read", "write", "delete"]


def greet(name: str) -> str:
    """Return a greeting message."""
    return f"Hello, {name}!"


async def fetch_data(url: str) -> dict:
    """Async function to fetch data from URL."""
    pass


typed_var: int = 42
`
	err := os.WriteFile(pyFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewPythonParser(tmpDir)
	result, err := parser.ParseFile(pyFile)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify symbols were extracted
	assert.NotEmpty(t, result.Symbols, "Should extract symbols from Python file")

	// Check for specific symbols
	symbolNames := make(map[string]bool)
	for _, sym := range result.Symbols {
		symbolNames[sym.Name] = true
		// Verify all symbols have the correct language
		assert.Equal(t, "python", sym.Language)
	}

	// Check for constants
	assert.True(t, symbolNames["MAX_RETRIES"], "Should extract MAX_RETRIES constant")
	assert.True(t, symbolNames["DEFAULT_TIMEOUT"], "Should extract DEFAULT_TIMEOUT constant")

	// Check for classes
	assert.True(t, symbolNames["User"], "Should extract User class")
	assert.True(t, symbolNames["AdminUser"], "Should extract AdminUser class")

	// Check for methods
	assert.True(t, symbolNames["__init__"], "Should extract __init__ method")
	assert.True(t, symbolNames["get_display_name"], "Should extract get_display_name method")
	assert.True(t, symbolNames["get_permissions"], "Should extract get_permissions method")

	// Check for functions
	assert.True(t, symbolNames["greet"], "Should extract greet function")
	assert.True(t, symbolNames["fetch_data"], "Should extract fetch_data async function")
}

func TestPythonParser_SupportedExtensions(t *testing.T) {
	parser := NewPythonParser("/test")

	extensions := parser.SupportedExtensions()
	assert.Contains(t, extensions, ".py")
	assert.Contains(t, extensions, ".pyi")
}

func TestPythonParser_CanParse(t *testing.T) {
	parser := NewPythonParser("/test")

	assert.True(t, parser.CanParse("app.py"))
	assert.True(t, parser.CanParse("app.pyi"))
	assert.True(t, parser.CanParse("/path/to/file.py"))
	assert.True(t, parser.CanParse("file.PY")) // Case insensitive
	assert.False(t, parser.CanParse("app.go"))
	assert.False(t, parser.CanParse("app.ts"))
	assert.False(t, parser.CanParse("app.rs"))
}

func TestPythonParser_ExtractVisibility(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "visibility.py")

	content := `
def public_func():
    pass

def _protected_func():
    pass

def __private_func():
    pass

class PublicClass:
    pass

class _ProtectedClass:
    pass
`
	err := os.WriteFile(pyFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewPythonParser(tmpDir)
	result, err := parser.ParseFile(pyFile)
	require.NoError(t, err)

	// Find symbols and check visibility
	for _, sym := range result.Symbols {
		switch sym.Name {
		case "public_func", "PublicClass":
			assert.Equal(t, "public", sym.Visibility, "%s should be public", sym.Name)
		case "_protected_func", "_ProtectedClass":
			assert.Equal(t, "protected", sym.Visibility, "%s should be protected", sym.Name)
		case "__private_func":
			assert.Equal(t, "private", sym.Visibility, "%s should be private", sym.Name)
		}
	}
}

func TestPythonParser_ParseDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source directory
	srcDir := filepath.Join(tmpDir, "src")
	err := os.MkdirAll(srcDir, 0755)
	require.NoError(t, err)

	// Create Python files
	files := map[string]string{
		"main.py":    "def main(): pass",
		"utils.py":   "def helper(): pass",
		"models.py":  "class User: pass",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(srcDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create __pycache__ (should be skipped)
	pycache := filepath.Join(srcDir, "__pycache__")
	err = os.MkdirAll(pycache, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(pycache, "cached.py"), []byte("def cached(): pass"), 0644)
	require.NoError(t, err)

	// Create venv (should be skipped)
	venv := filepath.Join(srcDir, "venv")
	err = os.MkdirAll(venv, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(venv, "lib.py"), []byte("def lib(): pass"), 0644)
	require.NoError(t, err)

	parser := NewPythonParser(srcDir)
	result, err := parser.ParseDirectory(srcDir)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have found symbols from source files but not __pycache__ or venv
	symbolNames := make(map[string]bool)
	for _, sym := range result.Symbols {
		symbolNames[sym.Name] = true
	}

	assert.True(t, symbolNames["main"], "Should find main function")
	assert.True(t, symbolNames["helper"], "Should find helper function")
	assert.True(t, symbolNames["User"], "Should find User class")
	assert.False(t, symbolNames["cached"], "Should NOT find cached from __pycache__")
	assert.False(t, symbolNames["lib"], "Should NOT find lib from venv")
}

func TestPythonParser_Language(t *testing.T) {
	parser := NewPythonParser("/test")
	assert.Equal(t, "python", parser.Language())
}

func TestPythonParserImplementsInterface(t *testing.T) {
	var _ LanguageParser = (*PythonParser)(nil)

	parser := NewPythonParser("/test")
	assert.NotNil(t, parser)
	assert.Equal(t, "python", parser.Language())
}

func TestPythonParser_TypeHints(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "typed.py")

	content := `from typing import List, Dict, Optional, Union

def process_data(
    items: List[str],
    config: Dict[str, Any],
    limit: Optional[int] = None
) -> List[Dict[str, str]]:
    """Process the data items."""
    pass

async def fetch_users(ids: List[int]) -> List[User]:
    """Fetch users by their IDs."""
    pass

class DataProcessor:
    """Processes data with type hints."""

    def transform(self, data: Dict[str, Any]) -> Optional[str]:
        """Transform data to string."""
        pass

    def validate(self, value: Union[str, int]) -> bool:
        """Validate the value."""
        pass
`
	err := os.WriteFile(pyFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewPythonParser(tmpDir)
	result, err := parser.ParseFile(pyFile)
	require.NoError(t, err)

	// Check that type hints are included in signatures
	for _, sym := range result.Symbols {
		switch sym.Name {
		case "process_data":
			assert.Contains(t, sym.Signature, "List[str]", "Should include List[str] type hint")
			assert.Contains(t, sym.Signature, "-> List[Dict[str, str]]", "Should include return type hint")
		case "fetch_users":
			assert.Contains(t, sym.Signature, "List[int]", "Should include List[int] type hint")
			assert.Contains(t, sym.Signature, "-> List[User]", "Should include return type hint")
		case "transform":
			assert.Contains(t, sym.Signature, "Dict[str, Any]", "Should include Dict type hint")
			assert.Contains(t, sym.Signature, "-> Optional[str]", "Should include Optional return type")
		case "validate":
			assert.Contains(t, sym.Signature, "Union[str, int]", "Should include Union type hint")
		}
	}
}

func TestPythonParser_GoogleStyleDocstrings(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "google_docs.py")

	content := `def calculate_sum(a: int, b: int) -> int:
    """Calculate the sum of two numbers.

    This function performs addition.

    Args:
        a (int): First number to add.
        b (int): Second number to add.

    Returns:
        int: The sum of a and b.

    Raises:
        ValueError: If inputs are not numbers.
        TypeError: If type conversion fails.
    """
    return a + b

class UserService:
    """Service for managing users.

    Attributes:
        db: Database connection.
        cache: Cache instance.
    """

    def create_user(self, name: str, email: str) -> User:
        """Create a new user in the system.

        Args:
            name (str): The user's full name.
            email (str): The user's email address.

        Returns:
            User: The newly created user object.

        Raises:
            ValidationError: If email is invalid.
        """
        pass
`
	err := os.WriteFile(pyFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewPythonParser(tmpDir)
	result, err := parser.ParseFile(pyFile)
	require.NoError(t, err)

	// Check docstring parsing
	for _, sym := range result.Symbols {
		switch sym.Name {
		case "calculate_sum":
			assert.Contains(t, sym.DocComment, "Calculate the sum", "Should have description")
			assert.Contains(t, sym.DocComment, "Params:", "Should have params section")
			assert.Contains(t, sym.DocComment, "a: int", "Should have param a with type")
			assert.Contains(t, sym.DocComment, "b: int", "Should have param b with type")
			assert.Contains(t, sym.DocComment, "Returns:", "Should have returns section")
		case "UserService":
			assert.Contains(t, sym.DocComment, "Service for managing users", "Should have class description")
		case "create_user":
			assert.Contains(t, sym.DocComment, "Create a new user", "Should have method description")
			assert.Contains(t, sym.DocComment, "name: str", "Should have name param with type")
		}
	}
}

func TestPythonParser_NumpyStyleDocstrings(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "numpy_docs.py")

	content := `def compute_statistics(data: List[float]) -> Dict[str, float]:
    """Compute basic statistics for the given data.

    A comprehensive function for statistical analysis.

    Parameters
    ----------
    data : List[float]
        Input data array for analysis.

    Returns
    -------
    Dict[str, float]
        Dictionary containing mean, median, and std.

    Raises
    ------
    ValueError
        If data is empty.
    """
    pass
`
	err := os.WriteFile(pyFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewPythonParser(tmpDir)
	result, err := parser.ParseFile(pyFile)
	require.NoError(t, err)

	// Find the function and check docstring
	for _, sym := range result.Symbols {
		if sym.Name == "compute_statistics" {
			assert.Contains(t, sym.DocComment, "Compute basic statistics", "Should have description")
			// NumPy style should be parsed
			t.Logf("NumPy docstring: %s", sym.DocComment)
		}
	}
}

func TestPythonParser_SphinxStyleDocstrings(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "sphinx_docs.py")

	content := `def send_email(to: str, subject: str, body: str) -> bool:
    """Send an email to the specified recipient.

    :param to: Email address of the recipient.
    :type to: str
    :param subject: Subject line of the email.
    :type subject: str
    :param body: Body content of the email.
    :type body: str
    :returns: True if email was sent successfully.
    :rtype: bool
    :raises SMTPError: If SMTP connection fails.
    :raises ValueError: If email address is invalid.
    """
    pass

class EmailClient:
    """Client for sending emails via SMTP.

    :param host: SMTP server hostname.
    :type host: str
    :param port: SMTP server port.
    :type port: int
    """

    def connect(self) -> None:
        """Connect to the SMTP server.

        :raises ConnectionError: If connection fails.
        """
        pass
`
	err := os.WriteFile(pyFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewPythonParser(tmpDir)
	result, err := parser.ParseFile(pyFile)
	require.NoError(t, err)

	// Check Sphinx-style docstring parsing
	for _, sym := range result.Symbols {
		switch sym.Name {
		case "send_email":
			assert.Contains(t, sym.DocComment, "Send an email", "Should have description")
			assert.Contains(t, sym.DocComment, "Params:", "Should have params section")
			assert.Contains(t, sym.DocComment, "to: str", "Should have to param with type")
			assert.Contains(t, sym.DocComment, "subject: str", "Should have subject param with type")
			assert.Contains(t, sym.DocComment, "Returns:", "Should have returns section")
			assert.Contains(t, sym.DocComment, "Raises:", "Should have raises section")
		case "EmailClient":
			assert.Contains(t, sym.DocComment, "Client for sending emails", "Should have class description")
		}
	}
}

func TestPythonParser_AsyncFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "async_funcs.py")

	content := `import asyncio

async def fetch_data(url: str) -> dict:
    """Fetch data from URL asynchronously."""
    pass

async def process_batch(items: List[str]) -> List[Result]:
    """Process a batch of items concurrently."""
    pass

class AsyncService:
    """Async service class."""

    async def initialize(self) -> None:
        """Initialize the async service."""
        pass

    async def shutdown(self) -> None:
        """Shutdown the async service."""
        pass
`
	err := os.WriteFile(pyFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewPythonParser(tmpDir)
	result, err := parser.ParseFile(pyFile)
	require.NoError(t, err)

	symbolNames := make(map[string]bool)
	for _, sym := range result.Symbols {
		symbolNames[sym.Name] = true
	}

	// Verify async functions are extracted
	assert.True(t, symbolNames["fetch_data"], "Should extract async function fetch_data")
	assert.True(t, symbolNames["process_batch"], "Should extract async function process_batch")
	assert.True(t, symbolNames["AsyncService"], "Should extract AsyncService class")
	assert.True(t, symbolNames["initialize"], "Should extract async method initialize")
	assert.True(t, symbolNames["shutdown"], "Should extract async method shutdown")
}
