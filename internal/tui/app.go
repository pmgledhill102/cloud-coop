// Package tui provides the terminal user interface for cloudcoop.
package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

	// Bubbles sub-models
	spinner  spinner.Model
	keys     keyMap
	help     help.Model
	viewport viewport.Model
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
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D7005F", Dark: "#FF87AF"})

	h := help.New()
	h.ShortSeparator = " · "

	return Model{
		loading: true,
		spinner: s,
		keys:    newKeyMap(),
		help:    h,
	}
}

// Init initialises the TUI application.
func (m Model) Init() tea.Cmd {
	return tea.Batch(loadConfig, m.spinner.Tick)
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Ensure sub-models are initialised (handles direct Model{} in tests).
	m.ensureInit()

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		newM, cmd := m.handleKeyMsg(msg)
		newM.syncViewport()
		return newM, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Reserve space for header (title + subtitle) and footer (help line).
		headerHeight := lipgloss.Height(m.renderHeader())
		footerHeight := lipgloss.Height(m.renderFooter())
		vpHeight := msg.Height - headerHeight - footerHeight
		if vpHeight < 1 {
			vpHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = vpHeight
		}

		m.help.Width = msg.Width
		m.syncViewport()
		return m, nil

	case configLoadedMsg:
		newM, cmd := m.handleConfigLoaded(msg)
		newM.syncViewport()
		return newM, cmd

	case vmInfoMsg:
		newM, cmd := m.handleVMInfo(msg)
		newM.syncViewport()
		return newM, cmd

	case vmDetailsMsg:
		newM, cmd := m.handleVMDetails(msg)
		newM.syncViewport()
		return newM, cmd

	case agentsMsg:
		newM, cmd := m.handleAgents(msg)
		newM.syncViewport()
		return newM, cmd

	case refreshTickMsg:
		return m.handleRefreshTick()

	case vmStartMsg:
		newM, cmd := m.handleVMStart(msg)
		newM.syncViewport()
		return newM, cmd

	case vmStopMsg:
		newM, cmd := m.handleVMStop(msg)
		newM.syncViewport()
		return newM, cmd

	case vmCreateMsg:
		newM, cmd := m.handleVMCreate(msg)
		newM.syncViewport()
		return newM, cmd

	case vmDeleteMsg:
		newM, cmd := m.handleVMDelete(msg)
		newM.syncViewport()
		return newM, cmd

	case agentAddedMsg:
		newM, cmd := m.handleAgentAdded(msg)
		newM.syncViewport()
		return newM, cmd

	case agentKilledMsg:
		newM, cmd := m.handleAgentKilled(msg)
		newM.syncViewport()
		return newM, cmd

	case connectReadyMsg:
		return m, tea.ExecProcess(msg.cmd, func(err error) tea.Msg {
			return connectFinishedMsg{err: err}
		})

	case connectFinishedMsg:
		newM, cmd := m.handleConnectFinished(msg)
		newM.syncViewport()
		return newM, cmd

	case firewallCheckedMsg:
		if msg.err != nil {
			log.Debug("firewall check failed (non-fatal)", "error", msg.err)
		}
		return m, nil

	case sshKeyCheckedMsg:
		if msg.err != nil {
			log.Debug("SSH key check failed (non-fatal)", "error", msg.err)
		}
		return m, nil

	case syncMsg:
		newM, cmd := m.handleSync(msg)
		newM.syncViewport()
		return newM, cmd

	default:
		// Update spinner animation (tick messages).
		var spinCmd tea.Cmd
		m.spinner, spinCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinCmd)

		// Forward to viewport for scroll handling.
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)

		m.syncViewport()
		return m, tea.Batch(cmds...)
	}
}

// View renders the TUI.
func (m Model) View() string {
	return m.renderView()
}
