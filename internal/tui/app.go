// Package tui provides the terminal user interface for cloudcoop.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
)

// Model represents the TUI application state.
type Model struct {
	width     int
	height    int
	ready     bool
	loading   bool
	operation string // "", "starting", "stopping", "adding", "killing"

	cfg     *config.Config
	cfgErr  error
	vmInfo  *cloud.VMInfo
	vmErr   error
	cleanup func()

	agents        *agent.ListResult
	agentsErr     error
	agentsLoading bool

	// Provisioning status
	provisionStatus  *provisioning.StatusInfo
	provisionErr     error
	provisionLoading bool

	// Agent selection and confirmation
	selectedAgentIdx int    // currently selected agent index in the list (not tmux index)
	confirmingKill   bool   // true when waiting for kill confirmation
	killTargetIndex  int    // tmux window index to kill
	killTargetName   string // name of agent to kill (for display)

	// VM create/delete states
	selectingSize    bool     // true when showing size selection menu
	sizeOptions      []string // available size names (e.g., "small", "medium", "large", "xlarge")
	selectedSizeIdx  int      // currently selected size index
	confirmingDelete bool     // true when waiting for delete confirmation
}

// New creates a new TUI application model.
func New() Model {
	return Model{
		loading: true,
	}
}

// Init initializes the TUI application.
func (m Model) Init() tea.Cmd {
	return loadConfig
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		newM, cmd := m.handleKeyMsg(msg)
		return newM, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case configLoadedMsg:
		return m.handleConfigLoaded(msg)

	case vmInfoMsg:
		return m.handleVMInfo(msg)

	case agentsMsg:
		return m.handleAgents(msg)

	case provisionStatusMsg:
		return m.handleProvisionStatus(msg)

	case vmStartMsg:
		return m.handleVMStart(msg)

	case vmStopMsg:
		return m.handleVMStop(msg)

	case vmCreateMsg:
		return m.handleVMCreate(msg)

	case vmDeleteMsg:
		return m.handleVMDelete(msg)

	case agentAddedMsg:
		return m.handleAgentAdded(msg)

	case agentKilledMsg:
		return m.handleAgentKilled(msg)

	case connectFinishedMsg:
		return m.handleConnectFinished(msg)
	}

	return m, nil
}

// View renders the TUI.
func (m Model) View() string {
	return m.renderView()
}
