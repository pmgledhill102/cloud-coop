package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/agent"
)

func TestPrintAgentList(t *testing.T) {
	tests := []struct {
		name           string
		result         *agent.ListResult
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "no session",
			result: &agent.ListResult{
				NoSession: true,
			},
			wantContains: []string{
				"No tmux session",
				"cloudcoop agents add",
			},
		},
		{
			name: "empty session",
			result: &agent.ListResult{
				NoSession: false,
				Sessions:  []agent.Session{},
			},
			wantContains: []string{
				"has no windows",
			},
		},
		{
			name: "multiple agents",
			result: &agent.ListResult{
				NoSession: false,
				Sessions: []agent.Session{
					{Index: 0, Name: "agent-1", Command: "claude"},
					{Index: 1, Name: "agent-2", Command: "aider"},
					{Index: 2, Name: "agent-3", Command: "bash"},
				},
			},
			wantContains: []string{
				"INDEX",
				"NAME",
				"COMMAND",
				"agent-1",
				"claude",
				"agent-2",
				"aider",
				"agent-3",
				"bash",
				"3 agent(s) running",
			},
		},
		{
			name: "single agent",
			result: &agent.ListResult{
				NoSession: false,
				Sessions: []agent.Session{
					{Index: 0, Name: "solo", Command: "claude"},
				},
			},
			wantContains: []string{
				"1 agent(s) running",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printAgentList(tt.result)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("printAgentList() output missing %q\nGot: %s", want, output)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(output, notWant) {
					t.Errorf("printAgentList() output should not contain %q\nGot: %s", notWant, output)
				}
			}
		})
	}
}

func TestParseBaseSessions(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name:   "single base session no group",
			output: "my-repo|",
			want:   []string{"my-repo"},
		},
		{
			name:   "base session with matching group",
			output: "my-repo|my-repo",
			want:   []string{"my-repo"},
		},
		{
			name:   "grouped session filtered out",
			output: "my-repo|my-repo\nmy-repo-abc123|my-repo",
			want:   []string{"my-repo"},
		},
		{
			name:   "multiple base sessions",
			output: "backend|\nfrontend|",
			want:   []string{"backend", "frontend"},
		},
		{
			name:   "mixed base and grouped",
			output: "backend|backend\nbackend-1234|backend\nfrontend|frontend\nfrontend-5678|frontend",
			want:   []string{"backend", "frontend"},
		},
		{
			name:   "empty output",
			output: "",
			want:   nil,
		},
		{
			name:   "whitespace only",
			output: "  \n  ",
			want:   nil,
		},
		{
			name:   "no pipe separator",
			output: "my-repo",
			want:   []string{"my-repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBaseSessions(tt.output)
			if len(got) != len(tt.want) {
				t.Fatalf("parseBaseSessions() returned %d sessions, want %d\ngot: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseBaseSessions()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
