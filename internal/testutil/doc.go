// Package testutil provides common test helpers for the cloudcoop project.
//
// This package includes:
//   - Mock SSH client for testing SSH-based operations without real connections
//   - Test fixtures loader for loading test data from files
//   - Assertion helpers for testing bubbletea TUI components
//
// Example usage:
//
//	func TestSSHOperation(t *testing.T) {
//	    mock := testutil.NewMockSSHClient()
//	    mock.ExpectCommand("ls -la").Return("file1\nfile2", nil)
//
//	    // Use mock in your code
//	    result, err := yourFunction(mock)
//
//	    mock.AssertExpectations(t)
//	}
//
//	func TestTUIModel(t *testing.T) {
//	    model := NewYourModel()
//	    tm := testutil.NewTestModel(t, model)
//
//	    tm.SendKey(tea.KeyEnter)
//	    tm.AssertViewContains("expected text")
//	}
package testutil
