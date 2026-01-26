// Package tui provides the terminal user interface for cloudcoop.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/cloud/gcp"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// Styles for the TUI.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginBottom(2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	stoppedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Width(14)
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

// configLoadedMsg is sent when config loading completes.
type configLoadedMsg struct {
	cfg *config.Config
	err error
}

// vmInfoMsg is sent when VM info query completes.
type vmInfoMsg struct {
	info    *cloud.VMInfo
	err     error
	cleanup func()
}

// vmStartMsg is sent when VM start completes.
type vmStartMsg struct {
	err error
}

// vmStopMsg is sent when VM stop completes.
type vmStopMsg struct {
	err error
}

// vmCreateMsg is sent when VM create completes.
type vmCreateMsg struct {
	err error
}

// vmDeleteMsg is sent when VM delete completes.
type vmDeleteMsg struct {
	err error
}

// agentsMsg is sent when agent listing completes.
type agentsMsg struct {
	result *agent.ListResult
	err    error
}

// agentAddedMsg is sent when agent creation completes.
type agentAddedMsg struct {
	session *agent.Session
	err     error
}

// agentKilledMsg is sent when agent kill completes.
type agentKilledMsg struct {
	index int
	err   error
}

// loadConfig loads the configuration file.
func loadConfig() tea.Msg {
	cfg, err := config.Load()
	return configLoadedMsg{cfg: cfg, err: err}
}

// fetchVMInfo queries the cloud provider for VM status.
func fetchVMInfo(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var provider cloud.Provider
		var cleanup func()

		switch cfg.Cloud.Provider {
		case "gcp":
			p, err := gcp.New(ctx, cfg.Cloud.GCP.Project, cfg.Cloud.GCP.Zone)
			if err != nil {
				return vmInfoMsg{err: fmt.Errorf("create GCP provider: %w", err)}
			}
			provider = p
			cleanup = func() { _ = p.Close() }
		default:
			return vmInfoMsg{err: fmt.Errorf("unsupported provider: %s", cfg.Cloud.Provider)}
		}

		info, err := provider.GetVMInfo(ctx, cfg.VM.Name)
		if err != nil {
			cleanup()
			return vmInfoMsg{err: fmt.Errorf("get VM info: %w", err)}
		}

		return vmInfoMsg{info: info, cleanup: cleanup}
	}
}

// startVM starts the VM asynchronously.
func startVM(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		var provider cloud.Provider
		var cleanup func()

		switch cfg.Cloud.Provider {
		case "gcp":
			p, err := gcp.New(ctx, cfg.Cloud.GCP.Project, cfg.Cloud.GCP.Zone)
			if err != nil {
				return vmStartMsg{err: fmt.Errorf("create GCP provider: %w", err)}
			}
			provider = p
			cleanup = func() { _ = p.Close() }
		default:
			return vmStartMsg{err: fmt.Errorf("unsupported provider: %s", cfg.Cloud.Provider)}
		}
		defer cleanup()

		if err := provider.StartVM(ctx, cfg.VM.Name); err != nil {
			return vmStartMsg{err: fmt.Errorf("start VM: %w", err)}
		}

		return vmStartMsg{}
	}
}

// stopVM stops the VM asynchronously.
func stopVM(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		var provider cloud.Provider
		var cleanup func()

		switch cfg.Cloud.Provider {
		case "gcp":
			p, err := gcp.New(ctx, cfg.Cloud.GCP.Project, cfg.Cloud.GCP.Zone)
			if err != nil {
				return vmStopMsg{err: fmt.Errorf("create GCP provider: %w", err)}
			}
			provider = p
			cleanup = func() { _ = p.Close() }
		default:
			return vmStopMsg{err: fmt.Errorf("unsupported provider: %s", cfg.Cloud.Provider)}
		}
		defer cleanup()

		if err := provider.StopVM(ctx, cfg.VM.Name); err != nil {
			return vmStopMsg{err: fmt.Errorf("stop VM: %w", err)}
		}

		return vmStopMsg{}
	}
}

