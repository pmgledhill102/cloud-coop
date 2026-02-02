package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
	"github.com/cloud-coop/cloudcoop/internal/workspace"
)

var errTestConfig = errors.New("test config error")

func TestCanModifyAgents(t *testing.T) {
	completedStatus := &provisioning.StatusInfo{Status: provisioning.StatusCompleted}

	tests := []struct {
		name string
		m    Model
		want bool
	}{
		{
			name: "can modify when VM running and no operation",
			m: Model{
				cfg:             &config.Config{},
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
				provisionStatus: completedStatus,
			},
			want: true,
		},
		{
			name: "cannot modify when VM stopped",
			m: Model{
				cfg:             &config.Config{},
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusStopped},
				provisionStatus: completedStatus,
			},
			want: false,
		},
		{
			name: "cannot modify when config error",
			m: Model{
				cfg:             &config.Config{},
				cfgErr:          errTestConfig,
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
				provisionStatus: completedStatus,
			},
			want: false,
		},
		{
			name: "cannot modify during operation",
			m: Model{
				cfg:             &config.Config{},
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
				operation:       "adding",
				provisionStatus: completedStatus,
			},
			want: false,
		},
		{
			name: "cannot modify while agents loading",
			m: Model{
				cfg:             &config.Config{},
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
				agentsLoading:   true,
				provisionStatus: completedStatus,
			},
			want: false,
		},
		{
			name: "cannot modify while provisioning loading",
			m: Model{
				cfg:              &config.Config{},
				vmInfo:           &cloud.VMInfo{Status: cloud.VMStatusRunning},
				provisionLoading: true,
				provisionStatus:  completedStatus,
			},
			want: false,
		},
		{
			name: "cannot modify when provisioning not complete",
			m: Model{
				cfg:             &config.Config{},
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
				provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusRunning},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.canModifyAgents()
			if got != tt.want {
				t.Errorf("canModifyAgents() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyPlus_AddAgent(t *testing.T) {
	// Setup model in state where adding is allowed
	m := Model{
		cfg:             &config.Config{},
		vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents:          &agent.ListResult{Sessions: []agent.Session{}},
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	// Simulate pressing '+'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should trigger adding operation
	if updated.operation != "adding" {
		t.Errorf("pressing + should set operation to 'adding', got %q", updated.operation)
	}

	// Should return a command (the addAgent command)
	if cmd == nil {
		t.Error("pressing + should return a command")
	}
}

func TestKeyPlus_NotAllowedWhenVMStopped(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusStopped},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should NOT trigger adding operation
	if updated.operation == "adding" {
		t.Error("pressing + when VM stopped should not trigger add")
	}
	if cmd != nil {
		t.Error("pressing + when VM stopped should not return a command")
	}
}

func TestKeyMinus_KillConfirmation(t *testing.T) {
	// Setup model with agents to kill
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
				{Index: 1, Name: "agent-1", Command: "claude"},
			},
		},
		selectedAgentIdx: 1, // Select agent-1
		provisionStatus:  &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	// Simulate pressing '-'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	// Should enter confirmation mode
	if !updated.confirmingKill {
		t.Error("pressing - should enter confirmation mode")
	}
	if updated.killTargetIndex != 1 {
		t.Errorf("kill target should be index 1, got %d", updated.killTargetIndex)
	}
	if updated.killTargetName != "agent-1" {
		t.Errorf("kill target name should be 'agent-1', got %q", updated.killTargetName)
	}
}

func TestKeyMinus_NotAllowedWithNoAgents(t *testing.T) {
	m := Model{
		cfg:             &config.Config{},
		vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents:          &agent.ListResult{Sessions: []agent.Session{}}, // No agents
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	// Should NOT enter confirmation mode
	if updated.confirmingKill {
		t.Error("pressing - with no agents should not enter confirmation mode")
	}
}

func TestKillConfirmation_Y(t *testing.T) {
	m := Model{
		cfg:             &config.Config{},
		vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		confirmingKill:  true,
		killTargetIndex: 1,
		killTargetName:  "agent-1",
	}

	// Simulate pressing 'Y' to confirm
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should exit confirmation mode and start killing
	if updated.confirmingKill {
		t.Error("pressing Y should exit confirmation mode")
	}
	if updated.operation != "killing" {
		t.Errorf("pressing Y should set operation to 'killing', got %q", updated.operation)
	}
	if cmd == nil {
		t.Error("pressing Y should return a command")
	}
}

func TestKillConfirmation_N(t *testing.T) {
	m := Model{
		cfg:             &config.Config{},
		vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
		confirmingKill:  true,
		killTargetIndex: 1,
	}

	// Simulate pressing 'N' to cancel
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should exit confirmation mode without killing
	if updated.confirmingKill {
		t.Error("pressing N should exit confirmation mode")
	}
	if updated.operation == "killing" {
		t.Error("pressing N should not start killing")
	}
	if cmd != nil {
		t.Error("pressing N should not return a command")
	}
}

func TestKillConfirmation_Escape(t *testing.T) {
	m := Model{
		cfg:             &config.Config{},
		vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
		confirmingKill:  true,
		killTargetIndex: 1,
	}

	// Simulate pressing Escape to cancel
	msg := tea.KeyMsg{Type: tea.KeyEscape}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should exit confirmation mode without killing
	if updated.confirmingKill {
		t.Error("pressing Escape should exit confirmation mode")
	}
	if cmd != nil {
		t.Error("pressing Escape should not return a command")
	}
}

func TestArrowKeys_AgentSelection(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
				{Index: 1, Name: "agent-1", Command: "claude"},
				{Index: 2, Name: "agent-2", Command: "aider"},
			},
		},
		selectedAgentIdx: 0,
	}

	// Press down arrow
	msg := tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	if updated.selectedAgentIdx != 1 {
		t.Errorf("down arrow should select index 1, got %d", updated.selectedAgentIdx)
	}

	// Press down again
	msg = tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ = updated.Update(msg)
	updated = newModel.(Model)

	if updated.selectedAgentIdx != 2 {
		t.Errorf("down arrow should select index 2, got %d", updated.selectedAgentIdx)
	}

	// Press down at bottom (should stay at 2)
	msg = tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ = updated.Update(msg)
	updated = newModel.(Model)

	if updated.selectedAgentIdx != 2 {
		t.Errorf("down arrow at bottom should stay at 2, got %d", updated.selectedAgentIdx)
	}

	// Press up arrow
	msg = tea.KeyMsg{Type: tea.KeyUp}
	newModel, _ = updated.Update(msg)
	updated = newModel.(Model)

	if updated.selectedAgentIdx != 1 {
		t.Errorf("up arrow should select index 1, got %d", updated.selectedAgentIdx)
	}
}

