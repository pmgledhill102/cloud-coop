package testutil

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestModel provides utilities for testing bubbletea tea.Model implementations.
// It wraps a model and provides methods for sending messages and asserting
// on the rendered view.
type TestModel struct {
	t     testing.TB
	model tea.Model
	view  string
}

// NewTestModel creates a new TestModel wrapper for testing a bubbletea model.
//
// Example:
//
//	func TestMyModel(t *testing.T) {
//	    model := NewMyModel()
//	    tm := testutil.NewTestModel(t, model)
//
//	    tm.Update(tea.KeyMsg{Type: tea.KeyEnter})
//	    tm.AssertViewContains("submitted")
//	}
func NewTestModel(t testing.TB, model tea.Model) *TestModel {
	t.Helper()

	tm := &TestModel{
		t:     t,
		model: model,
	}

	// Call Init and process any initial command
	initCmd := model.Init()
	if initCmd != nil {
		tm.ProcessCmd(initCmd)
	}

	tm.updateView()
	return tm
}

// Model returns the current underlying tea.Model.
func (tm *TestModel) Model() tea.Model {
	return tm.model
}

// View returns the current rendered view as a string.
func (tm *TestModel) View() string {
	return tm.view
}

// Update sends a message to the model and updates the internal state.
// Returns any commands that the model returns.
func (tm *TestModel) Update(msg tea.Msg) tea.Cmd {
	tm.t.Helper()

	var cmd tea.Cmd
	tm.model, cmd = tm.model.Update(msg)
	tm.updateView()
	return cmd
}

// updateView refreshes the cached view string.
func (tm *TestModel) updateView() {
	tm.view = tm.model.View()
}

// SendKey sends a key press message to the model.
func (tm *TestModel) SendKey(key tea.Key) tea.Cmd {
	tm.t.Helper()
	return tm.Update(tea.KeyMsg(key))
}

// SendKeyType sends a key type (like tea.KeyEnter, tea.KeyEsc) to the model.
func (tm *TestModel) SendKeyType(keyType tea.KeyType) tea.Cmd {
	tm.t.Helper()
	return tm.SendKey(tea.Key{Type: keyType})
}

// SendRune sends a single rune (character) key press to the model.
func (tm *TestModel) SendRune(r rune) tea.Cmd {
	tm.t.Helper()
	return tm.SendKey(tea.Key{Type: tea.KeyRunes, Runes: []rune{r}})
}

// SendString sends a string of characters to the model, one at a time.
func (tm *TestModel) SendString(s string) tea.Cmd {
	tm.t.Helper()

	var lastCmd tea.Cmd
	for _, r := range s {
		lastCmd = tm.SendRune(r)
	}
	return lastCmd
}

// SendWindowSize sends a window size message to the model.
func (tm *TestModel) SendWindowSize(width, height int) tea.Cmd {
	tm.t.Helper()
	return tm.Update(tea.WindowSizeMsg{Width: width, Height: height})
}

// AssertViewContains fails the test if the view does not contain the substring.
func (tm *TestModel) AssertViewContains(substr string) {
	tm.t.Helper()

	if !strings.Contains(tm.view, substr) {
		tm.t.Errorf("view does not contain %q\nview:\n%s", substr, tm.view)
	}
}

// AssertViewNotContains fails the test if the view contains the substring.
func (tm *TestModel) AssertViewNotContains(substr string) {
	tm.t.Helper()

	if strings.Contains(tm.view, substr) {
		tm.t.Errorf("view should not contain %q\nview:\n%s", substr, tm.view)
	}
}

// AssertViewEquals fails the test if the view does not exactly match expected.
func (tm *TestModel) AssertViewEquals(expected string) {
	tm.t.Helper()

	if tm.view != expected {
		tm.t.Errorf("view mismatch\nexpected:\n%s\nactual:\n%s", expected, tm.view)
	}
}

// AssertViewMatches fails the test if the view does not match the pattern.
// The pattern uses simple substring matching; for regex, use AssertViewMatchesRegex.
func (tm *TestModel) AssertViewMatches(patterns ...string) {
	tm.t.Helper()

	for _, pattern := range patterns {
		if !strings.Contains(tm.view, pattern) {
			tm.t.Errorf("view does not match pattern %q\nview:\n%s", pattern, tm.view)
		}
	}
}

// AssertViewLines fails the test if the view doesn't have at least n lines.
func (tm *TestModel) AssertViewLines(minLines int) {
	tm.t.Helper()

	lines := strings.Split(tm.view, "\n")
	if len(lines) < minLines {
		tm.t.Errorf("view has %d lines, expected at least %d\nview:\n%s",
			len(lines), minLines, tm.view)
	}
}

// GetViewLines returns the view split into lines.
func (tm *TestModel) GetViewLines() []string {
	return strings.Split(tm.view, "\n")
}

// GetViewLine returns a specific line from the view (0-indexed).
// Returns empty string if the line doesn't exist.
func (tm *TestModel) GetViewLine(index int) string {
	lines := tm.GetViewLines()
	if index < 0 || index >= len(lines) {
		return ""
	}
	return lines[index]
}

// ProcessCmd executes a command and sends its resulting message to the model.
// This is useful for testing async operations.
// Note: This only handles commands that return a single message.
func (tm *TestModel) ProcessCmd(cmd tea.Cmd) tea.Cmd {
	tm.t.Helper()

	if cmd == nil {
		return nil
	}

	msg := cmd()
	if msg == nil {
		return nil
	}

	return tm.Update(msg)
}

// ProcessCmds executes multiple commands in sequence.
func (tm *TestModel) ProcessCmds(cmds ...tea.Cmd) {
	tm.t.Helper()

	for _, cmd := range cmds {
		if cmd != nil {
			tm.ProcessCmd(cmd)
		}
	}
}

// BatchMsgs is a helper for creating batch messages in tests.
func BatchMsgs(msgs ...tea.Msg) []tea.Msg {
	return msgs
}

// Common key types for convenience
var (
	KeyEnter     = tea.Key{Type: tea.KeyEnter}
	KeyEsc       = tea.Key{Type: tea.KeyEscape}
	KeyTab       = tea.Key{Type: tea.KeyTab}
	KeyShiftTab  = tea.Key{Type: tea.KeyShiftTab}
	KeyUp        = tea.Key{Type: tea.KeyUp}
	KeyDown      = tea.Key{Type: tea.KeyDown}
	KeyLeft      = tea.Key{Type: tea.KeyLeft}
	KeyRight     = tea.Key{Type: tea.KeyRight}
	KeyBackspace = tea.Key{Type: tea.KeyBackspace}
	KeyDelete    = tea.Key{Type: tea.KeyDelete}
	KeySpace     = tea.Key{Type: tea.KeySpace}
	KeyCtrlC     = tea.Key{Type: tea.KeyCtrlC}
	KeyCtrlD     = tea.Key{Type: tea.KeyCtrlD}
)

// Key creates a key with the specified rune.
func Key(r rune) tea.Key {
	return tea.Key{Type: tea.KeyRunes, Runes: []rune{r}}
}

// KeyWithAlt creates a key with the Alt modifier.
func KeyWithAlt(r rune) tea.Key {
	return tea.Key{Type: tea.KeyRunes, Runes: []rune{r}, Alt: true}
}