// createVM creates a new VM with the specified machine type.
func createVM(cfg *config.Config, machineType string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		var provider cloud.Provider
		var cleanup func()

		switch cfg.Cloud.Provider {
		case "gcp":
			p, err := gcp.New(ctx, cfg.Cloud.GCP.Project, cfg.Cloud.GCP.Zone)
			if err != nil {
				return vmCreateMsg{err: fmt.Errorf("create GCP provider: %w", err)}
			}
			provider = p
			cleanup = func() { _ = p.Close() }
		default:
			return vmCreateMsg{err: fmt.Errorf("unsupported provider: %s", cfg.Cloud.Provider)}
		}
		defer cleanup()

		createCfg := cloud.VMCreateConfig{
			Name:        cfg.VM.Name,
			MachineType: machineType,
			DiskSizeGB:  cfg.VM.DiskSizeGB,
			Image:       cfg.VM.Image,
			Spot:        cfg.VM.Spot,
			Network:     cfg.VM.Network,
			Tags:        cfg.VM.Tags,
			SSHPort:     cfg.SSH.Port,
		}

		if err := provider.CreateVM(ctx, createCfg); err != nil {
			return vmCreateMsg{err: fmt.Errorf("create VM: %w", err)}
		}

		return vmCreateMsg{}
	}
}

// deleteVM deletes the configured VM.
func deleteVM(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		var provider cloud.Provider
		var cleanup func()

		switch cfg.Cloud.Provider {
		case "gcp":
			p, err := gcp.New(ctx, cfg.Cloud.GCP.Project, cfg.Cloud.GCP.Zone)
			if err != nil {
				return vmDeleteMsg{err: fmt.Errorf("create GCP provider: %w", err)}
			}
			provider = p
			cleanup = func() { _ = p.Close() }
		default:
			return vmDeleteMsg{err: fmt.Errorf("unsupported provider: %s", cfg.Cloud.Provider)}
		}
		defer cleanup()

		if err := provider.DeleteVM(ctx, cfg.VM.Name); err != nil {
			return vmDeleteMsg{err: fmt.Errorf("delete VM: %w", err)}
		}

		return vmDeleteMsg{}
	}
}

// fetchAgents queries the VM for running agent sessions.
func fetchAgents(cfg *config.Config, vmInfo *cloud.VMInfo) tea.Cmd {
	return func() tea.Msg {
		// Resolve SSH connection parameters using helpers
		ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
		if err != nil {
			return agentsMsg{err: fmt.Errorf("no IP address available")}
		}

		sshUser := ssh.ResolveSSHUser(cfg.SSH.User)

		// Connect via SSH
		client, err := ssh.NewClient(ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port))
		if err != nil {
			return agentsMsg{err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		// List agent sessions
		result, err := agent.ListSessions(client)
		if err != nil {
			return agentsMsg{err: err}
		}

		return agentsMsg{result: result}
	}
}

// addAgent creates a new agent session on the VM.
func addAgent(cfg *config.Config, vmInfo *cloud.VMInfo) tea.Cmd {
	return func() tea.Msg {
		// Resolve SSH connection parameters using helpers
		ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
		if err != nil {
			return agentAddedMsg{err: fmt.Errorf("no IP address available")}
		}

		sshUser := ssh.ResolveSSHUser(cfg.SSH.User)

		// Connect via SSH
		client, err := ssh.NewClient(ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port))
		if err != nil {
			return agentAddedMsg{err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		// Create agent session with config defaults
		opts := agent.CreateSessionOptions{
			Command: cfg.Agents.DefaultCommand, // Uses config default if set
		}

		session, err := agent.CreateSession(client, opts)
		if err != nil {
			return agentAddedMsg{err: err}
		}

		return agentAddedMsg{session: session}
	}
}

// connectToAgent creates a command to connect to an agent session interactively.
// It uses tea.ExecProcess to shell out to SSH and resume the TUI after.
func connectToAgent(cfg *config.Config, vmInfo *cloud.VMInfo, windowIndex int) tea.Cmd {
	// Resolve SSH connection parameters using helpers
	ip, _ := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	port := ssh.ResolvePort(cfg.SSH.Port)

	// Build the tmux attach command
	tmuxCmd := fmt.Sprintf("tmux select-window -t agents:%d && tmux attach -t agents", windowIndex)

	// Build SSH command with -t for PTY allocation
	c := exec.Command("ssh", "-t",
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("%s@%s", sshUser, ip),
		tmuxCmd,
	)

	// Use tea.ExecProcess to run the command and resume TUI after
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return connectFinishedMsg{err: err}
	})
}

