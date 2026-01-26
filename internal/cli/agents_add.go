package cli

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var agentsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new agent session",
	Long: `Add a new agent session in a tmux window on the cloud VM.

Creates a new tmux window in the "agents" session. If no session exists,
one will be created automatically.

Examples:
  cloudcoop agents add                          # Add with default command (bash)
  cloudcoop agents add --name=claude-1          # Add with custom name
  cloudcoop agents add --command="claude"       # Add with custom command
  cloudcoop agents add --name=ai --command="claude --dangerously-skip-permissions"`,
	RunE: runAgentsAdd,
}

var (
	addName    string
	addCommand string
)

func init() {
	agentsAddCmd.Flags().StringVar(&addName, "name", "", "window name (auto-generated if not specified)")
	agentsAddCmd.Flags().StringVar(&addCommand, "command", "", "command to run (default: bash)")
}

func runAgentsAdd(cmd *cobra.Command, args []string) error {
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
		fmt.Fprintf(os.Stderr, "VM is %s (must be running to add agents)\n", vmInfo.Status)
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

	// Connect via SSH
	log.Debug("connecting to VM via SSH", "host", ip, "user", sshUser, "port", cfg.SSH.Port)
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
	defer func() { _ = client.Close() }()

	// Determine command: flag > config > default
	command := addCommand
	if command == "" && cfg.Agents.DefaultCommand != "" {
		command = cfg.Agents.DefaultCommand
	}

	// Create agent session
	opts := agent.CreateSessionOptions{
		Name:    addName,
		Command: command,
	}

	session, err := agent.CreateSession(client, opts)
	if err != nil {
		if err == agent.ErrTmuxNotInstalled {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Install tmux:")
			fmt.Fprintln(os.Stderr, "  sudo apt install tmux    # Debian/Ubuntu")
			fmt.Fprintln(os.Stderr, "  sudo yum install tmux    # RHEL/CentOS")
			return nil
		}
		return fmt.Errorf("create session: %w", err)
	}

	// Report success
	displayName := session.Name
	if displayName == "" {
		displayName = fmt.Sprintf("agent-%d", session.Index)
	}
	displayCmd := session.Command
	if displayCmd == "" {
		displayCmd = "bash"
	}

	fmt.Printf("Created agent session: %s (index %d)\n", displayName, session.Index)
	fmt.Printf("Command: %s\n", displayCmd)
	fmt.Println()
	fmt.Println("Connect with:")
	fmt.Printf("  cloudcoop agents connect %d\n", session.Index)

	return nil
}
