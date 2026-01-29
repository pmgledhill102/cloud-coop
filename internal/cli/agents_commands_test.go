package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/testutil"
)

func TestRunAgentsList_VMNotRunning(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusStopped)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAgentsList(cmd, nil)
	if err != nil {
		t.Errorf("runAgentsList() unexpected error: %v", err)
	}
}

func TestRunAgentsList_NoSessions(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider()
	sshMock := testutil.NewMockSSHClient()
	sshMock.ExpectCommand("tmux list-windows *").Return("", errNoServer())
	cleanup := withFullMocks(cfg, mock, sshMock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAgentsList(cmd, nil)
	if err != nil {
		t.Errorf("runAgentsList() unexpected error: %v", err)
	}
}

func TestRunAgentsList_WithSessions(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider()
	sshMock := testutil.NewMockSSHClient()
	sshMock.ExpectCommand("tmux list-windows *").Return("0|main|claude\n1|dev|bash\n", nil)
	cleanup := withFullMocks(cfg, mock, sshMock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAgentsList(cmd, nil)
	if err != nil {
		t.Errorf("runAgentsList() unexpected error: %v", err)
	}
}

func TestRunAgentsAdd_VMNotFound(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusNotFound)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAgentsAdd(cmd, nil)
	if err != nil {
		t.Errorf("runAgentsAdd() unexpected error: %v", err)
	}
}

func TestRunAgentsAdd_CreatesSession(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider()
	sshMock := testutil.NewMockSSHClient()
	// First call: ListSessions (no session exists)
	sshMock.ExpectCommand("tmux list-windows *").Return("", errNoServer())
	// Second call: CreateSession (list sessions check inside CreateSession)
	sshMock.ExpectCommand("tmux list-windows *").Return("", errNoServer())
	// Third call: new-session
	sshMock.ExpectCommand("tmux new-session *").Return("", nil)
	cleanup := withFullMocks(cfg, mock, sshMock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAgentsAdd(cmd, nil)
	if err != nil {
		t.Errorf("runAgentsAdd() unexpected error: %v", err)
	}
}

func TestRunAgentsKill_VMNotRunning(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusStopped)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAgentsKill(cmd, []string{"0"})
	if err != nil {
		t.Errorf("runAgentsKill() unexpected error: %v", err)
	}
}

func TestRunAgentsKill_InvalidIndex(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAgentsKill(cmd, []string{"abc"})
	if err == nil {
		t.Error("expected error for invalid index")
	}
}

func TestRunAgentsKill_NoSession(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider()
	sshMock := testutil.NewMockSSHClient()
	// KillSession calls ListSessions first
	sshMock.ExpectCommand("tmux list-windows *").Return("", errNoServer())
	cleanup := withFullMocks(cfg, mock, sshMock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAgentsKill(cmd, []string{"0"})
	if err != nil {
		t.Errorf("runAgentsKill() unexpected error: %v", err)
	}
}

func TestRunAgentsAttach_RequiresFlag(t *testing.T) {
	// Reset flags
	attachNext = false
	attachWindow = ""

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAgentsAttach(cmd, nil)
	if err == nil {
		t.Error("expected error when neither --next nor --window is set")
	}
}

func TestRunConnect_InvalidIndex(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runConnect(cmd, []string{"xyz"})
	if err == nil {
		t.Error("expected error for invalid window index")
	}
}

// errNoServer returns an error simulating no tmux server running.
func errNoServer() error {
	return &tmuxError{msg: "no server running on /tmp/tmux-1000/default"}
}

type tmuxError struct{ msg string }

func (e *tmuxError) Error() string { return e.msg }