// connectFinishedMsg is sent when an interactive connect session ends.
type connectFinishedMsg struct {
	err error
}

// killAgent kills an agent session on the VM.
func killAgent(cfg *config.Config, vmInfo *cloud.VMInfo, index int) tea.Cmd {
	return func() tea.Msg {
		// Resolve SSH connection parameters using helpers
		ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
		if err != nil {
			return agentKilledMsg{index: index, err: fmt.Errorf("no IP address available")}
		}

		sshUser := ssh.ResolveSSHUser(cfg.SSH.User)

		// Connect via SSH
		client, err := ssh.NewClient(ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port))
		if err != nil {
			return agentKilledMsg{index: index, err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		// Kill agent session (force=true since user confirmed)
		opts := agent.KillSessionOptions{
			Index: index,
			Force: true,
		}

		err = agent.KillSession(client, opts)
		if err != nil {
			return agentKilledMsg{index: index, err: err}
		}

		return agentKilledMsg{index: index}
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
		// Handle size selection dialog
		if m.selectingSize {
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
				// Create VM with selected size
				m.selectingSize = false
				sizeName := m.sizeOptions[m.selectedSizeIdx]
				machineType := m.cfg.VM.MachineSizes[sizeName]
				m.operation = "creating"
				return m, createVM(m.cfg, machineType)
			case "esc", "escape", "n", "N":
				// Cancel size selection
				m.selectingSize = false
				return m, nil
			}
			return m, nil
		}

		// Handle delete confirmation dialog
		if m.confirmingDelete {
			switch msg.String() {
			case "y", "Y":
				// Confirm delete
				m.confirmingDelete = false
				m.operation = "deleting"
				return m, deleteVM(m.cfg)
			case "n", "N", "esc", "escape":
				// Cancel delete
				m.confirmingDelete = false
				return m, nil
			}
			return m, nil
		}

		// Handle kill confirmation dialog
		if m.confirmingKill {
			switch msg.String() {
			case "y", "Y":
				// Confirm kill
				m.confirmingKill = false
				m.operation = "killing"
				return m, killAgent(m.cfg, m.vmInfo, m.killTargetIndex)
			case "n", "N", "esc", "escape":
				// Cancel kill
				m.confirmingKill = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			if m.cleanup != nil {
				m.cleanup()
			}
			return m, tea.Quit
		case "r":
			// Refresh VM status
			if m.cfg != nil && m.cfgErr == nil && m.operation == "" {
				m.loading = true
				return m, fetchVMInfo(m.cfg)
			}
		case "s", "S":
			// Start VM (only when stopped and no operation in progress)
			if m.cfg != nil && m.cfgErr == nil && m.vmInfo != nil &&
				m.vmInfo.Status == cloud.VMStatusStopped && m.operation == "" {
				m.operation = "starting"
				return m, startVM(m.cfg)
			}
		case "t", "T":
			// Stop VM (only when running and no operation in progress)
			if m.cfg != nil && m.cfgErr == nil && m.vmInfo != nil &&
				m.vmInfo.Status == cloud.VMStatusRunning && m.operation == "" {
				m.operation = "stopping"
				return m, stopVM(m.cfg)
			}
		case "a", "A":
			// Add agent (only when VM is running and no operation in progress)
			if m.canModifyAgents() {
				m.operation = "adding"
				return m, addAgent(m.cfg, m.vmInfo)
			}
		case "K":
			// Kill agent (only when VM is running, agents exist, and no operation in progress)
			// Note: uppercase K only, to avoid conflict with vim navigation
			if m.canModifyAgents() && m.agents != nil && len(m.agents.Sessions) > 0 {
				// Get the selected agent
				if m.selectedAgentIdx < len(m.agents.Sessions) {
					selected := m.agents.Sessions[m.selectedAgentIdx]
					m.killTargetIndex = selected.Index
					m.killTargetName = selected.Name
					m.confirmingKill = true
				}
			}
		case "c":
			// Connect to selected agent (only when VM is running, agents exist, and no operation in progress)
			if m.canModifyAgents() && m.agents != nil && len(m.agents.Sessions) > 0 {
				if m.selectedAgentIdx < len(m.agents.Sessions) {
					selected := m.agents.Sessions[m.selectedAgentIdx]
					return m, connectToAgent(m.cfg, m.vmInfo, selected.Index)
				}
			}
		case "C":
			// Create VM (only when VM not found and no operation in progress)
			if m.cfg != nil && m.cfgErr == nil && m.vmInfo != nil &&
				m.vmInfo.Status == cloud.VMStatusNotFound && m.operation == "" {
				// Build size options from config
				m.sizeOptions = []string{"small", "medium", "large", "xlarge"}
				m.selectedSizeIdx = 0
				m.selectingSize = true
				return m, nil
			}
		case "D":
			// Delete VM (only when stopped and no operation in progress)
			if m.cfg != nil && m.cfgErr == nil && m.vmInfo != nil &&
				m.vmInfo.Status == cloud.VMStatusStopped && m.operation == "" {
				m.confirmingDelete = true
				return m, nil
			}
		case "up":
			// Move selection up
			if m.agents != nil && len(m.agents.Sessions) > 0 && m.selectedAgentIdx > 0 {
				m.selectedAgentIdx--
			}
		case "down", "j", "k":
			// Move selection down (j = vim down, k = vim up but we handle it as down to avoid confusion)
			// For cleaner implementation, only up/down arrows for selection
			if msg.String() == "down" || msg.String() == "j" {
				if m.agents != nil && len(m.agents.Sessions) > 0 && m.selectedAgentIdx < len(m.agents.Sessions)-1 {
					m.selectedAgentIdx++
				}
			} else if msg.String() == "k" {
				// k moves up (vim style)
				if m.agents != nil && len(m.agents.Sessions) > 0 && m.selectedAgentIdx > 0 {
					m.selectedAgentIdx--
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case configLoadedMsg:
		m.cfg = msg.cfg
		m.cfgErr = msg.err
		if msg.err != nil {
			m.loading = false
			return m, nil
		}
		// Config loaded, now fetch VM info
		if err := m.cfg.Validate(); err != nil {
			m.cfgErr = err
			m.loading = false
			return m, nil
		}
		return m, fetchVMInfo(m.cfg)

	case vmInfoMsg:
		m.loading = false
		m.vmInfo = msg.info
		m.vmErr = msg.err
		if msg.cleanup != nil {
			// Store cleanup for later
			if m.cleanup != nil {
				m.cleanup()
			}
			m.cleanup = msg.cleanup
		}
		// Fetch agents if VM is running
		if msg.err == nil && msg.info != nil && msg.info.Status == cloud.VMStatusRunning {
			m.agentsLoading = true
			m.agents = nil
			m.agentsErr = nil
			return m, fetchAgents(m.cfg, msg.info)
		}
		// Clear agents if VM not running
		m.agents = nil
		m.agentsErr = nil

	case agentsMsg:
		m.agentsLoading = false
		m.agents = msg.result
		m.agentsErr = msg.err

	case vmStartMsg:
		m.operation = ""
		if msg.err != nil {
			m.vmErr = msg.err
			return m, nil
		}
		// Refresh VM status after successful start
		m.loading = true
		return m, fetchVMInfo(m.cfg)

	case vmStopMsg:
		m.operation = ""
		if msg.err != nil {
			m.vmErr = msg.err
			return m, nil
		}
		// Refresh VM status after successful stop
		m.loading = true
		return m, fetchVMInfo(m.cfg)

	case vmCreateMsg:
		m.operation = ""
		if msg.err != nil {
			m.vmErr = msg.err
			return m, nil
		}
		// Refresh VM status after successful create
		m.loading = true
		return m, fetchVMInfo(m.cfg)

	case vmDeleteMsg:
		m.operation = ""
		if msg.err != nil {
			m.vmErr = msg.err
			return m, nil
		}
		// Refresh VM status after successful delete
		m.loading = true
		return m, fetchVMInfo(m.cfg)

	case agentAddedMsg:
		m.operation = ""
		if msg.err != nil {
			m.agentsErr = msg.err
			return m, nil
		}
		// Refresh agents list after successful add
		m.agentsLoading = true
		return m, fetchAgents(m.cfg, m.vmInfo)

	case agentKilledMsg:
		m.operation = ""
		if msg.err != nil {
			m.agentsErr = msg.err
			return m, nil
		}
		// Refresh agents list after successful kill
		m.agentsLoading = true
		// Reset selection if it would be out of bounds
		if m.agents != nil && m.selectedAgentIdx >= len(m.agents.Sessions)-1 {
			m.selectedAgentIdx = max(0, len(m.agents.Sessions)-2)
		}
		return m, fetchAgents(m.cfg, m.vmInfo)

	case connectFinishedMsg:
		// Connection ended - refresh agents list in case state changed
		if msg.err != nil {
			// Don't show error for normal disconnect
			// Only show errors that indicate actual problems
			if exitErr, ok := msg.err.(*exec.ExitError); ok {
				if exitErr.ExitCode() != 0 {
					m.agentsErr = msg.err
				}
			}
		}
		// Refresh agents list after returning from connect
		m.agentsLoading = true
		return m, fetchAgents(m.cfg, m.vmInfo)
	}

	return m, nil
}

// canModifyAgents returns true if agent add/kill operations are allowed.
func (m Model) canModifyAgents() bool {
	return m.cfg != nil &&
		m.cfgErr == nil &&
		m.vmInfo != nil &&
		m.vmInfo.Status == cloud.VMStatusRunning &&
		m.operation == "" &&
		!m.agentsLoading
}

// View renders the TUI.
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	title := titleStyle.Render("cloudcoop")
	subtitle := subtitleStyle.Render("Manage sandboxed AI coding agents on cloud VMs")

	var content string
	if m.cfgErr != nil {
		content = m.renderConfigError()
	} else if m.selectingSize {
		content = m.renderSizeSelection()
	} else if m.confirmingDelete {
		content = m.renderDeleteConfirmation()
	} else if m.confirmingKill {
		content = m.renderKillConfirmation()
	} else if m.operation == "starting" {
		content = boxStyle.Render("Starting VM... ◐")
	} else if m.operation == "stopping" {
		content = boxStyle.Render("Stopping VM... ◑")
	} else if m.operation == "creating" {
		content = boxStyle.Render("Creating VM... ◐")
	} else if m.operation == "deleting" {
		content = boxStyle.Render("Deleting VM... ◑")
	} else if m.operation == "adding" {
		content = boxStyle.Render("Adding agent... ◐")
	} else if m.operation == "killing" {
		content = boxStyle.Render("Killing agent... ◑")
	} else if m.loading {
		content = boxStyle.Render("Loading VM status...")
	} else if m.vmErr != nil {
		content = boxStyle.Render(errorStyle.Render(fmt.Sprintf("Error: %v", m.vmErr)))
	} else {
		content = m.renderVMStatus()
	}

	help := m.renderHelp()

	return fmt.Sprintf("\n%s\n%s\n\n%s\n\n%s\n",
		title,
		subtitle,
		content,
		help,
	)
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
	var lines []string

	// Cloud context
	lines = append(lines, fmt.Sprintf("%s%s",
		labelStyle.Render("Cloud:"),
		m.cfg.Cloud.Provider))

	if m.cfg.Cloud.Provider == "gcp" {
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("Project:"),
			m.cfg.Cloud.GCP.Project))
	}

	lines = append(lines, "")

	// VM info
	if info.Status == cloud.VMStatusNotFound {
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("VM:"),
			stoppedStyle.Render(info.Name+" (not found)")))
		lines = append(lines, "")
		lines = append(lines, stoppedStyle.Render("VM does not exist. Create it in GCP Console."))
	} else {
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("VM:"),
			info.Name))
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("Status:"),
			m.formatStatus(info.Status)))
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("Zone:"),
			info.Zone))
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("Machine Type:"),
			info.MachineType))

		if info.ExternalIP != "" {
			lines = append(lines, fmt.Sprintf("%s%s",
				labelStyle.Render("External IP:"),
				info.ExternalIP))
		}
		if info.InternalIP != "" {
			lines = append(lines, fmt.Sprintf("%s%s",
				labelStyle.Render("Internal IP:"),
				info.InternalIP))
		}
	}

	lines = append(lines, "")
	lines = append(lines, m.renderAgents()...)

	var result string
	for i, line := range lines {
		result += line
		if i < len(lines)-1 {
			result += "\n"
		}
	}

	return boxStyle.Render(result)
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

