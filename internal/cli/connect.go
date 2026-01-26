package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var connectCmd = &cobra.Command{
	Use:   "connect <index>",
	Short: "Connect to an agent session",
	Long: `Connect to an agent session by attaching to its tmux window.

The window index can be found using 'cloudcoop agents list'.

This command shells out to SSH and attaches to the tmux window,
giving you an interactive terminal session with the agent.

To detach from the session (return to cloudcoop) press: Ctrl+B D
To exit the agent completely, type: exit

Examples:
  cloudcoop connect 0           # Connect to agent at index 0
  cloudcoop connect 2           # Connect to agent at index 2`,
	Args: cobra.ExactArgs(1),
	RunE: runConnect,
}

func init() {
	rootCmd.AddCommand(connectCmd)
}

func runConnect(cmd *cobra.Command, args []string) error {
	// Check SSH is available
	if err := ssh.CheckSSHAvailable(); err != nil {
		return err
	}

	// Parse window index
	index, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid window index: %s", args[0])
	}

	// Load configuration
	cfg, err := config.Load()
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
		fmt.Fprintf(os.Stderr, "VM is %s (must be running to connect)\n", vmInfo.Status)
		return nil
	}

	// Get IP address for SSH
	ip := vmInfo.ExternalIP
	if ip == "" {
		ip = vmInfo.InternalIP
	}
	if ip == "" {
		fmt.Fprintln(os.Stderr, "VM has no IP address available for SSH connection")
		return nil
	}

	// Determine SSH user
	sshUser := cfg.SSH.User
	if sshUser == "" {
		if u, err := user.Current(); err == nil {
			sshUser = u.Username
		}
	}

	// First verify the window exists using Go SSH client
	log.Debug("verifying agent window exists", "index", index)
	sshCfg := ssh.Config{
		Host:    ip,
		User:    sshUser,
		Port:    cfg.SSH.Port,
		Timeout: ssh.DefaultTimeout,
	}

	client, err := ssh.NewClient(sshCfg)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	// Check agent exists
	result, err := agent.ListSessions(client)
	_ = client.Close() // Close before interactive session

	if err != nil {
		if errors.Is(err, agent.ErrTmuxNotInstalled) {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			return nil
		}
		return fmt.Errorf("list sessions: %w", err)
	}

	if result.NoSession {
		fmt.Fprintln(os.Stderr, "No agents session exists")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Create an agent first with:")
		fmt.Fprintln(os.Stderr, "  cloudcoop agents add")
		return nil
	}

	// Find the agent with the given index
	var found bool
	for _, s := range result.Sessions {
		if s.Index == index {
			found = true
			break
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "No agent found at index %d\n", index)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Available agents:")
		for _, s := range result.Sessions {
			fmt.Fprintf(os.Stderr, "  %d: %s (%s)\n", s.Index, s.Name, s.Command)
		}
		return nil
	}

	// Connect interactively
	fmt.Printf("Connecting to agent at index %d...\n", index)
	fmt.Println("(Detach with Ctrl+B D, exit with 'exit')")
	fmt.Println()

	opts := ssh.ConnectOptions{
		Host:        ip,
		User:        sshUser,
		Port:        cfg.SSH.Port,
		WindowIndex: index,
	}

	err = ssh.ConnectInteractive(opts)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Disconnected from agent session.")
	return nil
}
