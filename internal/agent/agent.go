// Package agent provides functionality for managing agent sessions via tmux.
package agent

import (
	"crypto/rand"
	"errors"
	"fmt"
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
	NoSession bool      // true if no tmux session exists
}

// ErrTmuxNotInstalled indicates tmux is not available on the remote host.
var ErrTmuxNotInstalled = errors.New("tmux not installed")

// tmuxListCmd builds the command to list tmux windows in the given session.
func tmuxListCmd(sessionName string) string {
	return "tmux list-windows -t " + shellEscape(sessionName) +
		" -F '#{window_index}|#{window_name}|#{pane_current_command}'"
}

// ListSessions queries the remote host for active agent sessions.
// It connects via SSH and lists windows in the named tmux session.
func ListSessions(runner ssh.Runner, sessionName string) (*ListResult, error) {
	output, err := runner.Run(tmuxListCmd(sessionName))
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

// ErrNoSession indicates the tmux session does not exist.
var ErrNoSession = errors.New("no tmux session exists")

// ErrWindowNotFound indicates the specified tmux window was not found.
var ErrWindowNotFound = errors.New("window not found")

// ErrActiveProcess indicates a window has an active process running.
var ErrActiveProcess = errors.New("window has active process")

// CreateSessionOptions contains options for creating a new agent session.
type CreateSessionOptions struct {
	Name    string // window name (auto-generated if empty)
	Command string // command to run (uses "bash" if empty)
}

// CreateSession creates a new tmux window in the named session.
// If no session exists, it creates the session first.
func CreateSession(runner ssh.Runner, sessionName string, opts CreateSessionOptions) (*Session, error) {
	// Default command to bash if not specified
	command := opts.Command
	if command == "" {
		command = "bash"
	}

	// Check if session exists first
	listResult, err := ListSessions(runner, sessionName)
	if err != nil && !errors.Is(err, ErrTmuxNotInstalled) {
		return nil, err
	}
	if errors.Is(err, ErrTmuxNotInstalled) {
		return nil, err
	}

	var windowIndex int
	if listResult.NoSession {
		// Create new session with the first window
		name := opts.Name
		if name == "" {
			name = "agent-0"
		}
		// Quote the command to handle spaces
		cmd := "tmux new-session -d -s " + shellEscape(sessionName) + " -n " + shellEscape(name) + " " + shellEscape(command)
		_, err := runner.Run(cmd)
		if err != nil {
			return nil, err
		}
		windowIndex = 0
	} else {
		// Find next available window index
		maxIndex := -1
		for _, s := range listResult.Sessions {
			if s.Index > maxIndex {
				maxIndex = s.Index
			}
		}
		windowIndex = maxIndex + 1

		// Generate name if not provided
		name := opts.Name
		if name == "" {
			name = "agent-" + itoa(windowIndex)
		}

		// Create new window in existing session
		cmd := "tmux new-window -t " + shellEscape(sessionName) + " -n " + shellEscape(name) + " " + shellEscape(command)
		_, err := runner.Run(cmd)
		if err != nil {
			return nil, err
		}
	}

	// Return the created session info
	return &Session{
		Index:   windowIndex,
		Name:    opts.Name,
		Command: command,
	}, nil
}

// KillSessionOptions contains options for killing an agent session.
type KillSessionOptions struct {
	Index int  // window index to kill
	Force bool // kill even if there's an active process
}

// KillSession kills a tmux window in the named session.
func KillSession(runner ssh.Runner, sessionName string, opts KillSessionOptions) error {
	// First verify the window exists and check for active process
	listResult, err := ListSessions(runner, sessionName)
	if err != nil {
		return err
	}

	if listResult.NoSession {
		return ErrNoSession
	}

	// Find the session with the given index
	var found *Session
	for _, s := range listResult.Sessions {
		if s.Index == opts.Index {
			found = &s
			break
		}
	}

	if found == nil {
		return ErrWindowNotFound
	}

	// Check if there's an active process (not just a shell)
	if !opts.Force && isActiveProcess(found.Command) {
		return ErrActiveProcess
	}

	// Kill the window
	cmd := "tmux kill-window -t " + shellEscape(sessionName) + ":" + itoa(opts.Index)
	_, err = runner.Run(cmd)
	if err != nil {
		// Check if the error is because the window doesn't exist
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "can't find") || strings.Contains(errStr, "not found") {
			return ErrWindowNotFound
		}
		return err
	}

	return nil
}