func (m Model) renderAgents() []string {
	var lines []string

	// Only show agents when VM is running
	if m.vmInfo == nil || m.vmInfo.Status != cloud.VMStatusRunning {
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("Agents:"),
			stoppedStyle.Render("(VM not running)")))
		return lines
	}

	if m.agentsLoading {
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("Agents:"),
			"loading..."))
		return lines
	}

	if m.agentsErr != nil {
		if errors.Is(m.agentsErr, agent.ErrTmuxNotInstalled) {
			lines = append(lines, fmt.Sprintf("%s%s",
				labelStyle.Render("Agents:"),
				stoppedStyle.Render("tmux not installed")))
		} else {
			lines = append(lines, fmt.Sprintf("%s%s",
				labelStyle.Render("Agents:"),
				errorStyle.Render(fmt.Sprintf("error: %v", m.agentsErr))))
		}
		return lines
	}

	if m.agents == nil {
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("Agents:"),
			stoppedStyle.Render("(unknown)")))
		return lines
	}

	if m.agents.NoSession {
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("Agents:"),
			stoppedStyle.Render("no session")))
		return lines
	}

	if len(m.agents.Sessions) == 0 {
		lines = append(lines, fmt.Sprintf("%s%s",
			labelStyle.Render("Agents:"),
			stoppedStyle.Render("0 running")))
		return lines
	}

	// Show agent count
	lines = append(lines, fmt.Sprintf("%s%s",
		labelStyle.Render("Agents:"),
		runningStyle.Render(fmt.Sprintf("%d running", len(m.agents.Sessions)))))

	// List individual agents with selection indicator
	for i, s := range m.agents.Sessions {
		selector := "  "
		if i == m.selectedAgentIdx {
			selector = "> "
		}
		lines = append(lines, fmt.Sprintf("%s%s%s (%s)",
			labelStyle.Render(""),
			selector,
			s.Name,
			s.Command))
	}

	return lines
}

