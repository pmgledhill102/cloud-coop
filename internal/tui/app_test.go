package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
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

func TestShouldAutoRefresh(t *testing.T) {
	tests := []struct {
		name string
		m    Model
		want bool
	}{
		{
			name: "auto-refresh during starting operation",
			m:    Model{operation: "starting"},
			want: true,
		},
		{
			name: "auto-refresh during stopping operation",
			m:    Model{operation: "stopping"},
			want: true,
		},
		{
			name: "auto-refresh during creating operation",
			m:    Model{operation: "creating"},
			want: true,
		},
		{
			name: "auto-refresh during deleting operation",
			m:    Model{operation: "deleting"},
			want: true,
		},
		{
			name: "auto-refresh during adding operation",
			m:    Model{operation: "adding"},
			want: true,
		},
		{
			name: "auto-refresh during killing operation",
			m:    Model{operation: "killing"},
			want: true,
		},
		{
			name: "auto-refresh when provisioning is pending",
			m: Model{
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
				provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusPending},
			},
			want: true,
		},
		{
			name: "auto-refresh when provisioning is running",
			m: Model{
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
				provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusRunning},
			},
			want: true,
		},
		{
			name: "no auto-refresh when provisioning is completed",
			m: Model{
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
				provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusCompleted},
			},
			want: false,
		},
		{
			name: "no auto-refresh when provisioning is failed",
			m: Model{
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusRunning},
				provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusFailed},
			},
			want: false,
		},
		{
			name: "no auto-refresh when idle with no operation",
			m:    Model{},
			want: false,
		},
		{
			name: "no auto-refresh when VM stopped even with provision status running",
			m: Model{
				vmInfo:          &cloud.VMInfo{Status: cloud.VMStatusStopped},
				provisionStatus: &provisioning.StatusInfo{Status: provisioning.StatusRunning},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.shouldAutoRefresh()
			if got != tt.want {
				t.Errorf("shouldAutoRefresh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRefreshTickMsg_TriggersRefresh(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		operation: "starting", // During operation, should refresh
		loading:   false,
	}

	msg := refreshTickMsg{}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should start loading
	if !updated.loading {
		t.Error("refresh tick should start loading")
	}

	// Should return a command
	if cmd == nil {
		t.Error("refresh tick should return a command")
	}
}

func TestRefreshTickMsg_NoRefreshWhenNotNeeded(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		operation: "", // No operation, no auto-refresh needed
		vmInfo:    &cloud.VMInfo{Status: cloud.VMStatusRunning},
		provisionStatus: &provisioning.StatusInfo{
			Status: provisioning.StatusCompleted, // Provisioning complete
		},
		loading: false,
	}

	msg := refreshTickMsg{}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should NOT start loading
	if updated.loading {
		t.Error("refresh tick should not start loading when idle")
	}

	// Should NOT return a command
	if cmd != nil {
		t.Error("refresh tick should not return a command when idle")
	}
}

func TestRefreshTickMsg_ReschedulesWhenAlreadyLoading(t *testing.T) {
	m := Model{
		cfg:       &config.Config{},
		operation: "starting",
		loading:   true, // Already loading
	}

	msg := refreshTickMsg{}
	_, cmd := m.Update(msg)

	// Should still return a command (to reschedule)
	if cmd == nil {
		t.Error("refresh tick should reschedule when already loading during operation")
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
