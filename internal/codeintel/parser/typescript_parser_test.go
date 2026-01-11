package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeScriptParser_ParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "example.ts")

	content := `
/**
 * User represents a user in the system.
 */
export interface User {
  id: string;
  name: string;
  email: string;
}

/**
 * Greeting function returns a greeting message.
 */
export function greet(name: string): string {
  return "Hello, " + name;
}

/**
 * UserService handles user operations.
 */
export class UserService {
  private users: User[] = [];

  /**
   * Adds a user to the service.
   */
  public addUser(user: User): void {
    this.users.push(user);
  }

  public getUser(id: string): User | undefined {
    return this.users.find(u => u.id === id);
  }
}

export const MAX_USERS = 100;

export type UserID = string;

export enum UserRole {
  Admin,
  User,
  Guest,
}

const fetchData = async (url: string): Promise<Response> => {
  return fetch(url);
};
`
	err := os.WriteFile(tsFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewTypeScriptParser(tmpDir)
	result, err := parser.ParseFile(tsFile)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify symbols were extracted
	assert.NotEmpty(t, result.Symbols, "Should extract symbols from TypeScript file")

	// Check for specific symbols
	symbolNames := make(map[string]bool)
	for _, sym := range result.Symbols {
		symbolNames[sym.Name] = true
		// Verify all symbols have the correct language
		assert.Equal(t, "typescript", sym.Language)
	}

	// Check for interface
	assert.True(t, symbolNames["User"], "Should extract User interface")

	// Check for function
	assert.True(t, symbolNames["greet"], "Should extract greet function")

	// Check for class
	assert.True(t, symbolNames["UserService"], "Should extract UserService class")

	// Check for methods
	assert.True(t, symbolNames["addUser"], "Should extract addUser method")
	assert.True(t, symbolNames["getUser"], "Should extract getUser method")

	// Check for constant
	assert.True(t, symbolNames["MAX_USERS"], "Should extract MAX_USERS constant")

	// Check for type alias
	assert.True(t, symbolNames["UserID"], "Should extract UserID type alias")

	// Check for enum
	assert.True(t, symbolNames["UserRole"], "Should extract UserRole enum")

	// Check for arrow function
	assert.True(t, symbolNames["fetchData"], "Should extract fetchData arrow function")
}

func TestTypeScriptParser_SupportedExtensions(t *testing.T) {
	parser := NewTypeScriptParser("/test")

	extensions := parser.SupportedExtensions()
	assert.Contains(t, extensions, ".ts")
	assert.Contains(t, extensions, ".tsx")
	assert.Contains(t, extensions, ".js")
	assert.Contains(t, extensions, ".jsx")
	assert.Contains(t, extensions, ".mjs")
	assert.Contains(t, extensions, ".cjs")
}

func TestTypeScriptParser_CanParse(t *testing.T) {
	parser := NewTypeScriptParser("/test")

	assert.True(t, parser.CanParse("app.ts"))
	assert.True(t, parser.CanParse("app.tsx"))
	assert.True(t, parser.CanParse("app.js"))
	assert.True(t, parser.CanParse("app.jsx"))
	assert.True(t, parser.CanParse("/path/to/file.ts"))
	assert.True(t, parser.CanParse("file.TS")) // Case insensitive
	assert.False(t, parser.CanParse("app.go"))
	assert.False(t, parser.CanParse("app.py"))
	assert.False(t, parser.CanParse("app.rs"))
}

func TestTypeScriptParser_ExtractVisibility(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "visibility.ts")

	content := `
export function publicFunc() {}
function privateFunc() {}

export class PublicClass {}
class PrivateClass {}
`
	err := os.WriteFile(tsFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewTypeScriptParser(tmpDir)
	result, err := parser.ParseFile(tsFile)
	require.NoError(t, err)

	// Find symbols and check visibility
	for _, sym := range result.Symbols {
		switch sym.Name {
		case "publicFunc", "PublicClass":
			assert.Equal(t, "public", sym.Visibility, "%s should be public", sym.Name)
		case "privateFunc", "PrivateClass":
			assert.Equal(t, "private", sym.Visibility, "%s should be private", sym.Name)
		}
	}
}

