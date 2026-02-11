package tui

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
)

// handleKeyMsg dispatches keyboard input to the appropriate handler.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.showHelp {
		return m.handleHelpOverlayKeys(msg)
	}
	if m.selectingSize {
		return m.handleSizeSelectionKeys(msg)
	}
	if m.confirmingDelete {
		return m.handleDeleteConfirmationKeys(msg)
	}
	if m.confirmingKill {
		return m.handleKillConfirmationKeys(msg)
	}
	return m.handleNormalKeys(msg)
}

func (m Model) handleSizeSelectionKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	switch key {
	case "up", "k":
		if m.selectedSizeIdx > 0 {
			m.selectedSizeIdx--
		}
	case "down", "j":
		if m.selectedSizeIdx < len(m.sizeOptions)-1 {
			m.selectedSizeIdx++
		}
	case "enter":
		m.selectingSize = false
		machineType := m.cfg.VM.MachineSizes[m.sizeOptions[m.selectedSizeIdx]]
		m.operation = "creating"
		return m, tea.Batch(createVM(m.cfg, machineType), scheduleRefresh(m.cfg))
	case "esc", "escape", "n":
		m.selectingSize = false
	}
	return m, nil
}

func (m Model) handleDeleteConfirmationKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	switch key {
	case "y":
		m.confirmingDelete = false
		m.operation = "deleting"
		return m, tea.Batch(deleteVM(m.cfg), scheduleRefresh(m.cfg))
	case "n", "esc", "escape":
		m.confirmingDelete = false
	}
	return m, nil
}

func (m Model) handleKillConfirmationKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	switch key {
	case "y":
		m.confirmingKill = false
		m.operation = "killing"
		return m, tea.Batch(killAgent(m.cfg, m.vmInfo, m.killTargetIndex, m.sessionName()), scheduleRefresh(m.cfg))
	case "n", "esc", "escape":
		m.confirmingKill = false
	}
	return m, nil
}

func (m Model) handleHelpOverlayKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "?", "esc", "escape":
		m.showHelp = false
	case "q", "ctrl+c":
		if m.cleanup != nil {
			m.cleanup()
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleNormalKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	switch key {
	case "q", "ctrl+c":
		if m.cleanup != nil {
			m.cleanup()
		}
		return m, tea.Quit
	case "r":
		if m.cfg != nil && m.cfgErr == nil && m.operation == "" {
			m.loading = true
			return m, fetchVMInfo(m.cfg)
		}
	case "s":
		if m.canVMOp() && m.vmInfo.Status == cloud.VMStatusStopped {
			m.operation = "starting"
			return m, tea.Batch(startVM(m.cfg), scheduleRefresh(m.cfg))
		}
	case "t":
		if m.canVMOp() && m.vmInfo.Status == cloud.VMStatusRunning {
			m.operation = "stopping"
			return m, tea.Batch(stopVM(m.cfg), scheduleRefresh(m.cfg))
		}
	case "+":
		if m.canModifyAgents() {
			m.operation = "adding"
			return m, tea.Batch(addAgent(m.cfg, m.vmInfo, m.sessionName()), scheduleRefresh(m.cfg))
		}
	case "w":
		if m.canModifyAgents() && m.workspaceInfo != nil {
			m.operation = "syncing"
			return m, tea.Batch(syncWorkspace(m.cfg, m.vmInfo, m.workspaceInfo), scheduleRefresh(m.cfg))
		}
	case "-":
		if m.canModifyAgents() && m.hasAgents() && m.selectedAgentIdx < len(m.agents.Sessions) {
			selected := m.agents.Sessions[m.selectedAgentIdx]
			m.killTargetIndex = selected.Index
			m.killTargetName = selected.Name
			m.confirmingKill = true
		}
	case "c":
		if m.canModifyAgents() && m.hasAgents() && m.selectedAgentIdx < len(m.agents.Sessions) {
			return m, connectToAgent(m.cfg, m.vmInfo, m.agents.Sessions[m.selectedAgentIdx].Index, m.sessionName())
		}
	case "n":
		if m.canVMOp() && m.vmInfo.Status == cloud.VMStatusNotFound {
			m.sizeOptions = []string{"small", "medium", "large", "xlarge"}
			m.selectedSizeIdx = 0
			m.selectingSize = true
		}
	case "d":
		if m.canVMOp() && m.vmInfo.Status == cloud.VMStatusStopped {
			m.confirmingDelete = true
		}
	case "a":
		m.autoRefreshPaused = !m.autoRefreshPaused
	case "?":
		m.showHelp = true
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(key[0]-'0') - 1 // convert "1"-"9" to 0-based index
		if m.canModifyAgents() && m.hasAgents() && idx < len(m.agents.Sessions) {
			return m, connectToAgent(m.cfg, m.vmInfo, m.agents.Sessions[idx].Index, m.sessionName())
		}
	case "up", "k":
		if m.hasAgents() && m.selectedAgentIdx > 0 {
			m.selectedAgentIdx--
		}
	case "down", "j":
		if m.hasAgents() && m.selectedAgentIdx < len(m.agents.Sessions)-1 {
			m.selectedAgentIdx++
		}
	}
	return m, nil
}

// canVMOp returns true if VM operations (start/stop/create/delete) are allowed.
func (m Model) canVMOp() bool {
	return m.cfg != nil && m.cfgErr == nil && m.vmInfo != nil && m.operation == ""
}

// hasAgents returns true if there are agents to select from.
func (m Model) hasAgents() bool {
	return m.agents != nil && len(m.agents.Sessions) > 0
}

