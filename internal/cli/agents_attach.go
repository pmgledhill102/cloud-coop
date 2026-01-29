package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var agentsAttachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Attach to an agent session",
	Long: `Attach to an agent session running in tmux on the cloud VM.

Use --next to automatically select the next unattached window, or
use --window to attach to a specific window by name or index.

Examples:
  cloudcoop agents attach --next              # auto-select next unattached window
  cloudcoop agents attach --window main       # attach to specific window by name
  cloudcoop agents attach --window 2          # attach to specific window by index`,
	RunE: runAgentsAttach,
}

var (
	attachNext   bool
	attachWindow string
)

func init() {
	agentsAttachCmd.Flags().BoolVar(&attachNext, "next", false, "auto-select next unattached window")
	agentsAttachCmd.Flags().StringVar(&attachWindow, "window", "", "window name or index to attach to")
	agentsAttachCmd.MarkFlagsMutuallyExclusive("next", "window")
}

func runAgentsAttach(cmd *cobra.Command, args []string) error {
	if !attachNext && attachWindow == "" {
		return fmt.Errorf("either --next or --window is required")
	}

	// Check that SSH is available for interactive connection
	if err := ssh.CheckSSHAvailable(); err != nil {
		return err
	}

	// Load configuration
	cfg, err := configLoader()
	if err != nil {
		return handleConfigError(err)
	}

	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Create provider to get VM info
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	provider, cleanup, err := createProvider(ctx, cfg)
	if err != nil {
		return handleProviderError(err)
	}
	defer cleanup()

	// Get VM info
	log.Debug("querying VM status", "name", cfg.VM.Name, "provider", provider.Name())
	vmInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		return fmt.Errorf("get VM status: %w", err)
	}

	if vmInfo.Status == cloud.VMStatusNotFound {
		fmt.Fprintln(os.Stderr, "VM not found:", cfg.VM.Name)
		return nil
	}

	if vmInfo.Status != cloud.VMStatusRunning {
		fmt.Fprintf(os.Stderr, "VM is %s (must be running to attach)\n", vmInfo.Status)
		return nil
	}

	// Resolve SSH connection parameters
	ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VM has no IP address available for SSH connection")
		return nil
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	sshPort := ssh.ResolvePort(cfg.SSH.Port)
	log.Debug("connecting to VM via SSH", "host", ip, "user", sshUser, "port", sshPort)

	// Connect via Go SSH client for setup commands
	client, err := ssh.NewClient(ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port))
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	// List sessions in the tmux session
	listResult, err := agent.ListSessions(client, resolveSessionName())
	if err != nil {
		_ = client.Close()
		if errors.Is(err, agent.ErrTmuxNotInstalled) {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			return nil
		}
		return fmt.Errorf("list sessions: %w", err)
	}

	if listResult.NoSession || len(listResult.Sessions) == 0 {
		_ = client.Close()
		fmt.Fprintln(os.Stderr, "No agent sessions found")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Start an agent session with:")
		fmt.Fprintln(os.Stderr, "  cloudcoop agents add")
		return nil
	}

	var targetWindow agent.Session
	var groupedSessionName string

	if attachNext {
		// Get clients to find unattached window
		clients, err := agent.ListClients(client, resolveSessionName())
		if err != nil {
			_ = client.Close()
			return fmt.Errorf("list clients: %w", err)
		}

		targetWindow, err = agent.FindNextWindow(listResult.Sessions, clients)
		if err != nil {
			_ = client.Close()
			if errors.Is(err, agent.ErrAllWindowsAttached) {
				fmt.Fprintln(os.Stderr, "All windows have attached clients")
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Create a new agent session with:")
				fmt.Fprintln(os.Stderr, "  cloudcoop agents add")
				return nil
			}
			return fmt.Errorf("find next window: %w", err)
		}

		// Create grouped session for this attachment
		groupedSessionName, err = agent.CreateGroupedSession(client, resolveSessionName(), targetWindow.Index)
		if err != nil {
			_ = client.Close()
			return fmt.Errorf("create grouped session: %w", err)
		}
	} else {
		// Find window by name or index
		targetWindow, err = findWindow(listResult.Sessions, attachWindow)
		if err != nil {
			_ = client.Close()
			return err
		}

		// Create grouped session for specific window too
		groupedSessionName, err = agent.CreateGroupedSession(client, resolveSessionName(), targetWindow.Index)
		if err != nil {
			_ = client.Close()
			return fmt.Errorf("create grouped session: %w", err)
		}
	}

	// Close Go SSH client before interactive attach
	_ = client.Close()

	fmt.Printf("Attaching to window %d (%s)...\n", targetWindow.Index, targetWindow.Name)

	// Connect interactively via native SSH
	connectOpts := ssh.ConnectOptions{
		Host:           ip,
		User:           sshUser,
		Port:           sshPort,
		Session:        resolveSessionName(),
		WindowIndex:    targetWindow.Index,
		GroupedSession: groupedSessionName,
	}

	return ssh.ConnectInteractive(connectOpts)
}

// findWindow looks up a window by name or index string.
func findWindow(sessions []agent.Session, nameOrIndex string) (agent.Session, error) {
	// Try as index first
	if idx, err := strconv.Atoi(nameOrIndex); err == nil {
		for _, s := range sessions {
			if s.Index == idx {
				return s, nil
			}
		}
		return agent.Session{}, fmt.Errorf("no window at index %d", idx)
	}

	// Try as name
	for _, s := range sessions {
		if s.Name == nameOrIndex {
			return s, nil
		}
	}
	return agent.Session{}, fmt.Errorf("no window named %q", nameOrIndex)
}
