package tui

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ops"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
	"github.com/cloud-coop/cloudcoop/internal/setup"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
	"github.com/cloud-coop/cloudcoop/internal/workspace"
)

// defaultSessionName is the tmux session name used until workspace detection
// is integrated.
const defaultSessionName = "agents"

// Message types for async operations.

type configLoadedMsg struct {
	cfg       *config.Config
	err       error
	workspace *workspace.Info
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

type connectFinishedMsg struct{ err error }

// connectReadyMsg is returned when pre-connect checks pass and the SSH
// exec.Cmd is ready to run. Update handles it by calling tea.ExecProcess.
type connectReadyMsg struct{ cmd *exec.Cmd }

type preflightMsg struct {
	result *cloud.PreflightResult
	err    error
}

type firewallCheckedMsg struct{ err error }

type sshKeyCheckedMsg struct{ err error }

// refreshTickMsg is sent periodically to trigger always-on auto-refresh.
type refreshTickMsg struct{}

// scheduleRefresh returns a command that sends a refreshTickMsg after the configured interval.
// The TUI always keeps a refresh timer running to stay current.
func scheduleRefresh(cfg *config.Config) tea.Cmd {
	interval := 15 * time.Second
	if cfg != nil && cfg.TUI.RefreshIntervalSec > 0 {
		interval = time.Duration(cfg.TUI.RefreshIntervalSec) * time.Second
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func loadConfig() tea.Msg {
	cfg, err := config.LoadMerged()
	ws, _ := workspace.Detect(workspace.NewGitRunner(".")) // nil if not in a git repo
	return configLoadedMsg{cfg: cfg, err: err, workspace: ws}
}

func fetchVMInfo(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ops.TimeoutVMStatus)
		defer cancel()

		provider, cleanup, err := ops.NewProvider(ctx, cfg)
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
		ctx, cancel := context.WithTimeout(context.Background(), ops.TimeoutVMLifecycle)
		defer cancel()

		provider, cleanup, err := ops.NewProvider(ctx, cfg)
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
		ctx, cancel := context.WithTimeout(context.Background(), ops.TimeoutVMLifecycle)
		defer cancel()

		provider, cleanup, err := ops.NewProvider(ctx, cfg)
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
		ctx, cancel := context.WithTimeout(context.Background(), ops.TimeoutVMCreate)
		defer cancel()

		provider, cleanup, err := ops.NewProvider(ctx, cfg)
		if err != nil {
			return vmCreateMsg{err: err}
		}
		defer cleanup()

		// Read SSH public key (non-fatal if missing)
		pubKey, _ := ssh.ReadPublicKey()
		sshUser := ssh.ResolveSSHUser(cfg.SSH.User)

		createCfg := cloud.VMCreateConfig{
			Name:               cfg.VM.Name,
			MachineType:        machineType,
			DiskSizeGB:         cfg.VM.DiskSizeGB,
			Image:              cfg.VM.Image,
			Spot:               cfg.VM.Spot,
			Network:            cfg.VM.Network,
			Subnet:             cfg.VM.Subnet,
			Tags:               cfg.VM.Tags,
			SSHPort:            cfg.SSH.Port,
			SSHUser:            sshUser,
			SSHPublicKey:       pubKey,
			ServiceAccount:     cfg.Cloud.GCP.ServiceAccount,
			ProvisionScriptURL: cfg.Provisioning.ScriptURL,
			MaxUptimeMinutes:   cfg.VM.MaxUptimeMinutes,
		}

		if err := provider.CreateVM(ctx, createCfg); err != nil {
			return vmCreateMsg{err: fmt.Errorf("create VM: %w", err)}
		}

		// Best-effort: wait for SSH to become reachable before reporting success.
		info, err := provider.GetVMInfo(ctx, cfg.VM.Name)
		if err == nil && info.ExternalIP != "" {
			waitCfg := ssh.SetupClientConfig(info.ExternalIP, sshUser, cfg.SSH.Port)
			waitCfg.VM = ssh.NewVMIdentity(info.Name, info.CloudcoopCreated)
			_ = ssh.WaitForSSH(waitCfg, 30*time.Second)
		}

		return vmCreateMsg{}
	}
}

func deleteVM(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ops.TimeoutVMLifecycle)
		defer cancel()

		provider, cleanup, err := ops.NewProvider(ctx, cfg)
		if err != nil {
			return vmDeleteMsg{err: err}
		}
		defer cleanup()

		if err := provider.DeleteVM(ctx, cfg.VM.Name); err != nil {
			return vmDeleteMsg{err: fmt.Errorf("delete VM: %w", err)}
		}

		// Clean up pinned host key for the deleted VM.
		_ = ssh.ClearPinnedKey(cfg.VM.Name)

		return vmDeleteMsg{}
	}
}

