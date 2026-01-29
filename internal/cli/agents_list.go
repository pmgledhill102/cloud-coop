package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List running agent sessions",
	Long: `List agent sessions running in tmux on the cloud VM.

This command connects to the VM via SSH and queries the tmux session
for running windows. Each window typically contains one AI coding agent.

Use --all to list sessions across all tmux sessions (repos).

Example output:
  INDEX  NAME      COMMAND
  0      agent-1   claude
  1      agent-2   aider
  2      agent-3   bash`,
	RunE: runAgentsList,
}

var listAll bool

func init() {
	agentsListCmd.Flags().BoolVar(&listAll, "all", false, "list sessions across all tmux sessions")
}

func runAgentsList(cmd *cobra.Command, args []string) error {
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

	if listAll {
		return runAgentsListAll(client)
	}

	// List agent sessions
	result, err := agent.ListSessions(client, resolveSessionName())
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
		fmt.Println("No tmux session found")
		fmt.Println()
		fmt.Println("Start an agent session with:")
		fmt.Println("  cloudcoop agents add")
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

// runAgentsListAll lists agents across all tmux sessions on the VM.
func runAgentsListAll(runner ssh.Runner) error {
	// Get all tmux sessions with their group info
	output, err := runner.Run("tmux list-sessions -F '#{session_name}|#{session_group}'")
	if err != nil {
		errStr := strings.ToLower(output + err.Error())
		if strings.Contains(errStr, "no server running") ||
			strings.Contains(errStr, "no such file or directory") ||
			strings.Contains(errStr, "error connecting") {
			fmt.Println("No tmux sessions found")
			return nil
		}
		if strings.Contains(errStr, "command not found") ||
			strings.Contains(errStr, "exit status 127") {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			return nil
		}
		return fmt.Errorf("list tmux sessions: %w", err)
	}

	// Parse sessions, filtering to base sessions only
	baseSessions := parseBaseSessions(output)
	if len(baseSessions) == 0 {
		fmt.Println("No tmux sessions found")
		return nil
	}

	totalAgents := 0
	for i, sessionName := range baseSessions {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("Session: %s\n", sessionName)

		result, err := agent.ListSessions(runner, sessionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  (error: %v)\n", err)
			continue
		}

		if result.NoSession || len(result.Sessions) == 0 {
			fmt.Println("  (no windows)")
			continue
		}

		fmt.Printf("  %-6s %-12s %s\n", "INDEX", "NAME", "COMMAND")
		for _, s := range result.Sessions {
			fmt.Printf("  %-6d %-12s %s\n", s.Index, s.Name, s.Command)
		}
		totalAgents += len(result.Sessions)
	}

	fmt.Println()
	fmt.Printf("%d agent(s) across %d session(s)\n", totalAgents, len(baseSessions))
	return nil
}

// parseBaseSessions parses tmux list-sessions output and returns base session names.
// Base sessions are those where session_group is empty or session_name equals session_group.
// Lines are expected in format: sessionName|sessionGroup
func parseBaseSessions(output string) []string {
	var sessions []string
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		name := parts[0]
		group := ""
		if len(parts) == 2 {
			group = parts[1]
		}

		// Base session: no group, or name equals group
		if group == "" || name == group {
			sessions = append(sessions, name)
		}
	}
	return sessions
}
