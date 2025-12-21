package agents

import (
	"testing"
)

func TestReactCodeAgent_parseFindings(t *testing.T) {
	agent := &ReactCodeAgent{}

	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{
			name: "valid JSON with decisions",
			input: `Here is my analysis:
{"decisions": [{"title": "Test", "what": "desc", "why": "reason", "tradeoffs": "trade", "confidence": "high", "evidence": ["file.go"]}]}`,
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "JSON with multiple decisions",
			input: `{"decisions": [
				{"title": "D1", "what": "w1", "why": "r1", "tradeoffs": "t1", "confidence": "high", "evidence": []},
				{"title": "D2", "what": "w2", "why": "r2", "tradeoffs": "t2", "confidence": "low", "evidence": ["f1", "f2"]}
			]}`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "no JSON in response",
			input:   "This is just plain text without any JSON",
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "empty decisions array",
			input:   `{"decisions": []}`,
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "malformed JSON",
			input:   `{"decisions": [{"title": "broken`,
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := agent.parseFindings(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFindings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(findings) != tt.wantLen {
				t.Errorf("parseFindings() got %d findings, want %d", len(findings), tt.wantLen)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a longer string", 10, "this is a ..."},
		{"with\nnewlines\nhere", 20, "with newlines here"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