func fetchAgents(cfg *config.Config, vmInfo *cloud.VMInfo, sessionName string) tea.Cmd {
	return func() tea.Msg {
		client, err := ops.ConnectSSH(cfg, vmInfo)
		if err != nil {
			return agentsMsg{err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		result, err := agent.ListSessions(client, sessionName)
		return agentsMsg{result: result, err: err}
	}
}

func addAgent(cfg *config.Config, vmInfo *cloud.VMInfo, sessionName string) tea.Cmd {
	return func() tea.Msg {
		client, err := ops.ConnectSSH(cfg, vmInfo)
		if err != nil {
			return agentAddedMsg{err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		opts := agent.CreateSessionOptions{Command: cfg.Agents.DefaultCommand}
		session, err := agent.CreateSession(client, sessionName, opts)
		return agentAddedMsg{session: session, err: err}
	}
}

// connectToAgent returns a tea.Cmd that performs SSH host-key verification
// asynchronously and, on success, returns a connectReadyMsg carrying the
// prepared exec.Cmd. The Update loop handles connectReadyMsg by calling
// tea.ExecProcess, which suspends the TUI for the interactive SSH session.
func connectToAgent(cfg *config.Config, vmInfo *cloud.VMInfo, windowIndex int, sessionName string) tea.Cmd {
	return func() tea.Msg {
		ip, _ := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
		sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
		port := ssh.ResolvePort(cfg.SSH.Port)
		vm := ssh.NewVMIdentity(vmInfo.Name, vmInfo.CloudcoopCreated)

		// Ensure host key is in cloudcoop's managed known_hosts before connecting
		if err := ssh.EnsureHostKeyPinned(ip, port, vm); err != nil {
			return connectFinishedMsg{err: fmt.Errorf("fetch host key: %w", err)}
		}

		knownHostsPath, err := ssh.CloudcoopKnownHostsPath()
		if err != nil {
			return connectFinishedMsg{err: fmt.Errorf("get known_hosts path: %w", err)}
		}

		tmuxCmd := fmt.Sprintf("tmux select-window -t %s:%d && tmux attach -t %s", sessionName, windowIndex, sessionName)
		c := exec.Command("ssh",
			"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
			"-t",
			"-p", fmt.Sprintf("%d", port),
			fmt.Sprintf("%s@%s", sshUser, ip), tmuxCmd)

		return connectReadyMsg{cmd: c}
	}
}

func killAgent(cfg *config.Config, vmInfo *cloud.VMInfo, index int, sessionName string) tea.Cmd {
	return func() tea.Msg {
		client, err := ops.ConnectSSH(cfg, vmInfo)
		if err != nil {
			return agentKilledMsg{index: index, err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		opts := agent.KillSessionOptions{Index: index, Force: true}
		err = agent.KillSession(client, sessionName, opts)
		return agentKilledMsg{index: index, err: err}
	}
}

type syncMsg struct {
	workspace *workspace.Info
	result    *workspace.SyncResult
	err       error
}

func syncWorkspace(cfg *config.Config, vmInfo *cloud.VMInfo, wsInfo *workspace.Info) tea.Cmd {
	return func() tea.Msg {
		client, err := ops.ConnectSSH(cfg, vmInfo)
		if err != nil {
			return syncMsg{err: fmt.Errorf("SSH: %w", err)}
		}
		defer func() { _ = client.Close() }()

		result, err := ops.SyncWorkspace(client, cfg, wsInfo, "")
		return syncMsg{workspace: wsInfo, result: result, err: err}
	}
}

func ensureSSHKey(cfg *config.Config, vmInfo *cloud.VMInfo) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ops.TimeoutAccess)
		defer cancel()

		provider, cleanup, err := ops.NewProvider(ctx, cfg)
		if err != nil {
			return sshKeyCheckedMsg{err: err}
		}
		defer cleanup()

		err = ops.EnsureSSHKey(ctx, cfg, provider, vmInfo.Name)
		return sshKeyCheckedMsg{err: err}
	}
}

func ensureFirewall(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ops.TimeoutAccess)
		defer cancel()

		provider, cleanup, err := ops.NewProvider(ctx, cfg)
		if err != nil {
			return firewallCheckedMsg{err: err}
		}
		defer cleanup()

		changed, err := ops.EnsureFirewall(ctx, cfg, provider)
		if err != nil {
			return firewallCheckedMsg{err: err}
		}

		if changed {
			log.Info("firewall updated")
		}

		return firewallCheckedMsg{}
	}
}

func runPreflight(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		provider, cleanup, err := ops.NewProvider(ctx, cfg)
		if err != nil {
			return preflightMsg{err: err}
		}
		defer cleanup()

		result, err := provider.Preflight(ctx, cloud.PreflightConfig{
			Network:      cfg.VM.Network,
			Subnet:       cfg.VM.Subnet,
			Zone:         cfg.Cloud.GCP.Zone,
			FirewallRule: setup.IAPFirewallRuleName,
		})
		return preflightMsg{result: result, err: err}
	}
}

// vmDetailsMsg combines agents and provisioning status into a single message
// to avoid double-redraw after each refresh tick.
type vmDetailsMsg struct {
	agents  *agent.ListResult
	agentsE error
	status  *provisioning.StatusInfo
	statusE error
}

// fetchVMDetails runs agent listing and provisioning status check in parallel,
// returning a single combined message to trigger one redraw instead of two.
func fetchVMDetails(cfg *config.Config, vmInfo *cloud.VMInfo, sessionName string) tea.Cmd {
	return func() tea.Msg {
		var (
			wg      sync.WaitGroup
			agents  *agent.ListResult
			agentsE error
			status  *provisioning.StatusInfo
			statusE error
		)

		wg.Add(2)

		go func() {
			defer wg.Done()
			client, err := ops.ConnectSSH(cfg, vmInfo)
			if err != nil {
				agentsE = fmt.Errorf("SSH: %w", err)
				return
			}
			defer func() { _ = client.Close() }()
			agents, agentsE = agent.ListSessions(client, sessionName)
		}()

		go func() {
			defer wg.Done()
			client, err := ops.ConnectSSH(cfg, vmInfo)
			if err != nil {
				statusE = fmt.Errorf("SSH: %w", err)
				return
			}
			defer func() { _ = client.Close() }()
			status, statusE = provisioning.CheckStatus(client)
		}()

		wg.Wait()
		return vmDetailsMsg{
			agents:  agents,
			agentsE: agentsE,
			status:  status,
			statusE: statusE,
		}
	}
}