func TestTypeScriptParser_ParseDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source directory
	srcDir := filepath.Join(tmpDir, "src")
	err := os.MkdirAll(srcDir, 0755)
	require.NoError(t, err)

	// Create TypeScript files
	files := map[string]string{
		"index.ts": "export function main() {}",
		"utils.ts": "export function helper() {}",
		"types.ts": "export interface Config {}",
		"test.ts":  "export function testFunc() {}", // Should be included (not test_)
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(srcDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create node_modules (should be skipped)
	nodeModules := filepath.Join(srcDir, "node_modules")
	err = os.MkdirAll(nodeModules, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(nodeModules, "lib.ts"), []byte("export function lib() {}"), 0644)
	require.NoError(t, err)

	parser := NewTypeScriptParser(srcDir)
	result, err := parser.ParseDirectory(srcDir)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have found symbols from source files but not node_modules
	symbolNames := make(map[string]bool)
	for _, sym := range result.Symbols {
		symbolNames[sym.Name] = true
	}

	assert.True(t, symbolNames["main"], "Should find main function")
	assert.True(t, symbolNames["helper"], "Should find helper function")
	assert.True(t, symbolNames["Config"], "Should find Config interface")
	assert.False(t, symbolNames["lib"], "Should NOT find lib from node_modules")
}

func TestTypeScriptParser_Language(t *testing.T) {
	parser := NewTypeScriptParser("/test")
	assert.Equal(t, "typescript", parser.Language())
}

func TestTypeScriptParserImplementsInterface(t *testing.T) {
	var _ LanguageParser = (*TypeScriptParser)(nil)

	parser := NewTypeScriptParser("/test")
	assert.NotNil(t, parser)
	assert.Equal(t, "typescript", parser.Language())
}

func TestTypeScriptParser_JSDocExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "jsdoc.ts")

	content := `
/**
 * Calculates the sum of two numbers.
 * @param a First number
 * @param b Second number
 * @returns The sum of a and b
 */
export function add(a: number, b: number): number {
  return a + b;
}

/**
 * User interface representing a user in the system.
 */
export interface User {
  id: string;
  name: string;
}

/**
 * Service class for managing users.
 */
export class UserManager {
  /**
   * Creates a new user.
   */
  createUser(name: string): User {
    return { id: "1", name };
  }
}
`
	err := os.WriteFile(tsFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewTypeScriptParser(tmpDir)
	result, err := parser.ParseFile(tsFile)
	require.NoError(t, err)

	// Verify JSDoc comments are extracted
	for _, sym := range result.Symbols {
		switch sym.Name {
		case "add":
			assert.Contains(t, sym.DocComment, "Calculates the sum", "add function should have JSDoc")
		case "User":
			assert.Contains(t, sym.DocComment, "User interface", "User interface should have JSDoc")
		case "UserManager":
			assert.Contains(t, sym.DocComment, "Service class", "UserManager class should have JSDoc")
		case "createUser":
			assert.Contains(t, sym.DocComment, "Creates a new user", "createUser method should have JSDoc")
		}
	}
}

