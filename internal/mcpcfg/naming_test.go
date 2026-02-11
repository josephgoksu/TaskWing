package mcpcfg

import "testing"

func TestServerNameClassification(t *testing.T) {
	tests := []struct {
		name       string
		serverName string
		canonical  bool
		legacy     bool
	}{
		{name: "canonical", serverName: "taskwing-mcp", canonical: true, legacy: false},
		{name: "legacy bare", serverName: "taskwing", canonical: false, legacy: true},
		{name: "legacy suffixed", serverName: "taskwing-mcp-my-project", canonical: false, legacy: true},
		{name: "non taskwing", serverName: "other-mcp", canonical: false, legacy: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCanonicalServerName(tt.serverName); got != tt.canonical {
				t.Fatalf("IsCanonicalServerName(%q) = %v, want %v", tt.serverName, got, tt.canonical)
			}
			if got := IsLegacyServerName(tt.serverName); got != tt.legacy {
				t.Fatalf("IsLegacyServerName(%q) = %v, want %v", tt.serverName, got, tt.legacy)
			}
		})
	}
}

func TestExtractTaskWingServerNames(t *testing.T) {
	output := `
taskwing-mcp: /usr/local/bin/taskwing mcp
taskwing-mcp-my-project: /usr/local/bin/taskwing mcp
other-mcp: /usr/local/bin/other mcp
`

	names := ExtractTaskWingServerNames(output)
	if len(names) == 0 {
		t.Fatal("expected extracted names")
	}

	if !ContainsCanonicalServerName(output) {
		t.Fatal("expected canonical server name detection")
	}
	if !ContainsLegacyServerName(output) {
		t.Fatal("expected legacy server name detection")
	}
}
