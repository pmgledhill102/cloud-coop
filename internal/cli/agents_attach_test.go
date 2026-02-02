package cli

import (
	"strings"
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/agent"
)

func TestFindWindow(t *testing.T) {
	sessions := []agent.Session{
		{Index: 0, Name: "main", Command: "claude"},
		{Index: 1, Name: "feature-auth", Command: "aider"},
		{Index: 2, Name: "fix-bug-42", Command: "bash"},
	}

	tests := []struct {
		name        string
		sessions    []agent.Session
		nameOrIndex string
		wantIndex   int
		wantName    string
		wantErr     bool
		errContains string
	}{
		{
			name:        "find by valid index",
			sessions:    sessions,
			nameOrIndex: "1",
			wantIndex:   1,
			wantName:    "feature-auth",
		},
		{
			name:        "find by invalid index",
			sessions:    sessions,
			nameOrIndex: "5",
			wantErr:     true,
			errContains: "no window at index 5",
		},
		{
			name:        "find by valid name",
			sessions:    sessions,
			nameOrIndex: "fix-bug-42",
			wantIndex:   2,
			wantName:    "fix-bug-42",
		},
		{
			name:        "find by invalid name",
			sessions:    sessions,
			nameOrIndex: "nonexistent",
			wantErr:     true,
			errContains: "no window named",
		},
		{
			name:        "index 0 boundary",
			sessions:    sessions,
			nameOrIndex: "0",
			wantIndex:   0,
			wantName:    "main",
		},
		{
			name:        "empty sessions returns error for index",
			sessions:    []agent.Session{},
			nameOrIndex: "0",
			wantErr:     true,
			errContains: "no window at index",
		},
		{
			name:        "empty sessions returns error for name",
			sessions:    []agent.Session{},
			nameOrIndex: "main",
			wantErr:     true,
			errContains: "no window named",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findWindow(tt.sessions, tt.nameOrIndex)
			if tt.wantErr {
				if err == nil {
					t.Errorf("findWindow() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("findWindow() error = %v, want containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("findWindow() unexpected error: %v", err)
				return
			}
			if got.Index != tt.wantIndex {
				t.Errorf("findWindow() index = %d, want %d", got.Index, tt.wantIndex)
			}
			if got.Name != tt.wantName {
				t.Errorf("findWindow() name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}