// isActiveProcess returns true if the command indicates an active process
// (not just an idle shell).
func isActiveProcess(command string) bool {
	// Common shell commands that indicate idle state
	idleCommands := []string{"bash", "sh", "zsh", "fish", "ksh", "csh", "tcsh", "dash"}
	cmdLower := strings.ToLower(strings.TrimSpace(command))
	for _, idle := range idleCommands {
		if cmdLower == idle {
			return false
		}
	}
	return true
}

// shellEscape escapes a string for safe use in shell commands.
func shellEscape(s string) string {
	// Use single quotes and escape any single quotes in the string
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// itoa converts an integer to a string (simple implementation to avoid fmt import).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// ErrAllWindowsAttached indicates every window in the session already has an attached client.
var ErrAllWindowsAttached = errors.New("all windows have attached clients")

// ClientInfo describes a tmux client attached to a session.
type ClientInfo struct {
	TTY         string
	Session     string
	WindowIndex int
	WindowName  string
}

// ListClients queries the remote host for clients attached to a tmux session.
func ListClients(runner ssh.Runner, sessionName string) ([]ClientInfo, error) {
	cmd := "tmux list-clients -t " + shellEscape(sessionName) +
		" -F '#{client_tty}|#{client_session}|#{window_index}|#{window_name}'"
	output, err := runner.Run(cmd)
	if err != nil {
		errStr := strings.ToLower(output + err.Error())
		if strings.Contains(errStr, "no clients") ||
			strings.Contains(errStr, "session not found") ||
			strings.Contains(errStr, "can't find session") ||
			strings.Contains(errStr, "no server running") {
			return nil, nil
		}
		return nil, err
	}
	return parseTmuxClients(output), nil
}

// parseTmuxClients parses the output of tmux list-clients.
// Each line is expected to be in format: tty|session|windowIndex|windowName
func parseTmuxClients(output string) []ClientInfo {
	var clients []ClientInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		var idx int
		if _, err := parseIndex(parts[2], &idx); err != nil {
			continue
		}
		clients = append(clients, ClientInfo{
			TTY:         parts[0],
			Session:     parts[1],
			WindowIndex: idx,
			WindowName:  parts[3],
		})
	}
	return clients
}

// FindNextWindow returns the first session window that has no attached client.
func FindNextWindow(sessions []Session, clients []ClientInfo) (Session, error) {
	attached := make(map[int]bool)
	for _, c := range clients {
		attached[c.WindowIndex] = true
	}
	for _, s := range sessions {
		if !attached[s.Index] {
			return s, nil
		}
	}
	return Session{}, ErrAllWindowsAttached
}

// CreateGroupedSession creates a new tmux session grouped with the base session,
// and selects the given window index. Returns the grouped session name.
func CreateGroupedSession(runner ssh.Runner, slug string, windowIndex int) (string, error) {
	suffix, err := randomHex(4)
	if err != nil {
		return "", fmt.Errorf("generate session suffix: %w", err)
	}
	grouped := slug + "-" + suffix

	// Create grouped session
	cmd := "tmux new-session -d -t " + shellEscape(slug) + " -s " + shellEscape(grouped)
	_, err = runner.Run(cmd)
	if err != nil {
		return "", fmt.Errorf("create grouped session: %w", err)
	}

	// Select the target window
	selectCmd := "tmux select-window -t " + shellEscape(grouped) + ":" + itoa(windowIndex)
	_, err = runner.Run(selectCmd)
	if err != nil {
		return "", fmt.Errorf("select window: %w", err)
	}

	return grouped, nil
}

// randomHex returns n random hex characters.
func randomHex(n int) (string, error) {
	b := make([]byte, (n+1)/2)
	_, err := cryptoRandRead(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b)[:n], nil
}

// cryptoRandRead is a variable for testability.
var cryptoRandRead = cryptoRandReadImpl

func cryptoRandReadImpl(b []byte) (int, error) {
	return rand.Read(b)
}