func TestAgentAddedMsg_RefreshesAgents(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		vmInfo:    &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		operation: "adding",
	}

	// Simulate successful add message
	msg := agentAddedMsg{session: &agent.Session{Index: 0, Name: "agent-0", Command: "bash"}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Operation should be cleared
	if updated.operation != "" {
		t.Errorf("operation should be cleared, got %q", updated.operation)
	}

	// Should start loading agents
	if !updated.agentsLoading {
		t.Error("should start loading agents after add")
	}

	// Should return a command to fetch agents
	if cmd == nil {
		t.Error("should return command to refresh agents")
	}
}

func TestAgentKilledMsg_RefreshesAgents(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		vmInfo:    &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		operation: "killing",
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
			},
		},
	}

	// Simulate successful kill message
	msg := agentKilledMsg{index: 0}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Operation should be cleared
	if updated.operation != "" {
		t.Errorf("operation should be cleared, got %q", updated.operation)
	}

	// Should start loading agents
	if !updated.agentsLoading {
		t.Error("should start loading agents after kill")
	}

	// Should return a command to fetch agents
	if cmd == nil {
		t.Error("should return command to refresh agents")
	}
}

func TestRenderHelp_ShowsAgentActions(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
			},
		},
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	help := m.renderHelp()

	// Should include agent actions with new labels
	if !containsString(help, "+: add agent") {
		t.Error("help should show '+: add agent'")
	}
	if !containsString(help, "-: kill agent") {
		t.Error("help should show '-: kill agent'")
	}
}

func TestRenderHelp_ConfirmationMode(t *testing.T) {
	m := Model{
		confirmingKill: true,
	}

	help := m.renderHelp()

	// Should show confirmation options only (lowercase)
	if !containsString(help, "y: confirm") {
		t.Error("help in confirmation mode should show 'y: confirm'")
	}
	if !containsString(help, "n: cancel") {
		t.Error("help in confirmation mode should show 'n: cancel'")
	}
}

func TestKey_LowercaseC_Connect(t *testing.T) {
	// Setup model with agents to connect to
	m := Model{
		cfg:    &config.Config{SSH: config.SSHConfig{Port: 22}},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
				{Index: 1, Name: "agent-1", Command: "claude"},
			},
		},
		selectedAgentIdx: 1, // Select agent-1
		provisionStatus:  &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	// Simulate pressing 'c' (lowercase for connect)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	_, cmd := m.Update(msg)

	// Should return a command (the ExecProcess command)
	if cmd == nil {
		t.Error("pressing c should return a command")
	}
}

