// Package tui provides the terminal user interface for cloudcoop.
package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/cloud/gcp"
	"github.com/cloud-coop/cloudcoop/internal/config"
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
	operation string // "", "starting", "stopping"

	cfg     *config.Config
	cfgErr  error
	vmInfo  *cloud.VMInfo
	vmErr   error
	cleanup func()
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

// Init initializes the TUI application.
func (m Model) Init() tea.Cmd {
	return loadConfig
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
	}

	return m, nil
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
	} else if m.operation == "starting" {
		content = boxStyle.Render("Starting VM... ◐")
	} else if m.operation == "stopping" {
		content = boxStyle.Render("Stopping VM... ◑")
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

Create a config file at:
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
	lines = append(lines, fmt.Sprintf("%s%s",
		labelStyle.Render("Agents:"),
		stoppedStyle.Render("(not yet implemented)")))

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

func (m Model) renderHelp() string {
	// Base actions always available
	actions := []string{"q: quit", "r: refresh"}

	// Add context-sensitive actions based on VM state
	if m.vmInfo != nil && m.operation == "" {
		switch m.vmInfo.Status {
		case cloud.VMStatusStopped:
			actions = append(actions, "S: start")
		case cloud.VMStatusRunning:
			actions = append(actions, "T: stop")
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
