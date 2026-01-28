package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// Supported agent types for authentication.
const (
	AgentClaude = "claude"
)

// agentAuthCommands maps agent types to their auth commands.
var agentAuthCommands = map[string]struct {
	login  string
	status string
}{
	AgentClaude: {
		login:  "claude auth login",
		status: "claude auth status",
	},
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage agent authentication",
	Long: `Manage authentication for AI agents running on the cloud VM.

Authentication is required for agents to access their respective APIs.
This command group provides tools to authenticate and check auth status.

Supported agents:
  claude    Claude Code (OAuth-based)

For OAuth-based agents, the login command establishes an SSH tunnel
for the OAuth callback, then runs the agent's auth command on the VM.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login [agent]",
	Short: "Authenticate an agent via OAuth",
	Long: `Authenticate an agent on the cloud VM via OAuth.

This command:
1. Connects to the VM via SSH with port forwarding (for OAuth callback)
2. Runs the agent's authentication command interactively
3. Opens your browser for OAuth login

The SSH session runs interactively, allowing you to complete the
authentication flow in your browser.

Examples:
  cloudcoop auth login           # Authenticate default agent (claude)
  cloudcoop auth login claude    # Authenticate Claude Code`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAuthLogin,
}

var authStatusCmd = &cobra.Command{
	Use:   "status [agent]",
	Short: "Check authentication status",
	Long: `Check the authentication status of an agent on the cloud VM.

This connects via SSH and runs the agent's status command to verify
that authentication is configured correctly.

Examples:
  cloudcoop auth status           # Check default agent (claude)
  cloudcoop auth status claude    # Check Claude Code auth status`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAuthStatus,
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	// Determine agent type (default to claude)
	agentType := AgentClaude
	if len(args) > 0 {
		agentType = strings.ToLower(args[0])
	}

	// Validate agent type
	agentCmds, ok := agentAuthCommands[agentType]
	if !ok {
		return fmt.Errorf("unsupported agent: %s (supported: claude)", agentType)
	}

	// Check that ssh command is available
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

	// Get VM info
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	provider, cleanup, err := createProvider(ctx, cfg)
	if err != nil {
		return handleProviderError(err)
	}
	defer cleanup()

	log.Debug("querying VM status", "name", cfg.VM.Name, "provider", provider.Name())
	vmInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		return fmt.Errorf("get VM status: %w", err)
	}

	// Check VM state
	if vmInfo.Status == cloud.VMStatusNotFound {
		fmt.Fprintln(os.Stderr, "VM not found:", cfg.VM.Name)
		return nil
	}

	if vmInfo.Status != cloud.VMStatusRunning {
		fmt.Fprintf(os.Stderr, "VM is %s (must be running for auth)\n", vmInfo.Status)
		return nil
	}

	// Get connection details
	host := vmInfo.ExternalIP
	if host == "" {
		return fmt.Errorf("VM %s has no external IP address", cfg.VM.Name)
	}

	user := ssh.ResolveSSHUser(cfg.SSH.User)
	port := ssh.ResolvePort(cfg.SSH.Port)

	// Build SSH command with port forwarding for OAuth callback
	// -L 8080:localhost:8080 forwards the OAuth callback port
	// -t forces pseudo-terminal allocation for interactive auth
	sshArgs := []string{
		"-L", "8080:localhost:8080",
		"-t",
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("%s@%s", user, host),
		agentCmds.login,
	}

	log.Debug("running auth login", "agent", agentType, "host", host, "user", user)
	fmt.Printf("Authenticating %s on %s...\n", agentType, cfg.VM.Name)
	fmt.Println()

	// Run SSH command interactively
	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	if err := sshCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Non-zero exit from remote command
			return fmt.Errorf("auth login failed (exit code %d)", exitErr.ExitCode())
		}
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("Authentication complete. Run 'cloudcoop auth status %s' to verify.\n", agentType)
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	// Determine agent type (default to claude)
	agentType := AgentClaude
	if len(args) > 0 {
		agentType = strings.ToLower(args[0])
	}

	// Validate agent type
	agentCmds, ok := agentAuthCommands[agentType]
	if !ok {
		return fmt.Errorf("unsupported agent: %s (supported: claude)", agentType)
	}

	// Load configuration
	cfg, err := configLoader()
	if err != nil {
		return handleConfigError(err)
	}

	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Get VM info
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	provider, cleanup, err := createProvider(ctx, cfg)
	if err != nil {
		return handleProviderError(err)
	}
	defer cleanup()

	log.Debug("querying VM status", "name", cfg.VM.Name, "provider", provider.Name())
	vmInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		return fmt.Errorf("get VM status: %w", err)
	}

	// Check VM state
	if vmInfo.Status == cloud.VMStatusNotFound {
		fmt.Fprintln(os.Stderr, "VM not found:", cfg.VM.Name)
		return nil
	}

	if vmInfo.Status != cloud.VMStatusRunning {
		fmt.Fprintf(os.Stderr, "VM is %s (must be running to check auth status)\n", vmInfo.Status)
		return nil
	}

	// Connect via SSH
	ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VM has no IP address available for SSH connection")
		return nil
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	log.Debug("connecting to VM via SSH", "host", ip, "user", sshUser, "port", cfg.SSH.Port)

	client, err := ssh.NewClient(ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port))
	if err != nil {
		return handleSSHError(err, ip, ssh.ResolvePort(cfg.SSH.Port))
	}
	defer func() { _ = client.Close() }()

	// Run auth status command
	log.Debug("running auth status", "agent", agentType, "command", agentCmds.status)
	output, err := client.Run(agentCmds.status)

	// Print header
	fmt.Printf("Auth status for %s on %s:\n", agentType, cfg.VM.Name)
	fmt.Println()

	// Print output
	if output != "" {
		fmt.Print(output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}

	if err != nil {
		// Check for command not found
		if strings.Contains(output, "command not found") || strings.Contains(output, "not found") {
			fmt.Println()
			fmt.Fprintf(os.Stderr, "%s is not installed on the VM.\n", agentType)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Install it with:")
			fmt.Fprintln(os.Stderr, "  cloudcoop provision")
			return nil
		}
		// Other errors - command ran but returned non-zero
		// This often indicates auth is not configured
		return nil
	}

	return nil
}