func TestKey_LowercaseC_NotAllowedWhenNoAgents(t *testing.T) {
	m := Model{
		cfg:             &config.Config{},
		vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents:          &agent.ListResult{Sessions: []agent.Session{}}, // No agents
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	_, cmd := m.Update(msg)

	// Should NOT return a command
	if cmd != nil {
		t.Error("pressing c with no agents should not return a command")
	}
}

func TestKey_LowercaseC_NotAllowedWhenVMStopped(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusStopped},
		agents: &agent.ListResult{
			Sessions: []agent.Session{{Index: 0, Name: "agent-0", Command: "bash"}},
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	_, cmd := m.Update(msg)

	// Should NOT return a command
	if cmd != nil {
		t.Error("pressing c when VM stopped should not return a command")
	}
}

func TestConnectFinishedMsg_RefreshesAgents(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
	}

	// Simulate connect finished message
	msg := connectFinishedMsg{err: nil}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should start loading agents
	if !updated.agentsLoading {
		t.Error("should start loading agents after connect finishes")
	}

	// Should return a command to fetch agents
	if cmd == nil {
		t.Error("should return command to refresh agents")
	}
}

func TestRenderHelp_ShowsConnectAction(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
			},
		},
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	help := m.renderHelp()

	// Should include connect action with number keys
	if !containsString(help, "c/1-9: connect") {
		t.Error("help should show 'c/1-9: connect'")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRefreshTickMsg_TriggersRefresh(t *testing.T) {
	m := Model{
		cfg:     &config.Config{},
		loading: false,
	}

	msg := refreshTickMsg{}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should start refreshing (not loading)
	if !updated.refreshing {
		t.Error("refresh tick should set refreshing")
	}
	if updated.loading {
		t.Error("refresh tick should not set loading (background refresh)")
	}

	// Should return a command (fetchVMInfo + scheduleRefresh batch)
	if cmd == nil {
		t.Error("refresh tick should return a command")
	}
}

func TestRefreshTickMsg_RefreshesWhenIdle(t *testing.T) {
	m := Model{
		cfg:     &config.Config{},
		vmInfo:  &cloud.VMInfo{Status: cloud.VMStatusRunning},
		loading: false,
	}

	msg := refreshTickMsg{}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Always-on: should set refreshing (not loading) for background refresh
	if !updated.refreshing {
		t.Error("refresh tick should set refreshing when idle")
	}
	if updated.loading {
		t.Error("refresh tick should not set loading for background refresh")
	}

	// Should return a command (fetchVMInfo + scheduleRefresh batch)
	if cmd == nil {
		t.Error("refresh tick should return a command when idle with valid config")
	}
}

func TestRefreshTickMsg_SkipsWhenPaused(t *testing.T) {
	m := Model{
		cfg:               &config.Config{},
		autoRefreshPaused: true,
		loading:           false,
	}

	msg := refreshTickMsg{}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should NOT start refreshing when paused
	if updated.refreshing {
		t.Error("refresh tick should not set refreshing when paused")
	}
	if updated.loading {
		t.Error("refresh tick should not set loading when paused")
	}

	// Should still reschedule (so unpausing resumes)
	if cmd == nil {
		t.Error("refresh tick should reschedule even when paused")
	}
}

func TestRefreshTickMsg_SkipsWhenAlreadyRefreshing(t *testing.T) {
	m := Model{
		cfg:        &config.Config{},
		refreshing: true,
	}

	msg := refreshTickMsg{}
	_, cmd := m.Update(msg)

	// Should still return a command (to reschedule)
	if cmd == nil {
		t.Error("refresh tick should reschedule when already refreshing")
	}
}

func TestRefreshTickMsg_ReschedulesWhenAlreadyLoading(t *testing.T) {
	m := Model{
		cfg:     &config.Config{},
		loading: true, // Already loading
	}

	msg := refreshTickMsg{}
	_, cmd := m.Update(msg)

	// Should still return a command (to reschedule)
	if cmd == nil {
		t.Error("refresh tick should reschedule when already loading")
	}
}

func TestRefreshTickMsg_NoRefreshWhenConfigError(t *testing.T) {
	m := Model{
		cfg:     &config.Config{},
		cfgErr:  errTestConfig,
		loading: false,
	}

	msg := refreshTickMsg{}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should NOT start loading when config has error
	if updated.loading {
		t.Error("refresh tick should not start loading when config has error")
	}

	// Should NOT return a command
	if cmd != nil {
		t.Error("refresh tick should not return a command when config has error")
	}
}

func TestRefreshTickMsg_NoRefreshWhenNilConfig(t *testing.T) {
	m := Model{
		cfg:     nil,
		loading: false,
	}

	msg := refreshTickMsg{}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should NOT start loading when config is nil
	if updated.loading {
		t.Error("refresh tick should not start loading when config is nil")
	}

	// Should NOT return a command
	if cmd != nil {
		t.Error("refresh tick should not return a command when config is nil")
	}
}

func TestKeyS_StartVM_SchedulesAutoRefresh(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusStopped, Name: "test-vm"},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should start operation
	if updated.operation != "starting" {
		t.Errorf("pressing S should set operation to 'starting', got %q", updated.operation)
	}

	// Should return a batched command (startVM + scheduleRefresh)
	if cmd == nil {
		t.Error("pressing S should return a command")
	}
}

