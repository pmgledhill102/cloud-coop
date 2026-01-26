package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
)

var errTestConfig = errors.New("test config error")

func TestCanModifyAgents(t *testing.T) {
	tests := []struct {
		name string
		m    Model
		want bool
	}{
		{
			name: "can modify when VM running and no operation",
			m: Model{
				cfg:    &config.Config{},
				vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning},
			},
			want: true,
		},
		{
			name: "cannot modify when VM stopped",
			m: Model{
				cfg:    &config.Config{},
				vmInfo: &cloud.VMInfo{Status: cloud.VMStatusStopped},
			},
			want: false,
		},
		{
			name: "cannot modify when config error",
			m: Model{
				cfg:    &config.Config{},
				cfgErr: errTestConfig,
				vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning},
			},
			want: false,
		},
		{
			name: "cannot modify during operation",
			m: Model{
				cfg:       &config.Config{},
				vmInfo:    &cloud.VMInfo{Status: cloud.VMStatusRunning},
				operation: "adding",
			},
			want: false,
		},
		{
			name: "cannot modify while agents loading",
			m: Model{
				cfg:           &config.Config{},
				vmInfo:        &cloud.VMInfo{Status: cloud.VMStatusRunning},
				agentsLoading: true,
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

func TestKeyA_AddAgent(t *testing.T) {
	// Setup model in state where adding is allowed
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents: &agent.ListResult{Sessions: []agent.Session{}},
	}

	// Simulate pressing 'A'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should trigger adding operation
	if updated.operation != "adding" {
		t.Errorf("pressing A should set operation to 'adding', got %q", updated.operation)
	}

	// Should return a command (the addAgent command)
	if cmd == nil {
		t.Error("pressing A should return a command")
	}
}

func TestKeyA_NotAllowedWhenVMStopped(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusStopped},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}}
	newModel, cmd := m.Update(msg)
	updated := newModel.(Model)

	// Should NOT trigger adding operation
	if updated.operation == "adding" {
		t.Error("pressing A when VM stopped should not trigger add")
	}
	if cmd != nil {
		t.Error("pressing A when VM stopped should not return a command")
	}
}

func TestKeyK_KillConfirmation(t *testing.T) {
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
	}

	// Simulate pressing 'K'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	// Should enter confirmation mode
	if !updated.confirmingKill {
		t.Error("pressing K should enter confirmation mode")
	}
	if updated.killTargetIndex != 1 {
		t.Errorf("kill target should be index 1, got %d", updated.killTargetIndex)
	}
	if updated.killTargetName != "agent-1" {
		t.Errorf("kill target name should be 'agent-1', got %q", updated.killTargetName)
	}
}

func TestKeyK_NotAllowedWithNoAgents(t *testing.T) {
	m := Model{
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents: &agent.ListResult{Sessions: []agent.Session{}}, // No agents
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}}
	newModel, _ := m.Update(msg)
	updated := newModel.(Model)

	// Should NOT enter confirmation mode
	if updated.confirmingKill {
		t.Error("pressing K with no agents should not enter confirmation mode")
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
	}

	help := m.renderHelp()

	// Should include agent actions
	if !containsString(help, "A: add agent") {
		t.Error("help should show 'A: add agent'")
	}
	if !containsString(help, "K: kill agent") {
		t.Error("help should show 'K: kill agent'")
	}
}

func TestRenderHelp_ConfirmationMode(t *testing.T) {
	m := Model{
		confirmingKill: true,
	}

	help := m.renderHelp()

	// Should show confirmation options only
	if !containsString(help, "Y: confirm") {
		t.Error("help in confirmation mode should show 'Y: confirm'")
	}
	if !containsString(help, "N: cancel") {
		t.Error("help in confirmation mode should show 'N: cancel'")
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
		cfg:    &config.Config{},
		vmInfo: &cloud.VMInfo{Status: cloud.VMStatusRunning, ExternalIP: "1.2.3.4"},
		agents: &agent.ListResult{Sessions: []agent.Session{}}, // No agents
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
	}

	help := m.renderHelp()

	// Should include connect action (lowercase c)
	if !containsString(help, "c: connect") {
		t.Error("help should show 'c: connect'")
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
