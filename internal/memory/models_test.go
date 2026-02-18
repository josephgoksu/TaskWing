package memory

import (
	"encoding/json"
	"testing"
)

func TestNode_Text_PlainText(t *testing.T) {
	n := Node{Content: "some plain text content"}
	if got := n.Text(); got != "some plain text content" {
		t.Errorf("Text() = %q, want %q", got, "some plain text content")
	}
}

func TestNode_Text_EmptyContent(t *testing.T) {
	n := Node{Content: ""}
	if got := n.Text(); got != "" {
		t.Errorf("Text() = %q, want empty string", got)
	}
}

func TestNode_Text_StructuredContent(t *testing.T) {
	sc := StructuredContent{
		Title:       "SQLite as primary store",
		Description: "Chose SQLite for local-first persistence",
		Why:         "Embedded, zero-config, good enough perf",
		Tradeoffs:   "No concurrent writes",
	}
	content, _ := json.Marshal(sc)
	n := Node{Content: string(content)}

	got := n.Text()
	want := "SQLite as primary store\nChose SQLite for local-first persistence\n\nWhy: Embedded, zero-config, good enough perf\nTradeoffs: No concurrent writes"
	if got != want {
		t.Errorf("Text() =\n%q\nwant\n%q", got, want)
	}
}

func TestNode_Text_StructuredWithSnippets(t *testing.T) {
	sc := StructuredContent{
		Title:       "Repository pattern",
		Description: "All data access goes through Repository interface",
		Snippets: []EvidenceSnippet{
			{FilePath: "internal/memory/store.go", Lines: "10-25", Code: "type Repository interface{...}"},
		},
	}
	content, _ := json.Marshal(sc)
	n := Node{Content: string(content)}

	got := n.Text()
	if got == "" {
		t.Fatal("Text() returned empty string for structured content with snippets")
	}
	// Should contain the evidence section
	if !contains(got, "Evidence:") {
		t.Error("Text() missing Evidence section")
	}
	if !contains(got, "internal/memory/store.go:10-25") {
		t.Error("Text() missing file:lines reference")
	}
}

func TestNode_Text_InvalidJSON(t *testing.T) {
	n := Node{Content: "{invalid json}"}
	if got := n.Text(); got != "{invalid json}" {
		t.Errorf("Text() = %q, want passthrough for invalid JSON", got)
	}
}

func TestNode_Text_JSONWithoutTitle(t *testing.T) {
	// Valid JSON but not a StructuredContent (no title)
	n := Node{Content: `{"description":"no title here"}`}
	if got := n.Text(); got != `{"description":"no title here"}` {
		t.Errorf("Text() should return as-is when Title is empty, got %q", got)
	}
}

func TestNode_ParseStructuredContent_Roundtrip(t *testing.T) {
	original := StructuredContent{
		Title:       "Test title",
		Description: "Test desc",
		Why:         "Test why",
		Tradeoffs:   "Test tradeoffs",
		Snippets: []EvidenceSnippet{
			{FilePath: "foo.go", Lines: "1-10", Code: "func Foo() {}"},
		},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	n := Node{Content: string(data)}
	sc := n.ParseStructuredContent()
	if sc == nil {
		t.Fatal("ParseStructuredContent() returned nil")
	}
	if sc.Title != original.Title {
		t.Errorf("Title = %q, want %q", sc.Title, original.Title)
	}
	if sc.Description != original.Description {
		t.Errorf("Description = %q, want %q", sc.Description, original.Description)
	}
	if sc.Why != original.Why {
		t.Errorf("Why = %q, want %q", sc.Why, original.Why)
	}
	if sc.Tradeoffs != original.Tradeoffs {
		t.Errorf("Tradeoffs = %q, want %q", sc.Tradeoffs, original.Tradeoffs)
	}
	if len(sc.Snippets) != 1 {
		t.Fatalf("Snippets len = %d, want 1", len(sc.Snippets))
	}
	if sc.Snippets[0].Code != "func Foo() {}" {
		t.Errorf("Snippet code = %q", sc.Snippets[0].Code)
	}
}

func TestNode_ParseStructuredContent_PlainText(t *testing.T) {
	n := Node{Content: "just some text"}
	if sc := n.ParseStructuredContent(); sc != nil {
		t.Errorf("ParseStructuredContent() = %+v, want nil for plain text", sc)
	}
}

func TestNode_ParseStructuredContent_Empty(t *testing.T) {
	n := Node{Content: ""}
	if sc := n.ParseStructuredContent(); sc != nil {
		t.Errorf("ParseStructuredContent() = %+v, want nil for empty", sc)
	}
}

func TestNode_Text_MinimalStructured(t *testing.T) {
	// Only title + description, no optional fields
	sc := StructuredContent{
		Title:       "Minimal",
		Description: "Just the basics",
	}
	content, _ := json.Marshal(sc)
	n := Node{Content: string(content)}

	got := n.Text()
	want := "Minimal\nJust the basics"
	if got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