func TestKeyT_StopVM_SchedulesAutoRefresh(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, Name: "test-vm"},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should start operation
	if updated.operation != "stopping" {
		t.Errorf("pressing T should set operation to 'stopping', got %q", updated.operation)
	}

	// Should return a batched command (stopVM + scheduleRefresh)
	if cmd == nil {
		t.Error("pressing T should return a command")
	}
}

func TestUppercaseQ_Quits(t *testing.T) {
	m := Model{}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}}
	_, cmd := m.Update(msg)

	// Should return quit command
	if cmd == nil {
		t.Error("pressing Q should return a command (quit)")
	}
}

func TestUppercaseR_Refreshes(t *testing.T) {
	m := Model{
		cfg: &config.Config{},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should trigger loading
	if !updated.loading {
		t.Error("pressing R should start loading")
	}
	if cmd == nil {
		t.Error("pressing R should return a command")
	}
}

func TestUppercaseC_Connects(t *testing.T) {
	// Uppercase C should now connect (not create), via normalization
	m := Model{
		cfg:    &config.Config{SSH: config.SSHConfig{Port: 22}},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
			},
		},
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}}
	_, cmd := m.Update(msg)

	// Should return a command (connect)
	if cmd == nil {
		t.Error("pressing C should connect to agent")
	}
}

func TestUppercaseK_NavigatesUp(t *testing.T) {
	// Uppercase K should navigate up (not kill), since kill is now '-'
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
				{Index: 1, Name: "agent-1", Command: "claude"},
			},
		},
		selectedAgentIdx: 1,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	// Should navigate up, not enter kill confirmation
	if updated.confirmingKill {
		t.Error("pressing K should not enter kill confirmation (kill is now '-')")
	}
	if updated.selectedAgentIdx != 0 {
		t.Errorf("pressing K should navigate up to index 0, got %d", updated.selectedAgentIdx)
	}
}

func TestNumberKey_DirectConnect(t *testing.T) {
	m := Model{
		cfg:    &config.Config{SSH: config.SSHConfig{Port: 22}},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
				{Index: 1, Name: "agent-1", Command: "claude"},
				{Index: 2, Name: "agent-2", Command: "aider"},
			},
		},
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	// Press '2' to connect to second agent
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	_, cmd := m.Update(msg)

	// Should return a command (connect)
	if cmd == nil {
		t.Error("pressing 2 should connect to agent at index 1")
	}
}

func TestNumberKey_OutOfRange(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
			},
		},
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	// Press '5' when only 1 agent exists
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}}
	_, cmd := m.Update(msg)

	// Should NOT return a command (out of range)
	if cmd != nil {
		t.Error("pressing 5 with only 1 agent should not return a command")
	}
}

func TestQuestionMark_TogglesHelp(t *testing.T) {
	m := Model{ready: true}

	// Press '?' to show help
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	if !updated.showHelp {
		t.Error("pressing ? should show help overlay")
	}

	// Press '?' again to dismiss
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	newModel, _ = updated.Update(msg)
	updated = newModel.(Model)

	if updated.showHelp {
		t.Error("pressing ? again should hide help overlay")
	}
}

func TestHelpOverlay_EscDismisses(t *testing.T) {
	m := Model{showHelp: true}

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	if updated.showHelp {
		t.Error("pressing Esc should dismiss help overlay")
	}
}

func TestRenderAgents_ShowsIndices(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
				{Index: 1, Name: "agent-1", Command: "claude"},
			},
		},
	}

	lines := m.renderAgents()

	// Should include [1] and [2] indices
	found1 := false
	found2 := false
	for _, line := range lines {
		if containsString(line, "[1]") {
			found1 = true
		}
		if containsString(line, "[2]") {
			found2 = true
		}
	}
	if !found1 {
		t.Error("agent list should show [1] index")
	}
	if !found2 {
		t.Error("agent list should show [2] index")
	}
}

func TestKeyN_CreateVM(t *testing.T) {
	// 'n' should now open size selection (was 'C')
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusNotFound},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	if !updated.selectingSize {
		t.Error("pressing n should open size selection for VM creation")
	}
}

