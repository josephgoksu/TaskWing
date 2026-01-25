package util

import (
	"context"
	"errors"
	"testing"
)

func TestShortID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		n    int
		want string
	}{
		{
			name: "default length truncates",
			id:   "task-abcdef12",
			n:    0,
			want: "task-abc",
		},
		{
			name: "negative uses default",
			id:   "task-abcdef12",
			n:    -1,
			want: "task-abc",
		},
		{
			name: "explicit length 10",
			id:   "task-abcdef12",
			n:    10,
			want: "task-abcde",
		},
		{
			name: "length equals ID",
			id:   "task-abc",
			n:    8,
			want: "task-abc",
		},
		{
			name: "length longer than ID",
			id:   "task-abc",
			n:    20,
			want: "task-abc",
		},
		{
			name: "plan ID",
			id:   "plan-xyz12345",
			n:    8,
			want: "plan-xyz",
		},
		{
			name: "empty ID",
			id:   "",
			n:    8,
			want: "",
		},
		{
			name: "very short",
			id:   "ab",
			n:    8,
			want: "ab",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ShortID(tc.id, tc.n)
			if got != tc.want {
				t.Errorf("ShortID(%q, %d) = %q, want %q", tc.id, tc.n, got, tc.want)
			}
		})
	}
}

// mockResolver implements IDPrefixResolver for testing.
type mockResolver struct {
	taskIDs []string
	planIDs []string
	err     error
}

func (m *mockResolver) FindTaskIDsByPrefix(_ context.Context, prefix string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	var matches []string
	for _, id := range m.taskIDs {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			matches = append(matches, id)
		}
	}
	return matches, nil
}

func (m *mockResolver) FindPlanIDsByPrefix(_ context.Context, prefix string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	var matches []string
	for _, id := range m.planIDs {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			matches = append(matches, id)
		}
	}
	return matches, nil
}

func TestResolveTaskID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		resolver   *mockResolver
		idOrPrefix string
		want       string
		wantErr    error
	}{
		{
			name: "full ID exact match",
			resolver: &mockResolver{
				taskIDs: []string{"task-abcdef12", "task-xyz12345"},
			},
			idOrPrefix: "task-abcdef12",
			want:       "task-abcdef12",
		},
		{
			name: "prefix matches one",
			resolver: &mockResolver{
				taskIDs: []string{"task-abcdef12", "task-xyz12345"},
			},
			idOrPrefix: "task-abc",
			want:       "task-abcdef12",
		},
		{
			name: "prefix without task- prepended",
			resolver: &mockResolver{
				taskIDs: []string{"task-abcdef12", "task-xyz12345"},
			},
			idOrPrefix: "abc",
			want:       "task-abcdef12",
		},
		{
			name: "prefix matches multiple - ambiguous",
			resolver: &mockResolver{
				taskIDs: []string{"task-abc11111", "task-abc22222", "task-abc33333"},
			},
			idOrPrefix: "task-abc",
			wantErr:    ErrAmbiguousID,
		},
		{
			name: "prefix matches none - not found",
			resolver: &mockResolver{
				taskIDs: []string{"task-abcdef12"},
			},
			idOrPrefix: "task-xyz",
			wantErr:    ErrNotFound,
		},
		{
			name:       "empty ID",
			resolver:   &mockResolver{},
			idOrPrefix: "",
			wantErr:    ErrNotFound,
		},
		{
			name: "resolver error",
			resolver: &mockResolver{
				err: errors.New("database error"),
			},
			idOrPrefix: "task-abc",
			wantErr:    errors.New("database error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveTaskID(ctx, tc.resolver, tc.idOrPrefix)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error containing %v, got nil", tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) && !containsError(err, tc.wantErr) {
					t.Errorf("error = %v, want %v", err, tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("ResolveTaskID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolvePlanID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		resolver   *mockResolver
		idOrPrefix string
		want       string
		wantErr    error
	}{
		{
			name: "full ID exact match",
			resolver: &mockResolver{
				planIDs: []string{"plan-abcdef12", "plan-xyz12345"},
			},
			idOrPrefix: "plan-abcdef12",
			want:       "plan-abcdef12",
		},
		{
			name: "prefix matches one",
			resolver: &mockResolver{
				planIDs: []string{"plan-abcdef12", "plan-xyz12345"},
			},
			idOrPrefix: "plan-abc",
			want:       "plan-abcdef12",
		},
		{
			name: "prefix without plan- prepended",
			resolver: &mockResolver{
				planIDs: []string{"plan-abcdef12", "plan-xyz12345"},
			},
			idOrPrefix: "abc",
			want:       "plan-abcdef12",
		},
		{
			name: "prefix matches multiple - ambiguous",
			resolver: &mockResolver{
				planIDs: []string{"plan-abc11111", "plan-abc22222"},
			},
			idOrPrefix: "plan-abc",
			wantErr:    ErrAmbiguousID,
		},
		{
			name: "prefix matches none - not found",
			resolver: &mockResolver{
				planIDs: []string{"plan-abcdef12"},
			},
			idOrPrefix: "plan-xyz",
			wantErr:    ErrNotFound,
		},
		{
			name:       "empty ID",
			resolver:   &mockResolver{},
			idOrPrefix: "",
			wantErr:    ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolvePlanID(ctx, tc.resolver, tc.idOrPrefix)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error containing %v, got nil", tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) && !containsError(err, tc.wantErr) {
					t.Errorf("error = %v, want %v", err, tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("ResolvePlanID() = %q, want %q", got, tc.want)
			}
		})
	}
}

// containsError checks if err contains the target error message.
func containsError(err, target error) bool {
	if err == nil || target == nil {
		return false
	}
	return err.Error() == target.Error() ||
		len(err.Error()) > len(target.Error()) &&
			err.Error()[len(err.Error())-len(target.Error()):] == target.Error()
}

func TestAmbiguousErrorMessage(t *testing.T) {
	ctx := context.Background()
	resolver := &mockResolver{
		taskIDs: []string{
			"task-aaa11111",
			"task-aaa22222",
			"task-aaa33333",
			"task-aaa44444",
			"task-aaa55555",
			"task-aaa66666", // 6th one, should be truncated
		},
	}

	_, err := ResolveTaskID(ctx, resolver, "task-aaa")
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, ErrAmbiguousID) {
		t.Errorf("expected ErrAmbiguousID, got: %v", err)
	}

	// Should mention 6 matches
	errStr := err.Error()
	if !contains(errStr, "6 tasks") {
		t.Errorf("error should mention 6 matches: %s", errStr)
	}

	// Should only show first 5 candidates (MaxAmbiguousCandidates)
	if contains(errStr, "task-aaa66666") {
		t.Errorf("error should not show 6th candidate: %s", errStr)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
