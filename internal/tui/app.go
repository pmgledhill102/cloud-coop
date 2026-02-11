// Package tui provides the terminal user interface for cloudcoop.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
	"github.com/cloud-coop/cloudcoop/internal/workspace"
)

// Model represents the TUI application state.
type Model struct {
	width             int
	height            int
	ready             bool
	loading           bool
	refreshing        bool // background refresh in progress (no loading flash)
	autoRefreshPaused bool
	operation         string // "", "starting", "stopping", "adding", "killing"

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

	// Workspace
	workspaceInfo *workspace.Info // detected from CWD on startup (nil if not in a git repo)

	// Firewall
	firewallChecked bool // true after firewall check has been triggered

	// SSH key
	sshKeyChecked bool // true after SSH key check has been triggered

	// Help overlay
	showHelp bool // true when help overlay is visible
}

// sessionName returns the tmux session name to use. It prefers the workspace
// slug when available, falling back to the default "agents" session name.
func (m Model) sessionName() string {
	if m.workspaceInfo != nil {
		return m.workspaceInfo.Slug
	}
	return defaultSessionName
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

	case refreshTickMsg:
		return m.handleRefreshTick()

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

	case firewallCheckedMsg:
		// Firewall check is fire-and-forget; log errors but don't update UI
		if msg.err != nil {
			log.Debug("firewall check failed (non-fatal)", "error", msg.err)
		}
		return m, nil

	case sshKeyCheckedMsg:
		// SSH key check is fire-and-forget; log errors but don't update UI
		if msg.err != nil {
			log.Debug("SSH key check failed (non-fatal)", "error", msg.err)
		}
		return m, nil

	case syncMsg:
		return m.handleSync(msg)
	}

	return m, nil
}

// View renders the TUI.
func (m Model) View() string {
	return m.renderView()
}
