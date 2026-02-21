package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleBootstrap_InvalidPath(t *testing.T) {
	base := t.TempDir()
	s := &Server{cwd: base}

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"traversal escape", "/../../../etc/passwd", http.StatusBadRequest},
		{"absolute outside base", "/tmp/evil", http.StatusBadRequest},
		{"nonexistent inside base", base + "/nonexistent", http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(BootstrapRequest{ProjectPath: tc.path})
			req := httptest.NewRequest(http.MethodPost, "/api/bootstrap", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			s.handleBootstrap(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("got status %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestHandleBootstrap_EmptyPathDefaultsToCwd(t *testing.T) {
	base := t.TempDir()
	s := &Server{cwd: base}

	// Empty body → defaults to cwd, which exists, so should pass path validation.
	// Will fail later (no LLM config) but should NOT return 400 for path.
	body, _ := json.Marshal(BootstrapRequest{})
	req := httptest.NewRequest(http.MethodPost, "/api/bootstrap", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleBootstrap(rec, req)
	// Should not be 400 (path is valid); will be 500 (LLM config missing) which is fine.
	if rec.Code == http.StatusBadRequest {
		t.Errorf("empty path should default to cwd, got 400: %s", rec.Body.String())
	}
}
