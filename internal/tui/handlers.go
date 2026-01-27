package tui

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
)

// handleKeyMsg dispatches keyboard input to the appropriate handler.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
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
	switch msg.String() {
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
		return m, createVM(m.cfg, machineType)
	case "esc", "escape", "n", "N":
		m.selectingSize = false
	}
	return m, nil
}

func (m Model) handleDeleteConfirmationKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirmingDelete = false
		m.operation = "deleting"
		return m, deleteVM(m.cfg)
	case "n", "N", "esc", "escape":
		m.confirmingDelete = false
	}
	return m, nil
}

func (m Model) handleKillConfirmationKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirmingKill = false
		m.operation = "killing"
		return m, killAgent(m.cfg, m.vmInfo, m.killTargetIndex)
	case "n", "N", "esc", "escape":
		m.confirmingKill = false
	}
	return m, nil
}

func (m Model) handleNormalKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
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
	case "s", "S":
		if m.canVMOp() && m.vmInfo.Status == cloud.VMStatusStopped {
			m.operation = "starting"
			return m, startVM(m.cfg)
		}
	case "t", "T":
		if m.canVMOp() && m.vmInfo.Status == cloud.VMStatusRunning {
			m.operation = "stopping"
			return m, stopVM(m.cfg)
		}
	case "a", "A":
		if m.canModifyAgents() {
			m.operation = "adding"
			return m, addAgent(m.cfg, m.vmInfo)
		}
	case "K":
		if m.canModifyAgents() && m.hasAgents() && m.selectedAgentIdx < len(m.agents.Sessions) {
			selected := m.agents.Sessions[m.selectedAgentIdx]
			m.killTargetIndex = selected.Index
			m.killTargetName = selected.Name
			m.confirmingKill = true
		}
	case "c":
		if m.canModifyAgents() && m.hasAgents() && m.selectedAgentIdx < len(m.agents.Sessions) {
			return m, connectToAgent(m.cfg, m.vmInfo, m.agents.Sessions[m.selectedAgentIdx].Index)
		}
	case "C":
		if m.canVMOp() && m.vmInfo.Status == cloud.VMStatusNotFound {
			m.sizeOptions = []string{"small", "medium", "large", "xlarge"}
			m.selectedSizeIdx = 0
			m.selectingSize = true
		}
	case "D":
		if m.canVMOp() && m.vmInfo.Status == cloud.VMStatusStopped {
			m.confirmingDelete = true
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
	if msg.err != nil {
		m.loading = false
		return m, nil
	}
	if err := m.cfg.Validate(); err != nil {
		m.cfgErr = err
		m.loading = false
		return m, nil
	}
	return m, fetchVMInfo(m.cfg)
}

func (m Model) handleVMInfo(msg vmInfoMsg) (Model, tea.Cmd) {
	m.loading = false
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
		return m, tea.Batch(fetchAgents(m.cfg, msg.info), fetchProvisionStatus(m.cfg, msg.info))
	}
	m.agents = nil
	m.agentsErr = nil
	m.provisionStatus = nil
	m.provisionErr = nil
	return m, nil
}

func (m Model) handleAgents(msg agentsMsg) (Model, tea.Cmd) {
	m.agentsLoading = false
	m.agents = msg.result
	m.agentsErr = msg.err
	return m, nil
}

func (m Model) handleProvisionStatus(msg provisionStatusMsg) (Model, tea.Cmd) {
	m.provisionLoading = false
	m.provisionStatus = msg.status
	m.provisionErr = msg.err
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
	return m, fetchAgents(m.cfg, m.vmInfo)
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
	return m, fetchAgents(m.cfg, m.vmInfo)
}

func (m Model) handleConnectFinished(msg connectFinishedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		if exitErr, ok := msg.err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			m.agentsErr = msg.err
		}
	}
	m.agentsLoading = true
	return m, fetchAgents(m.cfg, m.vmInfo)
}

// canModifyAgents returns true if agent add/kill operations are allowed.
func (m Model) canModifyAgents() bool {
	return m.cfg != nil && m.cfgErr == nil && m.vmInfo != nil &&
		m.vmInfo.Status == cloud.VMStatusRunning && m.operation == "" &&
		!m.agentsLoading && !m.provisionLoading &&
		provisioning.IsProvisioningComplete(m.provisionStatus)
}