func TestKeyN_UppercaseCreateVM(t *testing.T) {
	// 'N' should also create VM via normalization
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusNotFound},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	if !updated.selectingSize {
		t.Error("pressing N should open size selection for VM creation")
	}
}

func TestKeyD_DeleteVM(t *testing.T) {
	// lowercase 'd' should now delete (was 'D' only)
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusStopped},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	if !updated.confirmingDelete {
		t.Error("pressing d should enter delete confirmation")
	}
}

func TestRenderHelp_ShowsHelpShortcut(t *testing.T) {
	m := Model{}

	help := m.renderHelp()

	if !containsString(help, "?: help") {
		t.Error("help bar should show '?: help'")
	}
}

func TestKeyA_TogglesAutoRefreshPause(t *testing.T) {
	m := Model{
		cfg: &config.Config{},
	}

	// Press 'a' to pause
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	if !updated.autoRefreshPaused {
		t.Error("pressing a should pause auto-refresh")
	}

	// Press 'a' again to resume
	newModel, _ = updated.Update(msg)
	updated = newModel.(Model)

	if updated.autoRefreshPaused {
		t.Error("pressing a again should resume auto-refresh")
	}
}

func TestRenderHelp_ShowsPauseAutoRefresh(t *testing.T) {
	m := Model{}

	help := m.renderHelp()
	if !containsString(help, "a: pause auto") {
		t.Error("help should show 'a: pause auto' when not paused")
	}

	m.autoRefreshPaused = true
	help = m.renderHelp()
	if !containsString(help, "a: resume auto") {
		t.Error("help should show 'a: resume auto' when paused")
	}
}

func TestSessionName_DefaultWhenNoWorkspace(t *testing.T) {
	m := Model{}
	if got := m.sessionName(); got != "agents" {
		t.Errorf("sessionName() = %q, want %q", got, "agents")
	}
}

func TestSessionName_ReturnsSlug(t *testing.T) {
	m := Model{
		workspaceInfo: &workspace.Info{Slug: "acme-backend"},
	}
	if got := m.sessionName(); got != "acme-backend" {
		t.Errorf("sessionName() = %q, want %q", got, "acme-backend")
	}
}

