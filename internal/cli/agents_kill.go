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

var agentsKillCmd = &cobra.Command{
	Use:   "kill <index>",
	Short: "Kill an agent session",
	Long: `Kill an agent session by its tmux window index.

The window index can be found using 'cloudcoop agents list'.

By default, this command will refuse to kill a window that has an active
process running (i.e., not just a shell). Use --force to override this
protection.

Examples:
  cloudcoop agents kill 0           # Kill agent at index 0
  cloudcoop agents kill 2 --force   # Force kill even if process is running`,
	Args: cobra.ExactArgs(1),
	RunE: runAgentsKill,
}

var killForce bool

func init() {
	agentsKillCmd.Flags().BoolVarP(&killForce, "force", "f", false, "force kill even if window has active process")
}

func runAgentsKill(cmd *cobra.Command, args []string) error {
	// Parse window index
	index, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid window index: %s", args[0])
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

	// Check if VM is running
	if vmInfo.Status == cloud.VMStatusNotFound {
		fmt.Fprintln(os.Stderr, "VM not found:", cfg.VM.Name)
		return nil
	}

	if vmInfo.Status != cloud.VMStatusRunning {
		fmt.Fprintf(os.Stderr, "VM is %s (must be running to kill agents)\n", vmInfo.Status)
		return nil
	}

	// Connect via SSH using helper
	ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VM has no IP address available for SSH connection")
		return nil
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	log.Debug("connecting to VM via SSH", "host", ip, "user", sshUser, "port", cfg.SSH.Port)

	client, err := ssh.NewClient(ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port))
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Kill agent session
	opts := agent.KillSessionOptions{
		Index: index,
		Force: killForce,
	}

	err = agent.KillSession(client, opts)
	if err != nil {
		if errors.Is(err, agent.ErrTmuxNotInstalled) {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			return nil
		}
		if errors.Is(err, agent.ErrNoSession) {
			fmt.Fprintln(os.Stderr, "No agents session exists")
			return nil
		}
		if errors.Is(err, agent.ErrWindowNotFound) {
			fmt.Fprintf(os.Stderr, "No agent found at index %d\n", index)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "List agents with:")
			fmt.Fprintln(os.Stderr, "  cloudcoop agents list")
			return nil
		}
		if errors.Is(err, agent.ErrActiveProcess) {
			fmt.Fprintf(os.Stderr, "Agent at index %d has an active process running\n", index)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Use --force to kill anyway:")
			fmt.Fprintf(os.Stderr, "  cloudcoop agents kill %d --force\n", index)
			return nil
		}
		return fmt.Errorf("kill session: %w", err)
	}

	fmt.Printf("Killed agent at index %d\n", index)
	return nil
}
