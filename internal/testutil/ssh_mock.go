package testutil

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// SSHClient defines the interface for SSH operations that can be mocked.
// This interface should be implemented by the real SSH client in internal/ssh.
type SSHClient interface {
	// Run executes a command on the remote host and returns the output.
	Run(cmd string) (string, error)

	// Close closes the SSH connection.
	Close() error
}

// CommandExpectation represents an expected SSH command and its response.
type CommandExpectation struct {
	Command string
	Output  string
	Err     error
	called  bool
}

// MockSSHClient provides a mock implementation of SSHClient for testing.
type MockSSHClient struct {
	mu           sync.Mutex
	expectations []*CommandExpectation
	calls        []string
	closed       bool
	anyCommand   bool
	defaultOut   string
	defaultErr   error
}

// NewMockSSHClient creates a new mock SSH client.
func NewMockSSHClient() *MockSSHClient {
	return &MockSSHClient{
		expectations: make([]*CommandExpectation, 0),
		calls:        make([]string, 0),
	}
}

// ExpectCommand sets up an expectation for a specific command.
// Returns the expectation so you can chain .Return() calls.
func (m *MockSSHClient) ExpectCommand(cmd string) *CommandExpectation {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp := &CommandExpectation{Command: cmd}
	m.expectations = append(m.expectations, exp)
	return exp
}

// ExpectAnyCommand configures the mock to accept any command with a default response.
// Useful for tests that don't care about specific commands.
func (m *MockSSHClient) ExpectAnyCommand(output string, err error) *MockSSHClient {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.anyCommand = true
	m.defaultOut = output
	m.defaultErr = err
	return m
}

// Return sets the output and error for a command expectation.
func (e *CommandExpectation) Return(output string, err error) *CommandExpectation {
	e.Output = output
	e.Err = err
	return e
}

// Run implements SSHClient.Run by matching against expectations.
func (m *MockSSHClient) Run(cmd string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, cmd)

	// Look for a matching expectation
	for _, exp := range m.expectations {
		if !exp.called && (exp.Command == cmd || matchesPattern(exp.Command, cmd)) {
			exp.called = true
			return exp.Output, exp.Err
		}
	}

	// If any command is allowed, return default response
	if m.anyCommand {
		return m.defaultOut, m.defaultErr
	}

	// No matching expectation found
	return "", fmt.Errorf("unexpected SSH command: %s", cmd)
}

// Close implements SSHClient.Close.
func (m *MockSSHClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return nil
}

// IsClosed returns whether Close was called.
func (m *MockSSHClient) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.closed
}

// Calls returns the list of commands that were executed.
func (m *MockSSHClient) Calls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]string, len(m.calls))
	copy(result, m.calls)
	return result
}

// AssertExpectations verifies that all expected commands were called.
func (m *MockSSHClient) AssertExpectations(t testing.TB) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, exp := range m.expectations {
		if !exp.called {
			t.Errorf("expected SSH command was not called: %s", exp.Command)
		}
	}
}

// AssertNoUnexpectedCalls verifies that no unexpected commands were called.
func (m *MockSSHClient) AssertNoUnexpectedCalls(t testing.TB) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.calls {
		found := false
		for _, exp := range m.expectations {
			if exp.Command == call || matchesPattern(exp.Command, call) {
				found = true
				break
			}
		}
		if !found && !m.anyCommand {
			t.Errorf("unexpected SSH command was called: %s", call)
		}
	}
}

// Reset clears all expectations and recorded calls.
func (m *MockSSHClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.expectations = make([]*CommandExpectation, 0)
	m.calls = make([]string, 0)
	m.closed = false
	m.anyCommand = false
	m.defaultOut = ""
	m.defaultErr = nil
}

// matchesPattern provides simple pattern matching for command expectations.
// Supports "*" as a wildcard that matches any substring.
func matchesPattern(pattern, actual string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == actual
	}

	// Simple wildcard matching
	parts := strings.Split(pattern, "*")
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(actual[pos:], part)
		if idx == -1 {
			return false
		}
		// First part must be at the start if pattern doesn't start with *
		if i == 0 && !strings.HasPrefix(pattern, "*") && idx != 0 {
			return false
		}
		pos += idx + len(part)
	}
	// Last part must be at the end if pattern doesn't end with *
	if !strings.HasSuffix(pattern, "*") && pos != len(actual) {
		return false
	}
	return true
}
