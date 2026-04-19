package mcp

import (
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/memory"
)

func makeTestNodes(n int) []memory.Node {
	types := []string{memory.NodeTypeConstraint, memory.NodeTypeDecision, memory.NodeTypePattern}
	nodes := make([]memory.Node, n)
	for i := range nodes {
		nodes[i] = memory.Node{
			ID:      "n-" + strings.Repeat("0", 3-len(string(rune('0'+i%10)))) + string(rune('0'+i)),
			Type:    types[i%len(types)],
			Summary: "Summary for node " + string(rune('A'+i%26)),
			Content: `{"title":"Full content for node","description":"This is a long description with lots of detail that should only appear in full mode, not summary mode.","snippets":[{"file_path":"foo.go","lines":"1-10"}]}`,
		}
	}
	return nodes
}

func TestFormatKnowledgeSummary_Compact(t *testing.T) {
	nodes := makeTestNodes(100)
	result := FormatKnowledgeSummary(nodes)

	if !strings.Contains(result, "Knowledge Summary (100 nodes)") {
		t.Error("expected header with node count")
	}

	// Summary should NOT contain full content/snippets
	if strings.Contains(result, "snippets") {
		t.Error("summary should not contain snippet data")
	}
	if strings.Contains(result, "file_path") {
		t.Error("summary should not contain file paths from content")
	}

	// Should be much smaller than full dump
	full := FormatKnowledgeFull(nodes)
	if len(result) >= len(full) {
		t.Errorf("summary (%d chars) should be smaller than full (%d chars)", len(result), len(full))
	}
}

func TestFormatKnowledgeSummary_Empty(t *testing.T) {
	result := FormatKnowledgeSummary(nil)
	if !strings.Contains(result, "No knowledge nodes found") {
		t.Error("expected empty message")
	}
}

func TestFormatKnowledgeSummary_GroupsConstraintsFirst(t *testing.T) {
	nodes := []memory.Node{
		{ID: "1", Type: memory.NodeTypeDecision, Summary: "A decision"},
		{ID: "2", Type: memory.NodeTypeConstraint, Summary: "A constraint"},
		{ID: "3", Type: memory.NodeTypePattern, Summary: "A pattern"},
	}
	result := FormatKnowledgeSummary(nodes)
	constraintIdx := strings.Index(result, "Constraint")
	decisionIdx := strings.Index(result, "Decision")
	if constraintIdx == -1 || decisionIdx == -1 {
		t.Fatal("expected both constraint and decision sections")
	}
	if constraintIdx > decisionIdx {
		t.Error("constraints should appear before decisions")
	}
}

func TestFormatKnowledgePage_Basic(t *testing.T) {
	nodes := makeTestNodes(120)

	result := FormatKnowledgePage(nodes, 1, 50)
	if !strings.Contains(result, "Page 1/3") {
		t.Error("expected page 1/3 footer")
	}
	if !strings.Contains(result, "120 total nodes") {
		t.Error("expected total node count")
	}
	if !strings.Contains(result, "page=2") {
		t.Error("expected next page hint")
	}
}

func TestFormatKnowledgePage_LastPage(t *testing.T) {
	nodes := makeTestNodes(120)

	result := FormatKnowledgePage(nodes, 3, 50)
	if !strings.Contains(result, "Page 3/3") {
		t.Error("expected page 3/3 footer")
	}
	if strings.Contains(result, "page=4") {
		t.Error("last page should not have next page hint")
	}
}

func TestFormatKnowledgePage_BeyondRange(t *testing.T) {
	nodes := makeTestNodes(10)

	result := FormatKnowledgePage(nodes, 99, 50)
	if !strings.Contains(result, "Page 1/1") {
		t.Error("page beyond range should clamp to last page")
	}
}

func TestFormatKnowledgePage_ZeroPageSize(t *testing.T) {
	nodes := makeTestNodes(10)

	// Should not panic, defaults to 50
	result := FormatKnowledgePage(nodes, 1, 0)
	if !strings.Contains(result, "Page 1/1") {
		t.Error("pageSize=0 should default to 50, fitting all 10 nodes in 1 page")
	}
}

func TestFormatKnowledgePage_NegativePage(t *testing.T) {
	nodes := makeTestNodes(10)

	result := FormatKnowledgePage(nodes, -1, 50)
	if !strings.Contains(result, "Page 1/1") {
		t.Error("negative page should clamp to 1")
	}
}

func TestFormatKnowledgeFull_PreservedBehavior(t *testing.T) {
	nodes := makeTestNodes(5)
	result := FormatKnowledgeFull(nodes)

	if !strings.Contains(result, "Knowledge (5 nodes)") {
		t.Error("expected header with node count")
	}
	// Full mode should contain content previews
	if !strings.Contains(result, "Full content for node") {
		t.Error("full mode should contain content details")
	}
}
