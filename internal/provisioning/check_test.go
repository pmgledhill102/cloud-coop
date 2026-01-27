package provisioning

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// mockRunner implements ssh.Runner for testing.
type mockRunner struct {
	responses map[string]struct {
		output string
		err    error
	}
}

func (m *mockRunner) Run(cmd string) (string, error) {
	for pattern, resp := range m.responses {
		if strings.Contains(cmd, pattern) {
			return resp.output, resp.err
		}
	}
	return "", fmt.Errorf("unexpected command: %s", cmd)
}

func (m *mockRunner) Close() error {
	return nil
}

func TestCheckStatus(t *testing.T) {
	tests := []struct {
		name         string
		statusOutput string
		statusErr    error
		progressOut  string
		progressErr  error
		wantStatus   ProvisionStatus
		wantProgress string
		wantErr      bool
	}{
		{
			name:         "completed status",
			statusOutput: "completed",
			wantStatus:   StatusCompleted,
		},
		{
			name:         "running with progress",
			statusOutput: "running",
			progressOut:  "5/34 Installing dependencies",
			wantStatus:   StatusRunning,
			wantProgress: "5/34 Installing dependencies",
		},
		{
			name:         "pending (file not found)",
			statusOutput: "pending",
			wantStatus:   StatusPending,
		},
		{
			name:      "ssh error",
			statusErr: errors.New("connection refused"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRunner{
				responses: map[string]struct {
					output string
					err    error
				}{
					StatusFilePath: {
						output: tt.statusOutput,
						err:    tt.statusErr,
					},
					ProgressFilePath: {
						output: tt.progressOut,
						err:    tt.progressErr,
					},
				},
			}

			info, err := CheckStatus(mock)

			if tt.wantErr {
				if err == nil {
					t.Error("CheckStatus() expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("CheckStatus() unexpected error: %v", err)
			}

			if info.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", info.Status, tt.wantStatus)
			}
			if info.Progress != tt.wantProgress {
				t.Errorf("Progress = %q, want %q", info.Progress, tt.wantProgress)
			}
		})
	}
}

func TestStripShebang(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "with shebang",
			input:  "#!/bin/bash\necho hello\necho world",
			expect: "echo hello\necho world",
		},
		{
			name:   "without shebang",
			input:  "echo hello\necho world",
			expect: "echo hello\necho world",
		},
		{
			name:   "empty string",
			input:  "",
			expect: "",
		},
		{
			name:   "only shebang",
			input:  "#!/bin/bash",
			expect: "",
		},
		{
			name:   "different interpreter",
			input:  "#!/usr/bin/env python3\nprint('hello')",
			expect: "print('hello')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripShebang(tt.input)
			if got != tt.expect {
				t.Errorf("StripShebang() = %q, want %q", got, tt.expect)
			}
		})
	}
}
