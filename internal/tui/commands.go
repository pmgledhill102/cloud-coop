package tui

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/cloud/gcp"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// Message types for async operations.

type configLoadedMsg struct {
	cfg *config.Config
	err error
}

type vmInfoMsg struct {
	info    *cloud.VMInfo
	err     error
	cleanup func()
}

type vmStartMsg struct{ err error }
type vmStopMsg struct{ err error }
type vmCreateMsg struct{ err error }
type vmDeleteMsg struct{ err error }

type agentsMsg struct {
	result *agent.ListResult
	err    error
}

type agentAddedMsg struct {
	session *agent.Session
	err     error
}

type agentKilledMsg struct {
	index int
	err   error
}

type provisionStatusMsg struct {
	status *provisioning.StatusInfo
	err    error
}

type connectFinishedMsg struct{ err error }

// newProvider creates a cloud provider based on config.
func newProvider(ctx context.Context, cfg *config.Config) (cloud.Provider, func(), error) {
	switch cfg.Cloud.Provider {
	case "gcp":
		p, err := gcp.New(ctx, cfg.Cloud.GCP.Project, cfg.Cloud.GCP.Zone)
		if err != nil {
			return nil, nil, fmt.Errorf("create GCP provider: %w", err)
		}
		return p, func() { _ = p.Close() }, nil
	default:
		return nil, nil, fmt.Errorf("unsupported provider: %s", cfg.Cloud.Provider)
	}
}

func loadConfig() tea.Msg {
	cfg, err := config.Load()
	return configLoadedMsg{cfg: cfg, err: err}
}

func fetchVMInfo(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		provider, cleanup, err := newProvider(ctx, cfg)
		if err != nil {
			return vmInfoMsg{err: err}
		}

		info, err := provider.GetVMInfo(ctx, cfg.VM.Name)
		if err != nil {
			cleanup()
			return vmInfoMsg{err: fmt.Errorf("get VM info: %w", err)}
		}

		return vmInfoMsg{info: info, cleanup: cleanup}
	}
}

func startVM(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		provider, cleanup, err := newProvider(ctx, cfg)
		if err != nil {
			return vmStartMsg{err: err}
		}
		defer cleanup()

		if err := provider.StartVM(ctx, cfg.VM.Name); err != nil {
			return vmStartMsg{err: fmt.Errorf("start VM: %w", err)}
		}
		return vmStartMsg{}
	}
}

func stopVM(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		provider, cleanup, err := newProvider(ctx, cfg)
		if err != nil {
			return vmStopMsg{err: err}
		}
		defer cleanup()

		if err := provider.StopVM(ctx, cfg.VM.Name); err != nil {
			return vmStopMsg{err: fmt.Errorf("stop VM: %w", err)}
		}
		return vmStopMsg{}
	}
}

func createVM(cfg *config.Config, machineType string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		provider, cleanup, err := newProvider(ctx, cfg)
		if err != nil {
			return vmCreateMsg{err: err}
		}
		defer cleanup()

		createCfg := cloud.VMCreateConfig{
			Name:               cfg.VM.Name,
			MachineType:        machineType,
			DiskSizeGB:         cfg.VM.DiskSizeGB,
			Image:              cfg.VM.Image,
			Spot:               cfg.VM.Spot,
			Network:            cfg.VM.Network,
			Tags:               cfg.VM.Tags,
			SSHPort:            cfg.SSH.Port,
			ServiceAccount:     cfg.Cloud.GCP.ServiceAccount,
			ProvisionScriptURL: cfg.Provisioning.ScriptURL,
		}

		if err := provider.CreateVM(ctx, createCfg); err != nil {
			return vmCreateMsg{err: fmt.Errorf("create VM: %w", err)}
		}
		return vmCreateMsg{}
	}
}

func deleteVM(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		provider, cleanup, err := newProvider(ctx, cfg)
		if err != nil {
			return vmDeleteMsg{err: err}
		}
		defer cleanup()

		if err := provider.DeleteVM(ctx, cfg.VM.Name); err != nil {
			return vmDeleteMsg{err: fmt.Errorf("delete VM: %w", err)}
		}
		return vmDeleteMsg{}
	}
}

// connectSSH creates an SSH client for agent operations.
func connectSSH(cfg *config.Config, vmInfo *cloud.VMInfo) (*ssh.Client, error) {
	ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	if err != nil {
		return nil, fmt.Errorf("no IP address available")
	}
	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	return ssh.NewClient(ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port))
}

func fetchAgents(cfg *config.Config, vmInfo *cloud.VMInfo) tea.Cmd {
	return func() tea.Msg {
		client, err := connectSSH(cfg, vmInfo)
		if err != nil {
			return agentsMsg{err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		result, err := agent.ListSessions(client)
		return agentsMsg{result: result, err: err}
	}
}

func addAgent(cfg *config.Config, vmInfo *cloud.VMInfo) tea.Cmd {
	return func() tea.Msg {
		client, err := connectSSH(cfg, vmInfo)
		if err != nil {
			return agentAddedMsg{err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		opts := agent.CreateSessionOptions{Command: cfg.Agents.DefaultCommand}
		session, err := agent.CreateSession(client, opts)
		return agentAddedMsg{session: session, err: err}
	}
}

func connectToAgent(cfg *config.Config, vmInfo *cloud.VMInfo, windowIndex int) tea.Cmd {
	ip, _ := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	port := ssh.ResolvePort(cfg.SSH.Port)

	tmuxCmd := fmt.Sprintf("tmux select-window -t agents:%d && tmux attach -t agents", windowIndex)
	c := exec.Command("ssh", "-t", "-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("%s@%s", sshUser, ip), tmuxCmd)

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return connectFinishedMsg{err: err}
	})
}

func killAgent(cfg *config.Config, vmInfo *cloud.VMInfo, index int) tea.Cmd {
	return func() tea.Msg {
		client, err := connectSSH(cfg, vmInfo)
		if err != nil {
			return agentKilledMsg{index: index, err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		opts := agent.KillSessionOptions{Index: index, Force: true}
		err = agent.KillSession(client, opts)
		return agentKilledMsg{index: index, err: err}
	}
}

func fetchProvisionStatus(cfg *config.Config, vmInfo *cloud.VMInfo) tea.Cmd {
	return func() tea.Msg {
		client, err := connectSSH(cfg, vmInfo)
		if err != nil {
			return provisionStatusMsg{err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		status, err := provisioning.CheckStatus(client)
		return provisionStatusMsg{status: status, err: err}
	}
}
