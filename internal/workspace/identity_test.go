package workspace

import (
	"errors"
	"testing"
)

func TestLocalGitIdentity(t *testing.T) {
	tests := []struct {
		name    string
		results map[string]mockResult
		wantID  GitIdentity
		wantOK  bool
	}{
		{
			name: "both set",
			results: map[string]mockResult{
				"config user.name":  {output: "Alice"},
				"config user.email": {output: "alice@example.com"},
			},
			wantID: GitIdentity{Name: "Alice", Email: "alice@example.com"},
			wantOK: true,
		},
		{
			name: "name error",
			results: map[string]mockResult{
				"config user.name":  {err: errors.New("exit status 1")},
				"config user.email": {output: "alice@example.com"},
			},
			wantOK: false,
		},
		{
			name: "email error",
			results: map[string]mockResult{
				"config user.name":  {output: "Alice"},
				"config user.email": {err: errors.New("exit status 1")},
			},
			wantOK: false,
		},
		{
			name: "name empty",
			results: map[string]mockResult{
				"config user.name":  {output: ""},
				"config user.email": {output: "alice@example.com"},
			},
			wantOK: false,
		},
		{
			name: "email empty",
			results: map[string]mockResult{
				"config user.name":  {output: "Alice"},
				"config user.email": {output: ""},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockGitRunner{results: tt.results}
			id, ok := LocalGitIdentity(runner)
			if ok != tt.wantOK {
				t.Errorf("LocalGitIdentity() ok = %v, want %v", ok, tt.wantOK)
			}
			if id != tt.wantID {
				t.Errorf("LocalGitIdentity() = %+v, want %+v", id, tt.wantID)
			}
		})
	}
}

type mockSSHRunner struct {
	commands []string
	output   string
	err      error
}

func (m *mockSSHRunner) Run(cmd string) (string, error) {
	m.commands = append(m.commands, cmd)
	return m.output, m.err
}

func (m *mockSSHRunner) Close() error { return nil }

func TestSetupVMGitIdentity(t *testing.T) {
	t.Run("sets name and email", func(t *testing.T) {
		runner := &mockSSHRunner{}
		err := SetupVMGitIdentity(runner, GitIdentity{Name: "Alice", Email: "alice@example.com"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(runner.commands) != 1 {
			t.Fatalf("expected 1 command, got %d", len(runner.commands))
		}
		cmd := runner.commands[0]
		if want := "git config --global user.name 'Alice' && git config --global user.email 'alice@example.com'"; cmd != want {
			t.Errorf("command = %q, want %q", cmd, want)
		}
	})

	t.Run("escapes special characters", func(t *testing.T) {
		runner := &mockSSHRunner{}
		err := SetupVMGitIdentity(runner, GitIdentity{Name: "O'Brien", Email: "ob@example.com"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cmd := runner.commands[0]
		if want := `git config --global user.name 'O'"'"'Brien' && git config --global user.email 'ob@example.com'`; cmd != want {
			t.Errorf("command = %q, want %q", cmd, want)
		}
	})

	t.Run("returns error on SSH failure", func(t *testing.T) {
		runner := &mockSSHRunner{err: errors.New("connection refused")}
		err := SetupVMGitIdentity(runner, GitIdentity{Name: "Alice", Email: "alice@example.com"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
