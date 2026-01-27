package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
)

func (m Model) renderView() string {
	if !m.ready {
		return "Loading..."
	}

	title := titleStyle.Render("cloudcoop")
	subtitle := subtitleStyle.Render("Manage sandboxed AI coding agents on cloud VMs")

	var content string
	switch {
	case m.cfgErr != nil:
		content = m.renderConfigError()
	case m.selectingSize:
		content = m.renderSizeSelection()
	case m.confirmingDelete:
		content = m.renderDeleteConfirmation()
	case m.confirmingKill:
		content = m.renderKillConfirmation()
	case m.operation != "":
		content = m.renderOperation()
	case m.loading:
		content = boxStyle.Render("Loading VM status...")
	case m.vmErr != nil:
		content = boxStyle.Render(errorStyle.Render(fmt.Sprintf("Error: %v", m.vmErr)))
	default:
		content = m.renderVMStatus()
	}

	return fmt.Sprintf("\n%s\n%s\n\n%s\n\n%s\n", title, subtitle, content, m.renderHelp())
}

func (m Model) renderOperation() string {
	msgs := map[string]string{
		"starting": "Starting VM... ◐",
		"stopping": "Stopping VM... ◑",
		"creating": "Creating VM... ◐",
		"deleting": "Deleting VM... ◑",
		"adding":   "Adding agent... ◐",
		"killing":  "Killing agent... ◑",
	}
	return boxStyle.Render(msgs[m.operation])
}

func (m Model) renderConfigError() string {
	msg := fmt.Sprintf(`Configuration Error

%s

To get started, run the setup wizard:

  cloudcoop config init

Or create a config file manually at:
  ~/.config/cloudcoop/cloudcoop.toml

Example:
  [cloud]
  provider = "gcp"

  [cloud.gcp]
  project = "your-project-id"
  zone = "us-central1-a"

  [vm]
  name = "your-vm-name"`, m.cfgErr)
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
		return "◐ starting..."
	case cloud.VMStatusStopping:
		return "◑ stopping..."
	default:
		return string(status)
	}
}

func (m Model) formatProvisionStatus() string {
	if m.provisionLoading {
		return "checking..."
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
			return fmt.Sprintf("◐ %s", m.provisionStatus.Progress)
		}
		return "◐ running"
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
	if m.agentsLoading {
		return []string{labelLine("Agents:", "loading...")}
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
		lines = append(lines, fmt.Sprintf("%s%s%s (%s)", labelStyle.Render(""), sel, s.Name, s.Command))
	}
	return lines
}

func (m Model) renderKillConfirmation() string {
	return boxStyle.Render(fmt.Sprintf("Kill agent %q (index %d)?\n\nPress Y to confirm, N to cancel.",
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
	return boxStyle.Render(fmt.Sprintf("Delete VM %q?\n\nThis will permanently delete the VM and its boot disk.\n\nPress Y to confirm, N to cancel.",
		m.cfg.VM.Name))
}

func (m Model) renderHelp() string {
	if m.selectingSize {
		return helpStyle.Render("↑/↓: select • Enter: create • Esc: cancel")
	}
	if m.confirmingDelete || m.confirmingKill {
		return helpStyle.Render("Y: confirm • N: cancel")
	}

	actions := []string{"q: quit", "r: refresh"}
	if m.vmInfo != nil && m.operation == "" {
		switch m.vmInfo.Status {
		case cloud.VMStatusNotFound:
			actions = append(actions, "C: create")
		case cloud.VMStatusStopped:
			actions = append(actions, "S: start", "D: delete")
		case cloud.VMStatusRunning:
			actions = append(actions, "T: stop")
			if m.canModifyAgents() {
				actions = append(actions, "A: add agent")
				if m.agents != nil && len(m.agents.Sessions) > 0 {
					actions = append(actions, "c: connect", "K: kill agent", "↑/↓: select")
				}
			}
		}
	}
	return helpStyle.Render(strings.Join(actions, " • "))
}
