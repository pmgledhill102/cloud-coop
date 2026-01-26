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
