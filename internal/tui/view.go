package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
	"github.com/cloud-coop/cloudcoop/internal/version"
)

// renderHeader returns the title and subtitle lines.
func (m Model) renderHeader() string {
	title := titleStyle.Render("cloudcoop") + " " + versionStyle.Render(version.Short())
	subtitle := subtitleStyle.Render("Manage sandboxed AI coding agents on cloud VMs")
	return fmt.Sprintf("\n%s\n%s\n", title, subtitle)
}

// renderFooter returns the help line using the help bubble.
func (m Model) renderFooter() string {
	return "\n" + m.help.View(m.keys) + "\n"
}

// ensureInit lazily initialises key bindings and help model for
// Models created without calling New() (e.g., in tests).
func (m *Model) ensureInit() {
	if m.keys.Quit.Keys() == nil {
		m.keys = newKeyMap()
		m.help = help.New()
		m.help.ShortSeparator = " · "
	}
}

// renderHelp returns the contextual help text for the current state.
func (m *Model) renderHelp() string {
	m.ensureInit()
	m.updateKeyStates()
	if m.selectingSize || m.confirmingDelete || m.confirmingKill {
		return m.dialogueHelp()
	}
	return m.help.View(m.keys)
}

// buildContent returns the main content for the current model state.
func (m Model) buildContent() string {
	if m.showHelp {
		return m.renderHelpOverlay()
	}

	switch {
	case m.cfgErr != nil:
		return m.renderConfigError()
	case m.selectingSize:
		return m.renderSizeSelection()
	case m.confirmingDelete:
		return m.renderDeleteConfirmation()
	case m.confirmingKill:
		return m.renderKillConfirmation()
	case m.operation != "":
		return m.renderOperation()
	case m.loading && m.vmInfo == nil:
		return boxStyle.Render(m.spinner.View() + " Loading VM status...")
	case m.vmErr != nil:
		return boxStyle.Render(errorStyle.Render(fmt.Sprintf("Error: %v", m.vmErr)))
	default:
		return m.renderVMStatus()
	}
}

// syncViewport updates the viewport content from current model state.
func (m *Model) syncViewport() {
	m.ensureInit()
	m.updateKeyStates()
	m.viewport.SetContent(m.buildContent())
}

func (m Model) renderView() string {
	if !m.ready {
		return "Loading..."
	}

	header := m.renderHeader()
	content := m.buildContent()

	// Use viewport when sized (production); render directly otherwise (tests).
	if m.viewport.Width > 0 {
		content = m.viewport.View()
	}

	if m.selectingSize || m.confirmingDelete || m.confirmingKill {
		footer := "\n" + helpStyle.Render(m.dialogueHelp()) + "\n"
		return header + "\n" + content + footer
	}

	return header + "\n" + content + m.renderFooter()
}

// dialogueHelp returns context-specific help text for dialogue modes.
func (m Model) dialogueHelp() string {
	if m.selectingSize {
		return "↑/↓: select · Enter: create · Esc: cancel"
	}
	return "y: confirm · n: cancel"
}

func (m Model) renderOperation() string {
	msgs := map[string]string{
		"starting": "Starting VM...",
		"stopping": "Stopping VM...",
		"creating": "Creating VM...",
		"deleting": "Deleting VM...",
		"adding":   "Adding agent...",
		"killing":  "Killing agent...",
		"syncing":  "Syncing workspace...",
	}
	return boxStyle.Render(m.spinner.View() + " " + msgs[m.operation])
}

func (m Model) renderConfigError() string {
	msg := fmt.Sprintf(`Configuration Error

%s

To get started, run:

  cloudcoop setup`, m.cfgErr)
	return boxStyle.Render(errorStyle.Render(msg))
}

func (m Model) renderVMStatus() string {
	if m.vmInfo == nil {
		return boxStyle.Render("No VM info available")
	}

	info := m.vmInfo
	lines := []string{labelLine("Cloud:", m.cfg.Cloud.Provider)}

	if m.cfg.Cloud.Provider == "gcp" {
		lines = append(lines, labelLine("Project:", m.cfg.Cloud.GCP.Project))
	}
	if m.workspaceInfo != nil {
		lines = append(lines, labelLine("Workspace:", m.workspaceInfo.Slug))
	}
	lines = append(lines, "")

	if info.Status == cloud.VMStatusNotFound {
		lines = append(lines, labelLine("VM:", stoppedStyle.Render(info.Name+" (not found)")))
		lines = append(lines, "", stoppedStyle.Render("VM does not exist. Create it in GCP Console."))
	} else {
		lines = append(lines,
			labelLine("VM:", info.Name),
			labelLine("Status:", m.formatStatus(info.Status)),
			labelLine("Zone:", info.Zone),
			labelLine("Machine Type:", info.MachineType))
		if info.ExternalIP != "" {
			lines = append(lines, labelLine("External IP:", info.ExternalIP))
		}
		if info.InternalIP != "" {
			lines = append(lines, labelLine("Internal IP:", info.InternalIP))
		}
		if info.Status == cloud.VMStatusRunning {
			lines = append(lines, labelLine("Provisioning:", m.formatProvisionStatus()))
		}
	}

	lines = append(lines, "")
	lines = append(lines, m.renderAgents()...)
	return boxStyle.Render(strings.Join(lines, "\n"))
}