func TestKeyW_TriggersSyncWhenWorkspaceDetected(t *testing.T) {
	m := Model{
		cfg:             &config.Config{},
		vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
		workspaceInfo:   &workspace.Info{Slug: "my-repo", RemoteURL: "git@github.com:owner/my-repo.git"},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "syncing" {
		t.Errorf("pressing w should set operation to 'syncing', got %q", updated.operation)
	}
	if cmd == nil {
		t.Error("pressing w should return a command")
	}
}

func TestKeyW_NoOpWhenNoWorkspace(t *testing.T) {
	m := Model{
		cfg:             &config.Config{},
		vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation == "syncing" {
		t.Error("pressing w without workspace should not trigger sync")
	}
	if cmd != nil {
		t.Error("pressing w without workspace should not return a command")
	}
}

func TestSyncMsg_Success_RefreshesAgents(t *testing.T) {
	ws := &workspace.Info{Slug: "my-repo", RemoteURL: "git@github.com:owner/my-repo.git"}
	m := Model{
		cfg:       &config.Config{},
		vmInfo:    &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		operation: "syncing",
	}

	msg := syncMsg{
		workspace: ws,
		result:    &workspace.SyncResult{Slug: "my-repo"},
	}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "" {
		t.Errorf("operation should be cleared, got %q", updated.operation)
	}
	if !updated.agentsLoading {
		t.Error("should start loading agents after sync")
	}
	if updated.workspaceInfo != ws {
		t.Error("should store workspace info from sync result")
	}
	if cmd == nil {
		t.Error("should return command to refresh agents")
	}
}

func TestSyncMsg_Error_StoresError(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		vmInfo:    &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		operation: "syncing",
	}

	msg := syncMsg{err: errors.New("sync failed")}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "" {
		t.Errorf("operation should be cleared, got %q", updated.operation)
	}
	if updated.agentsErr == nil {
		t.Error("should store error in agentsErr")
	}
	if cmd != nil {
		t.Error("should not return a command on error")
	}
}

func TestRenderHelp_ShowsSyncWhenWorkspaceDetected(t *testing.T) {
	m := Model{
		cfg:             &config.Config{},
		vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
		provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
		workspaceInfo:   &workspace.Info{Slug: "my-repo"},
	}

	help := m.renderHelp()

	if !containsString(help, "w: sync") {
		t.Error("help should show 'w: sync' when workspace is detected")
	}
}

// --- Handler error path tests ---

func TestConfigLoadedMsg_WithError(t *testing.T) {
	m := Model{loading: true}

	msg := configLoadedMsg{err: errTestConfig}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.cfgErr == nil {
		t.Error("configLoadedMsg with error should set cfgErr")
	}
	if updated.loading {
		t.Error("configLoadedMsg with error should clear loading")
	}
	if cmd != nil {
		t.Error("configLoadedMsg with error should not return a command")
	}
}

func TestConfigLoadedMsg_WithValidationError(t *testing.T) {
	m := Model{loading: true}

	// Config with missing required fields triggers validation error
	msg := configLoadedMsg{cfg: &config.Config{}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.cfgErr == nil {
		t.Error("configLoadedMsg with invalid config should set cfgErr")
	}
	if updated.loading {
		t.Error("configLoadedMsg with validation error should clear loading")
	}
	if cmd != nil {
		t.Error("configLoadedMsg with validation error should not return a command")
	}
}

func TestVMInfoMsg_WithError(t *testing.T) {
	m := Model{
		cfg:     &config.Config{},
		loading: true,
	}

	msg := vmInfoMsg{err: errors.New("API error")}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.vmErr == nil {
		t.Error("vmInfoMsg with error should set vmErr")
	}
	if updated.loading {
		t.Error("vmInfoMsg with error should clear loading")
	}
	if cmd != nil {
		t.Error("vmInfoMsg with error should not return a command")
	}
}

func TestVMInfoMsg_NonRunningVM(t *testing.T) {
	m := Model{
		cfg:     &config.Config{},
		loading: true,
		agents:  &agent.ListResult{Sessions: []agent.Session{{Index: 0, Name: "test"}}},
	}

	msg := vmInfoMsg{info: &cloud.VMInfo{Status: cloud.VMStatusStopped}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.loading {
		t.Error("vmInfoMsg should clear loading")
	}
	if updated.agents != nil {
		t.Error("vmInfoMsg with non-running VM should clear agents")
	}
	if cmd != nil {
		t.Error("vmInfoMsg with non-running VM should not fetch agents")
	}
}

func TestVMStartMsg_WithError(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		operation: "starting",
	}

	msg := vmStartMsg{err: errors.New("start failed")}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "" {
		t.Errorf("vmStartMsg with error should clear operation, got %q", updated.operation)
	}
	if updated.vmErr == nil {
		t.Error("vmStartMsg with error should set vmErr")
	}
	if cmd != nil {
		t.Error("vmStartMsg with error should not return a command")
	}
}

func TestVMStopMsg_WithError(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		operation: "stopping",
	}

	msg := vmStopMsg{err: errors.New("stop failed")}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "" {
		t.Errorf("vmStopMsg with error should clear operation, got %q", updated.operation)
	}
	if updated.vmErr == nil {
		t.Error("vmStopMsg with error should set vmErr")
	}
	if cmd != nil {
		t.Error("vmStopMsg with error should not return a command")
	}
}

func TestAgentAddedMsg_WithError(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		vmInfo:    &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		operation: "adding",
	}

	msg := agentAddedMsg{err: errors.New("add failed")}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "" {
		t.Errorf("agentAddedMsg with error should clear operation, got %q", updated.operation)
	}
	if updated.agentsErr == nil {
		t.Error("agentAddedMsg with error should set agentsErr")
	}
	if cmd != nil {
		t.Error("agentAddedMsg with error should not return a command")
	}
}

func TestAgentKilledMsg_WithError(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		vmInfo:    &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		operation: "killing",
	}

	msg := agentKilledMsg{index: 0, err: errors.New("kill failed")}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "" {
		t.Errorf("agentKilledMsg with error should clear operation, got %q", updated.operation)
	}
	if updated.agentsErr == nil {
		t.Error("agentKilledMsg with error should set agentsErr")
	}
	if cmd != nil {
		t.Error("agentKilledMsg with error should not return a command")
	}
}

func TestConnectFinishedMsg_WithError(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
	}

	msg := connectFinishedMsg{err: errors.New("connect failed")}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should still refresh agents even on error
	if !updated.agentsLoading {
		t.Error("connectFinishedMsg should still refresh agents even on error")
	}
	if cmd == nil {
		t.Error("connectFinishedMsg should return a command to refresh agents")
	}
}

// --- View/render tests ---

func TestRenderView_NotReady(t *testing.T) {
	m := Model{ready: false}
	view := m.renderView()
	if !containsString(view, "Loading...") {
		t.Error("renderView when not ready should show Loading...")
	}
}

func TestRenderView_ShowsHelp(t *testing.T) {
	m := Model{ready: true, showHelp: true}
	view := m.renderView()
	if !containsString(view, "Keyboard Shortcuts") {
		t.Error("renderView with showHelp should render help overlay")
	}
}

