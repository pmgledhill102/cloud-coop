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

// agentAuthCommands maps agent types to their auth check commands.
// Claude doesn't have explicit auth commands - we verify auth by running a test prompt.
var agentAuthCommands = map[string]struct {
	// statusCheck is a command that succeeds only if authenticated
	statusCheck string
	// setupToken is the command for setting up authentication (interactive)
	setupToken string
}{
	AgentClaude: {
		statusCheck: "claude -p 'respond with: AUTH_OK' --max-turns 1 2>&1",
		setupToken:  "claude",
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
	Short: "Authenticate an agent interactively",
	Long: `Authenticate an agent on the cloud VM.

This command starts an interactive session with the agent on the VM.
For Claude, you'll be prompted with a URL to visit and a code to paste back.

The authentication is stored on the VM and shared across all agent sessions.

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

	// Ensure host key is available (uses cloudcoop's managed known_hosts)
	log.Debug("ensuring host key", "host", host, "port", port)
	if err := ssh.EnsureHostKey(host, port); err != nil {
		return fmt.Errorf("fetch host key: %w", err)
	}

	// Get path to cloudcoop's known_hosts file for native ssh
	knownHostsPath, err := ssh.CloudcoopKnownHostsPath()
	if err != nil {
		return fmt.Errorf("get known_hosts path: %w", err)
	}

	// Build SSH command for interactive session
	// -t forces pseudo-terminal allocation for interactive auth
	// -o UserKnownHostsFile uses cloudcoop's managed known_hosts
	sshArgs := []string{
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-t",
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("%s@%s", user, host),
		agentCmds.setupToken,
	}

	log.Debug("running auth login", "agent", agentType, "host", host, "user", user)
	fmt.Printf("Starting %s on %s for authentication...\n", agentType, cfg.VM.Name)
	fmt.Println("Follow the prompts to authenticate. Press Ctrl+C when done.")
	fmt.Println()

	// Run SSH command interactively
	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	if err := sshCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Non-zero exit - user likely pressed Ctrl+C which is expected
			if exitErr.ExitCode() == 130 || exitErr.ExitCode() == 2 {
				fmt.Println()
				fmt.Printf("Run 'cloudcoop auth status %s' to verify authentication.\n", agentType)
				return nil
			}
			return fmt.Errorf("auth login failed (exit code %d)", exitErr.ExitCode())
		}
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("Run 'cloudcoop auth status %s' to verify authentication.\n", agentType)
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

	// Run auth status check
	log.Debug("running auth status check", "agent", agentType, "command", agentCmds.statusCheck)
	output, err := client.Run(agentCmds.statusCheck)

	// Print header
	fmt.Printf("Auth status for %s on %s:\n", agentType, cfg.VM.Name)
	fmt.Println()

	// Check for command not found
	if strings.Contains(output, "command not found") || strings.Contains(output, "not found") {
		fmt.Printf("  %s is not installed on the VM.\n", agentType)
		fmt.Println()
		fmt.Println("  Install it with:")
		fmt.Println("    cloudcoop provision")
		return nil
	}

	// Check for successful auth (our test prompt returns AUTH_OK)
	if strings.Contains(output, "AUTH_OK") {
		fmt.Println("  ✓ Authenticated")
		return nil
	}

	// Check for common auth error messages
	if strings.Contains(output, "Invalid API key") ||
		strings.Contains(output, "Please run /login") ||
		strings.Contains(output, "authentication") ||
		err != nil {
		fmt.Println("  ✗ Not authenticated")
		fmt.Println()
		fmt.Println("  To authenticate, start an agent session and follow the login prompt:")
		fmt.Println("    cloudcoop agents add")
		fmt.Println("    cloudcoop agents attach 0")
		return nil
	}

	// Unknown output - print it for debugging
	fmt.Printf("  Status unknown. Output:\n%s\n", output)
	return nil
}
