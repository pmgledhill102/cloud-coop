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
				"No agents session",
				"tmux new-session",
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