func TestTypeScriptParser_ReactHooks(t *testing.T) {
	tmpDir := t.TempDir()
	tsxFile := filepath.Join(tmpDir, "component.tsx")

	content := `
import React, { useState, useEffect, useMemo, useCallback, useRef } from 'react';

/**
 * Custom hook for fetching data.
 */
export function useFetchData(url: string) {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch(url).then(res => res.json()).then(setData);
  }, [url]);

  return { data, loading };
}

/**
 * Custom hook for managing form state.
 */
export const useFormState = <T>(initialValue: T) => {
  const [value, setValue] = useState(initialValue);
  const reset = useCallback(() => setValue(initialValue), [initialValue]);
  return { value, setValue, reset };
};

export function Counter() {
  const [count, setCount] = useState(0);
  const prevCount = useRef(count);
  const doubleCount = useMemo(() => count * 2, [count]);

  return <div>{count}</div>;
}
`
	err := os.WriteFile(tsxFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewTypeScriptParser(tmpDir)
	result, err := parser.ParseFile(tsxFile)
	require.NoError(t, err)

	// Check custom hooks are detected
	symbolNames := make(map[string]bool)
	symbolDocs := make(map[string]string)
	for _, sym := range result.Symbols {
		symbolNames[sym.Name] = true
		symbolDocs[sym.Name] = sym.DocComment
	}

	// Verify custom hooks are extracted
	assert.True(t, symbolNames["useFetchData"], "Should extract useFetchData custom hook")
	assert.True(t, symbolNames["useFormState"], "Should extract useFormState custom hook")

	// Verify custom hooks are marked as React hooks
	assert.Contains(t, symbolDocs["useFetchData"], "[React Hook]", "useFetchData should be marked as React Hook")
	assert.Contains(t, symbolDocs["useFormState"], "[React Hook]", "useFormState should be marked as React Hook")

	// Check that hook usages are recorded as relations
	assert.NotEmpty(t, result.Relations, "Should have hook usage relations")

	// Verify hook relations have correct metadata
	hookCategories := make(map[string]bool)
	for _, rel := range result.Relations {
		if rel.Metadata != nil {
			if category, ok := rel.Metadata["hookCategory"].(string); ok {
				hookCategories[category] = true
			}
		}
	}

	assert.True(t, hookCategories["state-management"], "Should detect useState hooks")
	assert.True(t, hookCategories["side-effect"], "Should detect useEffect hooks")
	assert.True(t, hookCategories["memoization"], "Should detect useMemo/useCallback hooks")
	assert.True(t, hookCategories["ref"], "Should detect useRef hooks")
}

func TestTypeScriptParser_TSXFile(t *testing.T) {
	tmpDir := t.TempDir()
	tsxFile := filepath.Join(tmpDir, "app.tsx")

	content := `
import React from 'react';

interface ButtonProps {
  label: string;
  onClick: () => void;
}

export const Button: React.FC<ButtonProps> = ({ label, onClick }) => {
  return <button onClick={onClick}>{label}</button>;
};

export class AppComponent extends React.Component {
  render() {
    return <div>Hello World</div>;
  }
}

export function App() {
  return (
    <div>
      <Button label="Click me" onClick={() => alert('clicked')} />
    </div>
  );
}
`
	err := os.WriteFile(tsxFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewTypeScriptParser(tmpDir)
	result, err := parser.ParseFile(tsxFile)
	require.NoError(t, err)

	symbolNames := make(map[string]bool)
	for _, sym := range result.Symbols {
		symbolNames[sym.Name] = true
	}

	// Verify TSX-specific patterns are parsed
	assert.True(t, symbolNames["ButtonProps"], "Should extract ButtonProps interface")
	assert.True(t, symbolNames["Button"], "Should extract Button component")
	assert.True(t, symbolNames["AppComponent"], "Should extract AppComponent class")
	assert.True(t, symbolNames["App"], "Should extract App function component")
}

func TestTypeScriptParser_Decorators(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "decorated.ts")

	content := `import { Component, Injectable, Input } from '@angular/core';

@Injectable({
  providedIn: 'root'
})
export class UserService {
  getUser() { return null; }
}

@Component({
  selector: 'app-user',
  template: '<div>User</div>'
})
export class UserComponent {
  @Input() userId: string;

  constructor(private userService: UserService) {}
}

/**
 * A decorated class with JSDoc.
 */
@Component({
  selector: 'app-root'
})
export class AppComponent {
  title = 'app';
}
`
	err := os.WriteFile(tsFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewTypeScriptParser(tmpDir)
	result, err := parser.ParseFile(tsFile)
	require.NoError(t, err)

	// Find the decorated classes
	var userService, userComponent, appComponent *Symbol
	for i := range result.Symbols {
		switch result.Symbols[i].Name {
		case "UserService":
			userService = &result.Symbols[i]
		case "UserComponent":
			userComponent = &result.Symbols[i]
		case "AppComponent":
			appComponent = &result.Symbols[i]
		}
	}

	// Verify UserService has @Injectable decorator
	require.NotNil(t, userService, "Should find UserService")
	assert.Contains(t, userService.DocComment, "@Injectable", "UserService should have @Injectable decorator")

	// Verify UserComponent has @Component decorator
	require.NotNil(t, userComponent, "Should find UserComponent")
	assert.Contains(t, userComponent.DocComment, "@Component", "UserComponent should have @Component decorator")

	// Verify AppComponent has both decorator and JSDoc
	require.NotNil(t, appComponent, "Should find AppComponent")
	assert.Contains(t, appComponent.DocComment, "@Component", "AppComponent should have @Component decorator")
	assert.Contains(t, appComponent.DocComment, "decorated class with JSDoc", "AppComponent should preserve JSDoc")
}
