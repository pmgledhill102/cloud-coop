package agent

import (
	"errors"
	"testing"
)

// mockRunner implements ssh.Runner for testing.
type mockRunner struct {
	output string
	err    error
}

func (m *mockRunner) Run(cmd string) (string, error) {
	return m.output, m.err
}

func (m *mockRunner) Close() error {
	return nil
}

func TestListSessions(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		err           error
		wantSessions  []Session
		wantNoSession bool
		wantErr       error
	}{
		{
			name:   "multiple sessions",
			output: "0|agent-1|claude\n1|agent-2|aider\n2|agent-3|bash\n",
			wantSessions: []Session{
				{Index: 0, Name: "agent-1", Command: "claude"},
				{Index: 1, Name: "agent-2", Command: "aider"},
				{Index: 2, Name: "agent-3", Command: "bash"},
			},
		},
		{
			name:   "single session",
			output: "0|main|claude\n",
			wantSessions: []Session{
				{Index: 0, Name: "main", Command: "claude"},
			},
		},
		{
			name:         "empty output",
			output:       "",
			wantSessions: nil,
		},
		{
			name:         "whitespace only",
			output:       "  \n  \n  ",
			wantSessions: nil,
		},
		{
			name:          "no session exists",
			output:        "can't find session: agents",
			err:           errors.New("exit status 1"),
			wantNoSession: true,
		},
		{
			name:          "session not found variant",
			output:        "session not found: agents",
			err:           errors.New("exit status 1"),
			wantNoSession: true,
		},
		{
			name:          "no server running",
			output:        "no server running on /tmp/tmux-1000/default",
			err:           errors.New("exit status 1"),
			wantNoSession: true,
		},
		{
			name:          "error connecting to socket",
			output:        "error connecting to /tmp/tmux-1001/default (No such file or directory)",
			err:           errors.New("exit status 1"),
			wantNoSession: true,
		},
		{
			name:    "tmux not installed - command not found",
			output:  "bash: tmux: command not found",
			err:     errors.New("exit status 127"),
			wantErr: ErrTmuxNotInstalled,
		},
		{
			name:    "tmux not installed - exit 127",
			output:  "",
			err:     errors.New("exit status 127"),
			wantErr: ErrTmuxNotInstalled,
		},
		{
			name:   "handles trailing newlines",
			output: "0|agent-1|claude\n\n",
			wantSessions: []Session{
				{Index: 0, Name: "agent-1", Command: "claude"},
			},
		},
		{
			name:   "handles window index gaps",
			output: "0|agent-1|claude\n5|agent-5|bash\n",
			wantSessions: []Session{
				{Index: 0, Name: "agent-1", Command: "claude"},
				{Index: 5, Name: "agent-5", Command: "bash"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{output: tt.output, err: tt.err}
			result, err := ListSessions(runner)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ListSessions() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ListSessions() unexpected error: %v", err)
				return
			}

			if result.NoSession != tt.wantNoSession {
				t.Errorf("ListSessions() NoSession = %v, want %v", result.NoSession, tt.wantNoSession)
			}

			if len(result.Sessions) != len(tt.wantSessions) {
				t.Errorf("ListSessions() got %d sessions, want %d", len(result.Sessions), len(tt.wantSessions))
				return
			}

			for i, got := range result.Sessions {
				want := tt.wantSessions[i]
				if got.Index != want.Index || got.Name != want.Name || got.Command != want.Command {
					t.Errorf("Session[%d] = %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

func TestParseTmuxOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []Session
	}{
		{
			name:   "standard format",
			output: "0|agent-1|claude",
			want:   []Session{{Index: 0, Name: "agent-1", Command: "claude"}},
		},
		{
			name:   "command with spaces",
			output: "0|build|npm run dev",
			want:   []Session{{Index: 0, Name: "build", Command: "npm run dev"}},
		},
		{
			name:   "malformed line - too few parts",
			output: "0|agent-1",
			want:   nil,
		},
		{
			name:   "malformed line - non-numeric index",
			output: "abc|agent-1|claude",
			want:   nil,
		},
		{
			name: "mixed valid and invalid",
			output: `0|agent-1|claude
invalid
2|agent-2|bash`,
			want: []Session{
				{Index: 0, Name: "agent-1", Command: "claude"},
				{Index: 2, Name: "agent-2", Command: "bash"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTmuxOutput(tt.output)
			if len(got) != len(tt.want) {
				t.Errorf("parseTmuxOutput() got %d sessions, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Session[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// sequenceMockRunner is a mock runner that returns different results for each call.
type sequenceMockRunner struct {
	calls    []mockCall
	callIdx  int
	commands []string // records commands executed
}

type mockCall struct {
	output string
	err    error
}

func (m *sequenceMockRunner) Run(cmd string) (string, error) {
	m.commands = append(m.commands, cmd)
	if m.callIdx >= len(m.calls) {
		return "", errors.New("unexpected call")
	}
	call := m.calls[m.callIdx]
	m.callIdx++
	return call.output, call.err
}

func (m *sequenceMockRunner) Close() error {
	return nil
}

func TestCreateSession(t *testing.T) {
	tests := []struct {
		name        string
		opts        CreateSessionOptions
		calls       []mockCall
		wantSession *Session
		wantErr     error
		wantCmdPart string // partial command to check
	}{
		{
			name: "create first session (no existing session)",
			opts: CreateSessionOptions{},
			calls: []mockCall{
				{output: "can't find session: agents", err: errors.New("exit status 1")}, // list shows no session
				{output: "", err: nil}, // create session succeeds
			},
			wantSession: &Session{Index: 0, Name: "", Command: "bash"},
			wantCmdPart: "tmux new-session -d -s agents",
		},
		{
			name: "create window in existing session",
			opts: CreateSessionOptions{Name: "test-agent", Command: "claude"},
			calls: []mockCall{
				{output: "0|agent-0|bash\n", err: nil}, // list shows one window
				{output: "", err: nil},                 // create window succeeds
			},
			wantSession: &Session{Index: 1, Name: "test-agent", Command: "claude"},
			wantCmdPart: "tmux new-window -t agents",
		},
		{
			name: "create with custom command",
			opts: CreateSessionOptions{Command: "claude --dangerously-skip-permissions"},
			calls: []mockCall{
				{output: "can't find session: agents", err: errors.New("exit status 1")},
				{output: "", err: nil},
			},
			wantSession: &Session{Index: 0, Name: "", Command: "claude --dangerously-skip-permissions"},
		},
		{
			name: "tmux not installed",
			opts: CreateSessionOptions{},
			calls: []mockCall{
				{output: "bash: tmux: command not found", err: errors.New("exit status 127")},
			},
			wantErr: ErrTmuxNotInstalled,
		},
		{
			name: "auto-generate name for new session",
			opts: CreateSessionOptions{},
			calls: []mockCall{
				{output: "can't find session: agents", err: errors.New("exit status 1")},
				{output: "", err: nil},
			},
			wantSession: &Session{Index: 0, Name: "", Command: "bash"},
			wantCmdPart: "'agent-0'",
		},
		{
			name: "auto-generate name based on existing windows",
			opts: CreateSessionOptions{},
			calls: []mockCall{
				{output: "0|agent-0|bash\n1|agent-1|claude\n", err: nil},
				{output: "", err: nil},
			},
			wantSession: &Session{Index: 2, Name: "", Command: "bash"},
			wantCmdPart: "'agent-2'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &sequenceMockRunner{calls: tt.calls}
			session, err := CreateSession(runner, tt.opts)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("CreateSession() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateSession() unexpected error: %v", err)
				return
			}

			if session.Index != tt.wantSession.Index {
				t.Errorf("CreateSession() Index = %d, want %d", session.Index, tt.wantSession.Index)
			}

			if session.Command != tt.wantSession.Command {
				t.Errorf("CreateSession() Command = %q, want %q", session.Command, tt.wantSession.Command)
			}

			// Check that the expected command was executed
			if tt.wantCmdPart != "" {
				found := false
				for _, cmd := range runner.commands {
					if contains(cmd, tt.wantCmdPart) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("CreateSession() expected command containing %q, got %v", tt.wantCmdPart, runner.commands)
				}
			}
		})
	}
}

func TestKillSession(t *testing.T) {
	tests := []struct {
		name    string
		opts    KillSessionOptions
		calls   []mockCall
		wantErr error
	}{
		{
			name: "kill idle shell",
			opts: KillSessionOptions{Index: 0, Force: false},
			calls: []mockCall{
				{output: "0|agent-0|bash\n", err: nil}, // list shows bash (idle)
				{output: "", err: nil},                 // kill succeeds
			},
			wantErr: nil,
		},
		{
			name: "refuse to kill active process without force",
			opts: KillSessionOptions{Index: 0, Force: false},
			calls: []mockCall{
				{output: "0|agent-0|claude\n", err: nil}, // list shows claude (active)
			},
			wantErr: ErrActiveProcess,
		},
		{
			name: "force kill active process",
			opts: KillSessionOptions{Index: 0, Force: true},
			calls: []mockCall{
				{output: "0|agent-0|claude\n", err: nil}, // list shows claude (active)
				{output: "", err: nil},                   // kill succeeds
			},
			wantErr: nil,
		},
		{
			name: "window not found",
			opts: KillSessionOptions{Index: 5, Force: false},
			calls: []mockCall{
				{output: "0|agent-0|bash\n", err: nil}, // list shows only index 0
			},
			wantErr: ErrWindowNotFound,
		},
		{
			name: "no session exists",
			opts: KillSessionOptions{Index: 0, Force: false},
			calls: []mockCall{
				{output: "can't find session: agents", err: errors.New("exit status 1")},
			},
			wantErr: ErrNoSession,
		},
		{
			name: "tmux not installed",
			opts: KillSessionOptions{Index: 0, Force: false},
			calls: []mockCall{
				{output: "bash: tmux: command not found", err: errors.New("exit status 127")},
			},
			wantErr: ErrTmuxNotInstalled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &sequenceMockRunner{calls: tt.calls}
			err := KillSession(runner, tt.opts)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("KillSession() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("KillSession() unexpected error: %v", err)
			}
		})
	}
}

func TestIsActiveProcess(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"bash", false},
		{"sh", false},
		{"zsh", false},
		{"fish", false},
		{"BASH", false}, // case insensitive
		{"Zsh", false},
		{"claude", true},
		{"aider", true},
		{"npm run dev", true},
		{"python script.py", true},
		{"  bash  ", false}, // with whitespace
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := isActiveProcess(tt.command)
			if got != tt.want {
				t.Errorf("isActiveProcess(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "'simple'"},
		{"with spaces", "'with spaces'"},
		{"with'quote", "'with'\"'\"'quote'"},
		{"", "''"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellEscape(tt.input)
			if got != tt.want {
				t.Errorf("shellEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := itoa(tt.n)
			if got != tt.want {
				t.Errorf("itoa(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

// contains checks if s contains substr (simple implementation).
func contains(s, substr string) bool {
	return len(substr) <= len(s) && containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