func (m Model) handleConfigLoaded(msg configLoadedMsg) (Model, tea.Cmd) {
	m.cfg = msg.cfg
	m.cfgErr = msg.err
	m.workspaceInfo = msg.workspace
	if msg.err != nil {
		m.loading = false
		return m, nil
	}
	if err := m.cfg.Validate(); err != nil {
		m.cfgErr = err
		m.loading = false
		return m, nil
	}
	return m, tea.Batch(fetchVMInfo(m.cfg), scheduleRefresh(m.cfg))
}

func (m Model) handleVMInfo(msg vmInfoMsg) (Model, tea.Cmd) {
	m.loading = false
	m.refreshing = false
	m.vmInfo = msg.info
	m.vmErr = msg.err
	if msg.cleanup != nil {
		if m.cleanup != nil {
			m.cleanup()
		}
		m.cleanup = msg.cleanup
	}
	if msg.err == nil && msg.info != nil && msg.info.Status == cloud.VMStatusRunning {
		m.agentsLoading = true
		m.provisionLoading = true
		m.agents = nil
		m.agentsErr = nil
		m.provisionStatus = nil
		m.provisionErr = nil
		cmds := []tea.Cmd{
			fetchVMDetails(m.cfg, msg.info, m.sessionName()),
		}
		if !m.firewallChecked {
			m.firewallChecked = true
			cmds = append(cmds, ensureFirewall(m.cfg))
		}
		if !m.sshKeyChecked {
			m.sshKeyChecked = true
			cmds = append(cmds, ensureSSHKey(m.cfg, msg.info))
		}
		return m, tea.Batch(cmds...)
	}
	m.agents = nil
	m.agentsErr = nil
	m.provisionStatus = nil
	m.provisionErr = nil
	return m, nil
}

func (m Model) handleVMDetails(msg vmDetailsMsg) (Model, tea.Cmd) {
	m.agentsLoading = false
	m.provisionLoading = false
	m.agents = msg.agents
	m.agentsErr = msg.agentsE
	m.provisionStatus = msg.status
	m.provisionErr = msg.statusE
	return m, nil
}

func (m Model) handleAgents(msg agentsMsg) (Model, tea.Cmd) {
	m.agentsLoading = false
	m.agents = msg.result
	m.agentsErr = msg.err
	return m, nil
}

func (m Model) handleVMStart(msg vmStartMsg) (Model, tea.Cmd) {
	return m.handleVMOpResult(msg.err)
}

func (m Model) handleVMStop(msg vmStopMsg) (Model, tea.Cmd) {
	return m.handleVMOpResult(msg.err)
}

func (m Model) handleVMCreate(msg vmCreateMsg) (Model, tea.Cmd) {
	return m.handleVMOpResult(msg.err)
}

func (m Model) handleVMDelete(msg vmDeleteMsg) (Model, tea.Cmd) {
	return m.handleVMOpResult(msg.err)
}

func (m Model) handleVMOpResult(err error) (Model, tea.Cmd) {
	m.operation = ""
	if err != nil {
		m.vmErr = err
		return m, nil
	}
	m.loading = true
	return m, fetchVMInfo(m.cfg)
}

func (m Model) handleAgentAdded(msg agentAddedMsg) (Model, tea.Cmd) {
	m.operation = ""
	if msg.err != nil {
		m.agentsErr = msg.err
		return m, nil
	}
	m.agentsLoading = true
	return m, fetchAgents(m.cfg, m.vmInfo, m.sessionName())
}

func (m Model) handleAgentKilled(msg agentKilledMsg) (Model, tea.Cmd) {
	m.operation = ""
	if msg.err != nil {
		m.agentsErr = msg.err
		return m, nil
	}
	m.agentsLoading = true
	if m.agents != nil && m.selectedAgentIdx >= len(m.agents.Sessions)-1 {
		m.selectedAgentIdx = max(0, len(m.agents.Sessions)-2)
	}
	return m, fetchAgents(m.cfg, m.vmInfo, m.sessionName())
}

func (m Model) handleConnectFinished(msg connectFinishedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		if exitErr, ok := msg.err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			m.agentsErr = msg.err
		}
	}
	m.agentsLoading = true
	return m, fetchAgents(m.cfg, m.vmInfo, m.sessionName())
}

func (m Model) handleSync(msg syncMsg) (Model, tea.Cmd) {
	m.operation = ""
	if msg.err != nil {
		m.agentsErr = msg.err
		return m, nil
	}
	if msg.workspace != nil {
		m.workspaceInfo = msg.workspace
	}
	// Refresh agent list using the (possibly updated) session name
	m.agentsLoading = true
	return m, fetchAgents(m.cfg, m.vmInfo, m.sessionName())
}

// canModifyAgents returns true if agent add/kill operations are allowed.
func (m Model) canModifyAgents() bool {
	return m.cfg != nil && m.cfgErr == nil && m.vmInfo != nil &&
		m.vmInfo.Status == cloud.VMStatusRunning && m.operation == "" &&
		!m.agentsLoading && !m.provisionLoading &&
		provisioning.IsProvisioningComplete(m.provisionStatus)
}

// handleRefreshTick handles the periodic always-on refresh timer.
func (m Model) handleRefreshTick() (Model, tea.Cmd) {
	if m.cfg == nil || m.cfgErr != nil {
		return m, nil
	}
	if m.autoRefreshPaused {
		return m, scheduleRefresh(m.cfg)
	}
	if !m.loading && !m.refreshing {
		m.refreshing = true
		return m, tea.Batch(fetchVMInfo(m.cfg), scheduleRefresh(m.cfg))
	}
	// Already loading/refreshing, just reschedule for later
	return m, scheduleRefresh(m.cfg)
}