func TestRenderView_ConfigError(t *testing.T) {
	m := Model{ready: true, cfgErr: errTestConfig}
	view := m.renderView()
	if !containsString(view, "Configuration Error") {
		t.Error("renderView with cfgErr should show config error")
	}
}

func TestRenderView_SizeSelection(t *testing.T) {
	m := Model{
		ready:         true,
		selectingSize: true,
		cfg:           &config.Config{VM: config.VMConfig{MachineSizes: map[string]string{"small": "e2-small"}}},
		sizeOptions:   []string{"small"},
	}
	view := m.renderView()
	if !containsString(view, "Select VM size") {
		t.Error("renderView with selectingSize should show size selection")
	}
}

func TestRenderView_DeleteConfirmation(t *testing.T) {
	m := Model{
		ready:            true,
		confirmingDelete: true,
		cfg:              &config.Config{VM: config.VMConfig{Name: "test-vm"}},
	}
	view := m.renderView()
	if !containsString(view, "Delete VM") {
		t.Error("renderView with confirmingDelete should show delete confirmation")
	}
	if !containsString(view, "test-vm") {
		t.Error("renderView delete confirmation should show VM name")
	}
}

func TestRenderView_KillConfirmation(t *testing.T) {
	m := Model{
		ready:           true,
		confirmingKill:  true,
		killTargetName:  "agent-1",
		killTargetIndex: 1,
	}
	view := m.renderView()
	if !containsString(view, "Kill agent") {
		t.Error("renderView with confirmingKill should show kill confirmation")
	}
	if !containsString(view, "agent-1") {
		t.Error("renderView kill confirmation should show agent name")
	}
}

func TestRenderView_Operation(t *testing.T) {
	ops := map[string]string{
		"starting": "Starting VM",
		"stopping": "Stopping VM",
		"creating": "Creating VM",
		"deleting": "Deleting VM",
		"adding":   "Adding agent",
		"killing":  "Killing agent",
		"syncing":  "Syncing workspace",
	}
	for op, expected := range ops {
		t.Run(op, func(t *testing.T) {
			m := Model{ready: true, operation: op}
			view := m.renderView()
			if !containsString(view, expected) {
				t.Errorf("renderView with operation %q should contain %q", op, expected)
			}
		})
	}
}

func TestRenderView_Loading(t *testing.T) {
	m := Model{ready: true, loading: true, vmInfo: nil}
	view := m.renderView()
	if !containsString(view, "Loading VM status") {
		t.Error("renderView loading with nil vmInfo should show loading message")
	}
}

func TestRenderView_VMError(t *testing.T) {
	m := Model{ready: true, vmErr: errors.New("API failed")}
	view := m.renderView()
	if !containsString(view, "API failed") {
		t.Error("renderView with vmErr should show the error message")
	}
}

func TestFormatStatus_AllValues(t *testing.T) {
	m := Model{}
	tests := []struct {
		status cloud.VMStatus
		want   string
	}{
		{cloud.VMStatusRunning, "running"},
		{cloud.VMStatusStopped, "stopped"},
		{cloud.VMStatusStarting, "starting"},
		{cloud.VMStatusStopping, "stopping"},
		{cloud.VMStatusUnknown, string(cloud.VMStatusUnknown)},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := m.formatStatus(tt.status)
			if !containsString(got, tt.want) {
				t.Errorf("formatStatus(%q) = %q, want containing %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestFormatProvisionStatus_AllStates(t *testing.T) {
	tests := []struct {
		name string
		m    Model
		want string
	}{
		{
			name: "loading",
			m:    Model{provisionLoading: true},
			want: "checking",
		},
		{
			name: "error",
			m:    Model{provisionErr: errors.New("ssh failed")},
			want: "error",
		},
		{
			name: "nil status",
			m:    Model{},
			want: "unknown",
		},
		{
			name: "pending",
			m:    Model{provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusPending}},
			want: "pending",
		},
		{
			name: "running",
			m:    Model{provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusRunning}},
			want: "running",
		},
		{
			name: "running with progress",
			m:    Model{provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusRunning, Progress: "step 3/5"}},
			want: "step 3/5",
		},
		{
			name: "completed",
			m:    Model{provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted}},
			want: "completed",
		},
		{
			name: "failed",
			m:    Model{provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusFailed}},
			want: "failed",
		},
		{
			name: "failed with detail",
			m:    Model{provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusFailed, Error: "script exited"}},
			want: "script exited",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.formatProvisionStatus()
			if !containsString(got, tt.want) {
				t.Errorf("formatProvisionStatus() = %q, want containing %q", got, tt.want)
			}
		})
	}
}

