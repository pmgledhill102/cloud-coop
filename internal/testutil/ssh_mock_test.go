package testutil

import (
	"errors"
	"testing"
)

func TestMockSSHClient_Run_WithExpectation(t *testing.T) {
	mock := NewMockSSHClient()
	mock.ExpectCommand("ls -la").Return("file1\nfile2", nil)

	output, err := mock.Run("ls -la")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if output != "file1\nfile2" {
		t.Errorf("unexpected output: %s", output)
	}

	mock.AssertExpectations(t)
}

func TestMockSSHClient_Run_WithError(t *testing.T) {
	mock := NewMockSSHClient()
	expectedErr := errors.New("connection refused")
	mock.ExpectCommand("ssh-test").Return("", expectedErr)

	_, err := mock.Run("ssh-test")
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestMockSSHClient_Run_UnexpectedCommand(t *testing.T) {
	mock := NewMockSSHClient()
	mock.ExpectCommand("expected-cmd").Return("output", nil)

	_, err := mock.Run("unexpected-cmd")
	if err == nil {
		t.Error("expected error for unexpected command, got nil")
	}
}

func TestMockSSHClient_Run_AnyCommand(t *testing.T) {
	mock := NewMockSSHClient()
	mock.ExpectAnyCommand("default output", nil)

	output, err := mock.Run("any command")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if output != "default output" {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestMockSSHClient_Calls(t *testing.T) {
	mock := NewMockSSHClient()
	mock.ExpectAnyCommand("", nil)

	mock.Run("cmd1")
	mock.Run("cmd2")
	mock.Run("cmd3")

	calls := mock.Calls()
	if len(calls) != 3 {
		t.Errorf("expected 3 calls, got %d", len(calls))
	}
	if calls[0] != "cmd1" || calls[1] != "cmd2" || calls[2] != "cmd3" {
		t.Errorf("unexpected calls: %v", calls)
	}
}

func TestMockSSHClient_Close(t *testing.T) {
	mock := NewMockSSHClient()

	if mock.IsClosed() {
		t.Error("expected IsClosed to be false initially")
	}

	err := mock.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mock.IsClosed() {
		t.Error("expected IsClosed to be true after Close")
	}
}

func TestMockSSHClient_Reset(t *testing.T) {
	mock := NewMockSSHClient()
	mock.ExpectCommand("cmd").Return("out", nil)
	mock.Run("cmd")
	mock.Close()

	mock.Reset()

	if len(mock.Calls()) != 0 {
		t.Error("expected calls to be empty after reset")
	}
	if mock.IsClosed() {
		t.Error("expected IsClosed to be false after reset")
	}
}

func TestMockSSHClient_WildcardPattern(t *testing.T) {
	mock := NewMockSSHClient()
	mock.ExpectCommand("tmux *").Return("session output", nil)

	output, err := mock.Run("tmux list-sessions")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if output != "session output" {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		pattern string
		actual  string
		want    bool
	}{
		{"exact", "exact", true},
		{"exact", "notexact", false},
		{"*", "anything", true},
		{"prefix*", "prefix-suffix", true},
		{"prefix*", "notprefix", false},
		{"*suffix", "prefix-suffix", true},
		{"*suffix", "suffixnot", false},
		{"pre*suf", "pre-middle-suf", true},
		{"pre*suf", "pre-middle-not", false},
		{"*mid*", "prefix-mid-suffix", true},
		{"tmux *", "tmux list-windows", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.actual, func(t *testing.T) {
			got := matchesPattern(tt.pattern, tt.actual)
			if got != tt.want {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v",
					tt.pattern, tt.actual, got, tt.want)
			}
		})
	}
}