func labelLine(label, value string) string {
	return labelStyle.Render(label) + value
}

func (m Model) formatStatus(status cloud.VMStatus) string {
	switch status {
	case cloud.VMStatusRunning:
		return runningStyle.Render("● running")
	case cloud.VMStatusStopped:
		return stoppedStyle.Render("○ stopped")
	case cloud.VMStatusStarting:
		return m.spinner.View() + " starting..."
	case cloud.VMStatusStopping:
		return m.spinner.View() + " stopping..."
	default:
		return string(status)
	}
}

func (m Model) formatProvisionStatus() string {
	if m.provisionLoading && m.provisionStatus == nil {
		return m.spinner.View() + " checking..."
	}
	if m.provisionErr != nil {
		return errorStyle.Render(fmt.Sprintf("error: %v", m.provisionErr))
	}
	if m.provisionStatus == nil {
		return stoppedStyle.Render("(unknown)")
	}

	switch m.provisionStatus.Status {
	case provisioning.StatusPending:
		return stoppedStyle.Render("○ pending")
	case provisioning.StatusRunning:
		if m.provisionStatus.Progress != "" {
			return m.spinner.View() + " " + m.provisionStatus.Progress
		}
		return m.spinner.View() + " running"
	case provisioning.StatusCompleted:
		return runningStyle.Render("● completed")
	case provisioning.StatusFailed:
		if m.provisionStatus.Error != "" {
			return errorStyle.Render(fmt.Sprintf("✗ failed: %s", m.provisionStatus.Error))
		}
		return errorStyle.Render("✗ failed")
	default:
		return stoppedStyle.Render("? unknown")
	}
}

func (m Model) renderAgents() []string {
	if m.vmInfo == nil || m.vmInfo.Status != cloud.VMStatusRunning {
		return []string{labelLine("Agents:", stoppedStyle.Render("(VM not running)"))}
	}
	if m.agentsLoading && m.agents == nil {
		return []string{labelLine("Agents:", m.spinner.View()+" loading...")}
	}
	if m.agentsErr != nil {
		if errors.Is(m.agentsErr, agent.ErrTmuxNotInstalled) {
			return []string{labelLine("Agents:", stoppedStyle.Render("tmux not installed"))}
		}
		return []string{labelLine("Agents:", errorStyle.Render(fmt.Sprintf("error: %v", m.agentsErr)))}
	}
	if m.agents == nil {
		return []string{labelLine("Agents:", stoppedStyle.Render("(unknown)"))}
	}
	if m.agents.NoSession {
		return []string{labelLine("Agents:", stoppedStyle.Render("no session"))}
	}
	if len(m.agents.Sessions) == 0 {
		return []string{labelLine("Agents:", stoppedStyle.Render("0 running"))}
	}

	lines := []string{labelLine("Agents:", runningStyle.Render(fmt.Sprintf("%d running", len(m.agents.Sessions))))}
	for i, s := range m.agents.Sessions {
		sel := "  "
		if i == m.selectedAgentIdx {
			sel = "> "
		}
		idx := ""
		if i < 9 {
			idx = fmt.Sprintf("[%d] ", i+1)
		}
		lines = append(lines, fmt.Sprintf("%s%s%s%s (%s)", labelStyle.Render(""), sel, idx, s.Name, s.Command))
	}
	return lines
}

func (m Model) renderKillConfirmation() string {
	return boxStyle.Render(fmt.Sprintf("Kill agent %q (index %d)?\n\nPress y to confirm, n to cancel.",
		m.killTargetName, m.killTargetIndex))
}

func (m Model) renderSizeSelection() string {
	lines := []string{"Select VM size:\n"}
	for i, size := range m.sizeOptions {
		sel := "  "
		if i == m.selectedSizeIdx {
			sel = "> "
		}
		lines = append(lines, fmt.Sprintf("%s%s (%s)", sel, size, m.cfg.VM.MachineSizes[size]))
	}
	lines = append(lines, "\n↑/↓: select • Enter: create • Esc: cancel")
	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderDeleteConfirmation() string {
	return boxStyle.Render(fmt.Sprintf("Delete VM %q?\n\nThis will permanently delete the VM and its boot disk.\n\nPress y to confirm, n to cancel.",
		m.cfg.VM.Name))
}

func (m Model) renderHelpOverlay() string {
	help := `Keyboard Shortcuts

  Navigation
    k/↑         Move up
    j/↓         Move down

  VM Management
    s           Start VM
    t           Stop VM
    n           New VM (create)
    d           Delete VM (when stopped)

  Agent Management
    +           Add agent
    -           Kill agent (with confirmation)
    c/Enter     Connect to selected agent
    1-9         Connect to agent by index
    w           Sync workspace worktrees

  General
    r           Refresh
    a           Toggle auto-refresh pause
    ?           Toggle this help
    Esc         Close help
    q           Quit

Press ? or Esc to dismiss.`
	return boxStyle.Render(help)
}
