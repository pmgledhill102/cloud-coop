// Package agent provides functionality for managing agent sessions via tmux.
package agent

import (
	"errors"
	"strings"

	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// Session represents a single agent running in a tmux window.
type Session struct {
	Index   int    // tmux window index
	Name    string // window name (e.g., "agent-1")
	Command string // current command (e.g., "claude")
}

// ListResult contains the result of listing agent sessions.
type ListResult struct {
	Sessions  []Session // list of active agent sessions
	NoSession bool      // true if no "agents" tmux session exists
}

// ErrTmuxNotInstalled indicates tmux is not available on the remote host.
var ErrTmuxNotInstalled = errors.New("tmux not installed")

// tmuxListCmd is the command to list tmux windows in the agents session.
const tmuxListCmd = "tmux list-windows -t agents -F '#{window_index}|#{window_name}|#{pane_current_command}'"

// ListSessions queries the remote host for active agent sessions.
// It connects via SSH and lists windows in the "agents" tmux session.
func ListSessions(runner ssh.Runner) (*ListResult, error) {
	output, err := runner.Run(tmuxListCmd)
	if err != nil {
		// Check for specific error conditions
		errStr := strings.ToLower(output + err.Error())

		// tmux not installed
		if strings.Contains(errStr, "command not found") ||
			strings.Contains(errStr, "not found") && strings.Contains(errStr, "tmux") ||
			strings.Contains(errStr, "exit status 127") {
			return nil, ErrTmuxNotInstalled
		}

		// No agents session exists (or no tmux server running)
		if strings.Contains(errStr, "session not found") ||
			strings.Contains(errStr, "can't find session") ||
			strings.Contains(errStr, "no server running") ||
			strings.Contains(errStr, "error connecting") ||
			strings.Contains(errStr, "no such file or directory") {
			return &ListResult{NoSession: true}, nil
		}

		// Unknown error
		return nil, err
	}

	sessions := parseTmuxOutput(output)
	return &ListResult{Sessions: sessions}, nil
}

// parseTmuxOutput parses the output of tmux list-windows command.
// Each line is expected to be in format: index|name|command
func parseTmuxOutput(output string) []Session {
	var sessions []Session

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}

		var index int
		if _, err := parseIndex(parts[0], &index); err != nil {
			continue
		}

		sessions = append(sessions, Session{
			Index:   index,
			Name:    parts[1],
			Command: parts[2],
		})
	}

	return sessions
}

// parseIndex parses a string to an integer index.
func parseIndex(s string, index *int) (bool, error) {
	s = strings.TrimSpace(s)
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, errors.New("invalid index")
		}
		n = n*10 + int(c-'0')
	}
	*index = n
	return true, nil
}
