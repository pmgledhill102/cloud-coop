package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/testutil"
)

func TestRunTerminalGenerate_InvalidFormat(t *testing.T) {
	// Set the format flag to an invalid value
	origFormat := terminalFormat
	terminalFormat = "invalid"
	defer func() { terminalFormat = origFormat }()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runTerminalGenerate(cmd, nil)
	if err == nil {
		t.Error("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("error = %v, want containing 'invalid format'", err)
	}
}

func TestRunTerminalGenerate_VMNotFound(t *testing.T) {
	origFormat := terminalFormat
	terminalFormat = "ghostty"
	defer func() { terminalFormat = origFormat }()

	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusNotFound)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runTerminalGenerate(cmd, nil)
	if err != nil {
		t.Errorf("runTerminalGenerate() unexpected error: %v", err)
	}
}

func TestRunTerminalGenerate_VMStopped(t *testing.T) {
	origFormat := terminalFormat
	terminalFormat = "ghostty"
	defer func() { terminalFormat = origFormat }()

	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusStopped)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runTerminalGenerate(cmd, nil)
	if err != nil {
		t.Errorf("runTerminalGenerate() unexpected error: %v", err)
	}
}

func TestRunTerminalGenerate_TmuxNotInstalled(t *testing.T) {
	origFormat := terminalFormat
	terminalFormat = "ghostty"
	defer func() { terminalFormat = origFormat }()

	cfg := testConfig()
	mock := cloud.NewMockProvider()
	sshMock := testutil.NewMockSSHClient()
	sshMock.ExpectCommand("tmux list-windows *").Return("tmux: command not found", &tmuxError{msg: "tmux: command not found"})
	cleanup := withFullMocks(cfg, mock, sshMock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runTerminalGenerate(cmd, nil)
	if err != nil {
		t.Errorf("runTerminalGenerate() unexpected error: %v", err)
	}
}

func TestRunTerminalGenerate_NoSessions(t *testing.T) {
	origFormat := terminalFormat
	terminalFormat = "ghostty"
	defer func() { terminalFormat = origFormat }()

	cfg := testConfig()
	mock := cloud.NewMockProvider()
	sshMock := testutil.NewMockSSHClient()
	sshMock.ExpectCommand("tmux list-windows *").Return("", errNoServer())
	cleanup := withFullMocks(cfg, mock, sshMock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runTerminalGenerate(cmd, nil)
	if err != nil {
		t.Errorf("runTerminalGenerate() unexpected error: %v", err)
	}
}

func TestRunTerminalGenerate_HappyPath(t *testing.T) {
	origFormat := terminalFormat
	origOutput := terminalOutput
	origGrid := terminalGrid
	terminalFormat = "ghostty"
	terminalOutput = "" // stdout
	terminalGrid = ""
	defer func() {
		terminalFormat = origFormat
		terminalOutput = origOutput
		terminalGrid = origGrid
	}()

	cfg := testConfig()
	mock := cloud.NewMockProvider()
	sshMock := testutil.NewMockSSHClient()
	sshMock.ExpectCommand("tmux list-windows *").Return("0|main|claude\n1|dev|bash\n", nil)
	cleanup := withFullMocks(cfg, mock, sshMock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runTerminalGenerate(cmd, nil)
	if err != nil {
		t.Errorf("runTerminalGenerate() unexpected error: %v", err)
	}
}