func (m Model) renderKillConfirmation() string {
	msg := fmt.Sprintf(`Kill agent "%s" (index %d)?

Press Y to confirm, N to cancel.`,
		m.killTargetName, m.killTargetIndex)

	return boxStyle.Render(msg)
}

func (m Model) renderSizeSelection() string {
	var lines []string
	lines = append(lines, "Select VM size:\n")

	for i, size := range m.sizeOptions {
		selector := "  "
		if i == m.selectedSizeIdx {
			selector = "> "
		}
		machineType := m.cfg.VM.MachineSizes[size]
		lines = append(lines, fmt.Sprintf("%s%s (%s)", selector, size, machineType))
	}

	lines = append(lines, "\n↑/↓: select • Enter: create • Esc: cancel")

	var result string
	for i, line := range lines {
		result += line
		if i < len(lines)-1 {
			result += "\n"
		}
	}

	return boxStyle.Render(result)
}

func (m Model) renderDeleteConfirmation() string {
	msg := fmt.Sprintf(`Delete VM "%s"?

This will permanently delete the VM and its boot disk.

Press Y to confirm, N to cancel.`,
		m.cfg.VM.Name)

	return boxStyle.Render(msg)
}

func (m Model) renderHelp() string {
	// Handle selection/confirmation dialogs
	if m.selectingSize {
		return helpStyle.Render("↑/↓: select • Enter: create • Esc: cancel")
	}
	if m.confirmingDelete || m.confirmingKill {
		return helpStyle.Render("Y: confirm • N: cancel")
	}

	// Base actions always available
	actions := []string{"q: quit", "r: refresh"}

	// Add context-sensitive actions based on VM state
	if m.vmInfo != nil && m.operation == "" {
		switch m.vmInfo.Status {
		case cloud.VMStatusNotFound:
			actions = append(actions, "C: create")
		case cloud.VMStatusStopped:
			actions = append(actions, "S: start", "D: delete")
		case cloud.VMStatusRunning:
			actions = append(actions, "T: stop")
			// Agent management actions when VM is running
			if !m.agentsLoading {
				actions = append(actions, "A: add agent")
				if m.agents != nil && len(m.agents.Sessions) > 0 {
					actions = append(actions, "c: connect", "K: kill agent", "↑/↓: select")
				}
			}
		}
	}

	var result string
	for i, action := range actions {
		if i > 0 {
			result += " • "
		}
		result += action
	}

	return helpStyle.Render(result)
}
