package terminal

import (
	"strings"
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/agent"
)

func TestIsValidFormat(t *testing.T) {
	tests := []struct {
		format string
		want   bool
	}{
		{"ghostty", true},
		{"iterm2", true},
		{"kitty", true},
		{"GHOSTTY", true}, // case insensitive
		{"ITerm2", true},  // case insensitive
		{"unknown", false},
		{"", false},
		{"terminal", false},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := IsValidFormat(tt.format)
			if got != tt.want {
				t.Errorf("IsValidFormat(%q) = %v, want %v", tt.format, got, tt.want)
			}
		})
	}
}

func TestParseGrid(t *testing.T) {
	tests := []struct {
		spec    string
		want    Grid
		wantErr bool
	}{
		{"4x3", Grid{Cols: 4, Rows: 3}, false},
		{"2x2", Grid{Cols: 2, Rows: 2}, false},
		{"1x1", Grid{Cols: 1, Rows: 1}, false},
		{"3X4", Grid{Cols: 3, Rows: 4}, false},     // case insensitive
		{" 2 x 3 ", Grid{Cols: 2, Rows: 3}, false}, // whitespace
		{"invalid", Grid{}, true},
		{"4x", Grid{}, true},
		{"x3", Grid{}, true},
		{"0x3", Grid{}, true},
		{"4x0", Grid{}, true},
		{"-1x3", Grid{}, true},
		{"4x-1", Grid{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			got, err := ParseGrid(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGrid(%q) error = %v, wantErr %v", tt.spec, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseGrid(%q) = %v, want %v", tt.spec, got, tt.want)
			}
		})
	}
}

func TestCalculateGrid(t *testing.T) {
	tests := []struct {
		count int
		want  Grid
	}{
		{0, Grid{Cols: 1, Rows: 1}},
		{1, Grid{Cols: 1, Rows: 1}},
		{2, Grid{Cols: 2, Rows: 1}},
		{3, Grid{Cols: 2, Rows: 2}},
		{4, Grid{Cols: 2, Rows: 2}},
		{5, Grid{Cols: 3, Rows: 2}},
		{6, Grid{Cols: 3, Rows: 2}},
		{9, Grid{Cols: 3, Rows: 3}},
		{12, Grid{Cols: 4, Rows: 3}},
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.count)), func(t *testing.T) {
			got := CalculateGrid(tt.count)
			// Check that the grid can fit all agents
			if got.Cols*got.Rows < tt.count {
				t.Errorf("CalculateGrid(%d) = %v, cannot fit %d agents", tt.count, got, tt.count)
			}
			// Check that the result is reasonable (not too many empty slots)
			if tt.count > 0 {
				efficiency := float64(tt.count) / float64(got.Cols*got.Rows)
				if efficiency < 0.5 {
					t.Errorf("CalculateGrid(%d) = %v, poor efficiency %.2f", tt.count, got, efficiency)
				}
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	cfg := Config{
		Grid:    Grid{Cols: 2, Rows: 2},
		Host:    "10.0.0.1",
		User:    "testuser",
		Port:    22,
		Session: "agents",
		Sessions: []agent.Session{
			{Index: 0, Name: "agent-0", Command: "claude"},
			{Index: 1, Name: "agent-1", Command: "aider"},
			{Index: 2, Name: "agent-2", Command: "claude"},
		},
	}

	formats := []string{"ghostty", "iterm2", "kitty"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			cfg.Format = format
			output, err := Generate(cfg)
			if err != nil {
				t.Errorf("Generate(%s) error = %v", format, err)
				return
			}
			if output == "" {
				t.Errorf("Generate(%s) returned empty output", format)
			}

			// Check that the output contains the SSH connection info
			if !strings.Contains(output, "10.0.0.1") {
				t.Errorf("Generate(%s) output doesn't contain host", format)
			}
			if !strings.Contains(output, "testuser") {
				t.Errorf("Generate(%s) output doesn't contain user", format)
			}
		})
	}
}

func TestGenerateInvalidFormat(t *testing.T) {
	cfg := Config{
		Format:  "invalid",
		Grid:    Grid{Cols: 2, Rows: 2},
		Host:    "10.0.0.1",
		User:    "testuser",
		Port:    22,
		Session: "agents",
	}

	_, err := Generate(cfg)
	if err == nil {
		t.Error("Generate with invalid format should return error")
	}
}

func TestGenerateGhostty(t *testing.T) {
	cfg := Config{
		Format:  "ghostty",
		Grid:    Grid{Cols: 2, Rows: 1},
		Host:    "192.168.1.100",
		User:    "ubuntu",
		Port:    2222,
		Session: "agents",
		Sessions: []agent.Session{
			{Index: 0, Name: "agent-0", Command: "claude"},
			{Index: 1, Name: "agent-1", Command: "aider"},
		},
	}

	output, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Check shebang
	if !strings.HasPrefix(output, "#!/bin/bash") {
		t.Error("Ghostty output should start with bash shebang")
	}

	// Check SSH command with custom port
	if !strings.Contains(output, "-p 2222") {
		t.Error("Ghostty output should contain custom port")
	}

	// Check for agent names in comments
	if !strings.Contains(output, "agent-0") || !strings.Contains(output, "agent-1") {
		t.Error("Ghostty output should contain agent names")
	}
}

func TestGenerateITerm2(t *testing.T) {
	cfg := Config{
		Format:  "iterm2",
		Grid:    Grid{Cols: 2, Rows: 2},
		Host:    "10.0.0.5",
		User:    "admin",
		Port:    22,
		Session: "agents",
		Sessions: []agent.Session{
			{Index: 0, Name: "agent-0", Command: "claude"},
			{Index: 1, Name: "agent-1", Command: "aider"},
			{Index: 2, Name: "agent-2", Command: "gemini"},
		},
	}

	output, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Check AppleScript structure
	if !strings.Contains(output, "tell application \"iTerm2\"") {
		t.Error("iTerm2 output should contain iTerm2 application reference")
	}

	// Check for window creation
	if !strings.Contains(output, "create window") {
		t.Error("iTerm2 output should contain window creation")
	}

	// Check for split commands
	if !strings.Contains(output, "split") {
		t.Error("iTerm2 output should contain split commands")
	}
}

func TestGenerateKitty(t *testing.T) {
	cfg := Config{
		Format:  "kitty",
		Grid:    Grid{Cols: 3, Rows: 1},
		Host:    "agent-vm.example.com",
		User:    "dev",
		Port:    22,
		Session: "agents",
		Sessions: []agent.Session{
			{Index: 0, Name: "claude-1", Command: "claude"},
			{Index: 1, Name: "claude-2", Command: "claude"},
			{Index: 2, Name: "aider-1", Command: "aider"},
		},
	}

	output, err := Generate(cfg)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Check Kitty session format
	if !strings.Contains(output, "layout grid") {
		t.Error("Kitty output should specify grid layout")
	}

	// Check for launch commands
	if !strings.Contains(output, "launch") {
		t.Error("Kitty output should contain launch commands")
	}

	// Check for tab creation
	if !strings.Contains(output, "new_tab") {
		t.Error("Kitty output should contain tab creation")
	}

	// Check agent titles
	if !strings.Contains(output, "claude-1") || !strings.Contains(output, "aider-1") {
		t.Error("Kitty output should contain agent names as titles")
	}
}
