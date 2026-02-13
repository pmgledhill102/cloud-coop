package tui

import (
	"github.com/charmbracelet/bubbles/key"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

// keyMap defines all TUI keybindings using structured key.Binding values.
// Each binding includes help text displayed in the footer and overlay.
type keyMap struct {
	// Navigation
	Up   key.Binding
	Down key.Binding

	// VM Management
	Start  key.Binding
	Stop   key.Binding
	Create key.Binding
	Delete key.Binding

	// Agent Management
	AddAgent  key.Binding
	KillAgent key.Binding
	Connect   key.Binding
	Sync      key.Binding

	// General
	Refresh     key.Binding
	AutoRefresh key.Binding
	Help        key.Binding
	Quit        key.Binding

	// Dialogue keys (not shown in main help)
	Confirm key.Binding
	Cancel  key.Binding
}

// newKeyMap returns the default keybindings.
func newKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k", "K"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j", "J"),
			key.WithHelp("↓/j", "down"),
		),
		Start: key.NewBinding(
			key.WithKeys("s", "S"),
			key.WithHelp("s", "start"),
		),
		Stop: key.NewBinding(
			key.WithKeys("t", "T"),
			key.WithHelp("t", "stop"),
		),
		Create: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n", "new VM"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d", "D"),
			key.WithHelp("d", "delete"),
		),
		AddAgent: key.NewBinding(
			key.WithKeys("+"),
			key.WithHelp("+", "add agent"),
		),
		KillAgent: key.NewBinding(
			key.WithKeys("-"),
			key.WithHelp("-", "kill agent"),
		),
		Connect: key.NewBinding(
			key.WithKeys("c", "C", "enter"),
			key.WithHelp("Enter/c/1-9", "connect"),
		),
		Sync: key.NewBinding(
			key.WithKeys("w", "W"),
			key.WithHelp("w", "sync"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r", "R"),
			key.WithHelp("r", "refresh"),
		),
		AutoRefresh: key.NewBinding(
			key.WithKeys("a", "A"),
			key.WithHelp("a", "pause auto"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "Q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("n", "esc", "escape"),
			key.WithHelp("n/Esc", "cancel"),
		),
	}
}

// ShortHelp returns the keybindings shown in the footer help line.
// Only enabled bindings are rendered by the help bubble.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Help, k.Quit, k.Refresh, k.AutoRefresh,
		k.Start, k.Stop, k.Create, k.Delete,
		k.Sync, k.AddAgent, k.Connect, k.KillAgent,
		k.Up, k.Down,
	}
}

// FullHelp returns keybindings grouped for the expanded help overlay.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Start, k.Stop, k.Create, k.Delete},
		{k.AddAgent, k.KillAgent, k.Connect, k.Sync},
		{k.Refresh, k.AutoRefresh, k.Help, k.Quit},
	}
}

// updateKeyStates enables/disables keybindings based on the current model state.
func (m *Model) updateKeyStates() {
	canVM := m.canVMOp()
	canAgent := m.canModifyAgents()
	hasAgents := m.hasAgents()

	// VM operations: contextual based on VM status
	m.keys.Start.SetEnabled(canVM && m.vmInfo != nil && m.vmInfo.Status == cloud.VMStatusStopped)
	m.keys.Stop.SetEnabled(canVM && m.vmInfo != nil && m.vmInfo.Status == cloud.VMStatusRunning)
	m.keys.Create.SetEnabled(canVM && m.vmInfo != nil && m.vmInfo.Status == cloud.VMStatusNotFound)
	m.keys.Delete.SetEnabled(canVM && m.vmInfo != nil && m.vmInfo.Status == cloud.VMStatusStopped)

	// Agent operations
	m.keys.AddAgent.SetEnabled(canAgent)
	m.keys.KillAgent.SetEnabled(canAgent && hasAgents)
	m.keys.Connect.SetEnabled(canAgent && hasAgents)
	m.keys.Sync.SetEnabled(canAgent && m.workspaceInfo != nil)
	m.keys.Up.SetEnabled(hasAgents)
	m.keys.Down.SetEnabled(hasAgents)

	// Auto-refresh label changes based on state
	if m.autoRefreshPaused {
		m.keys.AutoRefresh.SetHelp("a", "resume auto")
	} else {
		m.keys.AutoRefresh.SetHelp("a", "pause auto")
	}

	// Refresh needs config
	m.keys.Refresh.SetEnabled(m.cfg != nil && m.cfgErr == nil && m.operation == "")
}
