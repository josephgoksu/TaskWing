package parser

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTypeScriptParser_EdgeCases is a QA audit test for edge cases
func TestTypeScriptParser_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "edge_case.ts")

	content := `// Edge case 1: Nested parentheses in function parameters
export function handleCallback(callback: (x: number, y: (z: string) => void) => void): void {
  callback(1, (s) => console.log(s));
}

// Edge case 2: Multi-line function signature
export function multiLine(
  param1: string,
  param2: number
): Promise<void> {
  return Promise.resolve();
}

// Edge case 3: Generic with constraints
export function withGenerics<T extends object, K extends keyof T>(obj: T, key: K): T[K] {
  return obj[key];
}

// Edge case 4: String containing function-like syntax
const fakeFunction = "function notReal() {}";
const template = ` + "`" + `class FakeClass { }` + "`" + `;

// Edge case 5: Arrow function returning object literal
const getObject = () => ({ name: "test" });

// Edge case 6: Destructured parameters
export function destructured({ name, age }: { name: string; age: number }): string {
  return name;
}

// Edge case 7: Class with complex generics
export class Repository<T extends Entity, ID extends string | number = string> implements IRepository<T> {
  findById(id: ID): T | undefined { return undefined; }
}

// Edge case 8: Commented out code
// export function commented() {}
/* export class CommentedClass {} */

// Edge case 9: Overloaded function signatures
export function overloaded(x: string): string;
export function overloaded(x: number): number;
export function overloaded(x: string | number): string | number {
  return x;
}
`
	err := os.WriteFile(tsFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewTypeScriptParser(tmpDir)
	result, err := parser.ParseFile(tsFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Expected functions that SHOULD be found
	expectedFunctions := []string{
		"handleCallback",
		"multiLine",
		"withGenerics",
		"destructured",
		"overloaded",
	}

	// Functions that should NOT be found (in comments/strings)
	unexpectedFunctions := []string{
		"notReal",
		"commented",
	}

	// Classes that should be found
	expectedClasses := []string{
		"Repository",
	}

	// Classes that should NOT be found
	unexpectedClasses := []string{
		"FakeClass",
		"CommentedClass",
	}

	// Build symbol name set
	symbolNames := make(map[string]bool)
	for _, sym := range result.Symbols {
		symbolNames[sym.Name] = true
		t.Logf("Found: %s (%s) sig=%s", sym.Name, sym.Kind, sym.Signature)
	}

	// Check expected functions
	for _, fn := range expectedFunctions {
		if !symbolNames[fn] {
			t.Errorf("MISSING: Expected function %q was not extracted", fn)
		}
	}

	// Check unexpected functions (false positives)
	for _, fn := range unexpectedFunctions {
		if symbolNames[fn] {
			t.Errorf("FALSE POSITIVE: Function %q was extracted from comment/string", fn)
		}
	}

	// Check expected classes
	for _, cls := range expectedClasses {
		if !symbolNames[cls] {
			t.Errorf("MISSING: Expected class %q was not extracted", cls)
		}
	}

	// Check unexpected classes
	for _, cls := range unexpectedClasses {
		if symbolNames[cls] {
			t.Errorf("FALSE POSITIVE: Class %q was extracted from comment/string", cls)
		}
	}
}

// TestPythonParser_EdgeCases is a QA audit test for Python edge cases
func TestPythonParser_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "edge_case.py")

	content := `# Edge case 1: Decorator chain
@decorator1
@decorator2(arg)
def decorated_func():
    pass

# Edge case 2: Async context manager
async def async_context():
    async with something:
        pass

# Edge case 3: Multi-line function signature
def multiline_params(
    param1: str,
    param2: int,
    param3: Optional[str] = None
) -> Dict[str, Any]:
    pass

# Edge case 4: String containing def
fake_def = "def not_a_function(): pass"
triple_quote = """
def also_not_a_function():
    pass
"""

# Edge case 5: Nested class
class Outer:
    class Inner:
        def inner_method(self):
            pass

    def outer_method(self):
        pass

# Edge case 6: Lambda (should not be extracted as function)
my_lambda = lambda x: x * 2

# Edge case 7: Commented code
# def commented_func():
#     pass

# Edge case 8: Property decorator
class WithProperty:
    @property
    def my_property(self) -> str:
        return "value"

    @my_property.setter
    def my_property(self, value: str):
        self._value = value
`
	err := os.WriteFile(pyFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewPythonParser(tmpDir)
	result, err := parser.ParseFile(pyFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Build symbol set
	symbolNames := make(map[string]bool)
	for _, sym := range result.Symbols {
		symbolNames[sym.Name] = true
		t.Logf("Found: %s (%s)", sym.Name, sym.Kind)
	}

	// Expected
	expected := []string{"decorated_func", "async_context", "multiline_params", "Outer", "WithProperty"}
	for _, name := range expected {
		if !symbolNames[name] {
			t.Errorf("MISSING: Expected %q was not extracted", name)
		}
	}

	// False positives
	unexpected := []string{"not_a_function", "also_not_a_function", "commented_func"}
	for _, name := range unexpected {
		if symbolNames[name] {
			t.Errorf("FALSE POSITIVE: %q was extracted from comment/string", name)
		}
	}
}

// TestRustParser_EdgeCases is a QA audit test for Rust edge cases
func TestRustParser_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	rsFile := filepath.Join(tmpDir, "edge_case.rs")

	content := `// Edge case 1: Macro-generated code
macro_rules! define_func {
    ($name:ident) => {
        fn $name() {}
    };
}

// Edge case 2: Attribute macros
#[derive(Debug, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Attributed {
    pub field: String,
}

// Edge case 3: Generic lifetime bounds
pub fn with_lifetime<'a, 'b: 'a, T: Clone + 'a>(x: &'a T, y: &'b T) -> &'a T {
    x
}

// Edge case 4: Raw string containing fn
let raw = r#"fn not_a_function() {}"#;
let normal = "fn also_not() {}";

// Edge case 5: Async trait method
pub trait AsyncTrait {
    async fn async_method(&self) -> Result<(), Error>;
}

// Edge case 6: Const generics
pub struct Array<T, const N: usize> {
    data: [T; N],
}

// Edge case 7: Where clause spanning multiple lines
pub fn complex_where<T, U>(x: T, y: U) -> T
where
    T: Clone + Send + Sync,
    U: Into<T>,
{
    x
}

// Edge case 8: Commented code
// fn commented() {}
/* fn block_commented() {} */

// Edge case 9: Unsafe fn
pub unsafe fn unsafe_function(ptr: *mut i32) {
    *ptr = 42;
}
`
	err := os.WriteFile(rsFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewRustParser(tmpDir)
	result, err := parser.ParseFile(rsFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Build symbol set
	symbolNames := make(map[string]bool)
	for _, sym := range result.Symbols {
		symbolNames[sym.Name] = true
		t.Logf("Found: %s (%s)", sym.Name, sym.Kind)
	}

	// Expected
	expected := []string{"Attributed", "with_lifetime", "AsyncTrait", "Array", "complex_where", "unsafe_function"}
	for _, name := range expected {
		if !symbolNames[name] {
			t.Errorf("MISSING: Expected %q was not extracted", name)
		}
	}

	// False positives
	unexpected := []string{"not_a_function", "also_not", "commented", "block_commented"}
	for _, name := range unexpected {
		if symbolNames[name] {
			t.Errorf("FALSE POSITIVE: %q was extracted from comment/string", name)
		}
	}
}
