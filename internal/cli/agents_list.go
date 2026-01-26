package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List running agent sessions",
	Long: `List agent sessions running in tmux on the cloud VM.

This command connects to the VM via SSH and queries the "agents" tmux session
for running windows. Each window typically contains one AI coding agent.

Example output:
  INDEX  NAME      COMMAND
  0      agent-1   claude
  1      agent-2   aider
  2      agent-3   bash`,
	RunE: runAgentsList,
}

func runAgentsList(cmd *cobra.Command, args []string) error {
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
		fmt.Fprintf(os.Stderr, "VM is %s (must be running to list agents)\n", vmInfo.Status)
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

	// List agent sessions
	result, err := agent.ListSessions(client)
	if err != nil {
		if errors.Is(err, agent.ErrTmuxNotInstalled) {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Install tmux:")
			fmt.Fprintln(os.Stderr, "  sudo apt install tmux    # Debian/Ubuntu")
			fmt.Fprintln(os.Stderr, "  sudo yum install tmux    # RHEL/CentOS")
			return nil
		}
		return fmt.Errorf("list sessions: %w", err)
	}

	// Display results
	printAgentList(result)
	return nil
}

func printAgentList(result *agent.ListResult) {
	if result.NoSession {
		fmt.Println("No agents session")
		fmt.Println()
		fmt.Println("Start an agents session with:")
		fmt.Println("  tmux new-session -s agents")
		return
	}

	if len(result.Sessions) == 0 {
		fmt.Println("Agents session exists but has no windows")
		return
	}

	// Print header
	fmt.Printf("%-6s %-12s %s\n", "INDEX", "NAME", "COMMAND")

	// Print sessions
	for _, s := range result.Sessions {
		fmt.Printf("%-6d %-12s %s\n", s.Index, s.Name, s.Command)
	}

	fmt.Println()
	fmt.Printf("%d agent(s) running\n", len(result.Sessions))
}