func TestRenderConfigError_ContainsInstructions(t *testing.T) {
	m := Model{cfgErr: errors.New("config not found")}
	rendered := m.renderConfigError()
	if !containsString(rendered, "cloudcoop config init") {
		t.Error("renderConfigError should contain setup wizard instruction")
	}
	if !containsString(rendered, "cloudcoop.toml") {
		t.Error("renderConfigError should contain config file path")
	}
}

func TestRenderSizeSelection_ShowsOptions(t *testing.T) {
	m := Model{
		cfg: &config.Config{VM: config.VMConfig{MachineSizes: map[string]string{
			"small":  "e2-small",
			"medium": "e2-medium",
		}}},
		sizeOptions:     []string{"small", "medium"},
		selectedSizeIdx: 0,
	}
	rendered := m.renderSizeSelection()
	if !containsString(rendered, "small") {
		t.Error("renderSizeSelection should show 'small' option")
	}
	if !containsString(rendered, "medium") {
		t.Error("renderSizeSelection should show 'medium' option")
	}
	if !containsString(rendered, ">") {
		t.Error("renderSizeSelection should show selection indicator")
	}
}

func TestRenderDeleteConfirmation_ShowsVMName(t *testing.T) {
	m := Model{cfg: &config.Config{VM: config.VMConfig{Name: "my-sandbox"}}}
	rendered := m.renderDeleteConfirmation()
	if !containsString(rendered, "my-sandbox") {
		t.Error("renderDeleteConfirmation should show VM name")
	}
	if !containsString(rendered, "permanently delete") {
		t.Error("renderDeleteConfirmation should warn about permanent deletion")
	}
}

func TestRenderKillConfirmation_ShowsAgentName(t *testing.T) {
	m := Model{killTargetName: "feature-auth", killTargetIndex: 2}
	rendered := m.renderKillConfirmation()
	if !containsString(rendered, "feature-auth") {
		t.Error("renderKillConfirmation should show agent name")
	}
}

// --- VM operation success tests ---

func TestVMStartMsg_Success(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		operation: "starting",
	}

	msg := vmStartMsg{err: nil}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "" {
		t.Errorf("vmStartMsg success should clear operation, got %q", updated.operation)
	}
	if !updated.loading {
		t.Error("vmStartMsg success should set loading to refresh VM info")
	}
	if cmd == nil {
		t.Error("vmStartMsg success should return a command to fetch VM info")
	}
}

func TestVMStopMsg_Success(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		operation: "stopping",
	}

	msg := vmStopMsg{err: nil}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "" {
		t.Errorf("vmStopMsg success should clear operation, got %q", updated.operation)
	}
	if !updated.loading {
		t.Error("vmStopMsg success should set loading to refresh VM info")
	}
	if cmd == nil {
		t.Error("vmStopMsg success should return a command to fetch VM info")
	}
}

func TestVMCreateMsg_WithError(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		operation: "creating",
	}

	msg := vmCreateMsg{err: errors.New("quota exceeded")}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "" {
		t.Errorf("vmCreateMsg with error should clear operation, got %q", updated.operation)
	}
	if updated.vmErr == nil {
		t.Error("vmCreateMsg with error should set vmErr")
	}
	if cmd != nil {
		t.Error("vmCreateMsg with error should not return a command")
	}
}

func TestVMDeleteMsg_WithError(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		operation: "deleting",
	}

	msg := vmDeleteMsg{err: errors.New("delete failed")}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.operation != "" {
		t.Errorf("vmDeleteMsg with error should clear operation, got %q", updated.operation)
	}
	if updated.vmErr == nil {
		t.Error("vmDeleteMsg with error should set vmErr")
	}
	if cmd != nil {
		t.Error("vmDeleteMsg with error should not return a command")
	}
}

func TestVMInfoMsg_RunningVM_FetchesAgents(t *testing.T) {
	m := Model{
		cfg:     &config.Config{},
		loading: true,
	}

	msg := vmInfoMsg{info: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	if updated.loading {
		t.Error("vmInfoMsg should clear loading")
	}
	if !updated.agentsLoading {
		t.Error("vmInfoMsg with running VM should set agentsLoading")
	}
	if !updated.provisionLoading {
		t.Error("vmInfoMsg with running VM should set provisionLoading")
	}
	if cmd == nil {
		t.Error("vmInfoMsg with running VM should return commands to fetch agents and provision status")
	}
}

func TestUppercaseJ_NavigatesDown(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning},
		agents: &agent.ListResult{
			Sessions: []agent.Session{
				{Index: 0, Name: "agent-0", Command: "bash"},
				{Index: 1, Name: "agent-1", Command: "claude"},
			},
		},
		selectedAgentIdx: 0,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	if updated.selectedAgentIdx != 1 {
		t.Errorf("pressing J should navigate down to index 1, got %d", updated.selectedAgentIdx)
	}
}
