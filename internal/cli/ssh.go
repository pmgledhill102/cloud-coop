package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var sshCmd = &cobra.Command{
	Use:   "ssh <command>",
	Short: "Execute a command on the VM via SSH",
	Long: `Execute a command on the configured VM via SSH.

Examples:
  cloudcoop ssh hostname
  cloudcoop ssh "cat /etc/os-release"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSSH,
}

var (
	sshUser string
	sshPort int
)

func init() {
	sshCmd.Flags().StringVarP(&sshUser, "user", "u", "", "SSH username (default: from config or current user)")
	sshCmd.Flags().IntVarP(&sshPort, "port", "p", 0, "SSH port (default: from config or 22)")
}

func runSSH(cmd *cobra.Command, args []string) error {
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

	vmInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		return fmt.Errorf("get VM info: %w", err)
	}

	if vmInfo.Status != cloud.VMStatusRunning {
		return fmt.Errorf("VM %s is not running (status: %s)", cfg.VM.Name, vmInfo.Status)
	}

	host := vmInfo.ExternalIP
	if host == "" {
		return fmt.Errorf("VM %s has no external IP address", cfg.VM.Name)
	}

	// Determine user (flag > config > env > default)
	user := sshUser
	if user == "" {
		user = cfg.SSH.User
	}
	if user == "" {
		user = os.Getenv("USER")
	}
	if user == "" {
		user = "root"
	}

	// Determine port (flag > config > default)
	port := sshPort
	if port == 0 {
		port = cfg.SSH.Port
	}

	log.Debug("connecting via SSH", "host", host, "port", port, "user", user)

	// Connect and run command
	sshCfg := ssh.DefaultConfig(host, user)
	sshCfg.Port = port
	client, err := ssh.NewClient(sshCfg)
	if err != nil {
		return handleSSHError(err, host, port)
	}
	defer func() { _ = client.Close() }()

	command := strings.Join(args, " ")
	output, err := client.Run(command)
	if output != "" {
		fmt.Print(output)
	}
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

func handleSSHError(err error, host string, port int) error {
	errStr := err.Error()
	hostPort := fmt.Sprintf("%s:%d", host, port)

	if strings.Contains(errStr, "no SSH authentication methods") {
		fmt.Fprintln(os.Stderr, "No SSH authentication methods available.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Ensure you have:")
		fmt.Fprintln(os.Stderr, "  1. SSH agent running with key loaded (ssh-add), OR")
		fmt.Fprintln(os.Stderr, "  2. Unencrypted key at ~/.ssh/id_ed25519")
		return nil
	}

	if strings.Contains(errStr, "connection refused") {
		fmt.Fprintf(os.Stderr, "Connection refused to %s.\n", hostPort)
		fmt.Fprintln(os.Stderr, "Check VM is running and firewall allows SSH.")
		return nil
	}

	if strings.Contains(errStr, "knownhosts: key is unknown") {
		fmt.Fprintf(os.Stderr, "Host key for %s is not in known_hosts.\n", host)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "To add the host key, run:")
		fmt.Fprintf(os.Stderr, "  ssh-keyscan -p %d %s >> ~/.ssh/known_hosts\n", port, host)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Or connect once with ssh to accept the key:")
		if port == 22 {
			fmt.Fprintf(os.Stderr, "  ssh %s\n", host)
		} else {
			fmt.Fprintf(os.Stderr, "  ssh -p %d %s\n", port, host)
		}
		return nil
	}

	if strings.Contains(errStr, "i/o timeout") || strings.Contains(errStr, "connection timed out") {
		fmt.Fprintf(os.Stderr, "Connection to %s timed out.\n", hostPort)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Possible causes:")
		fmt.Fprintf(os.Stderr, "  - Firewall blocking SSH (port %d)\n", port)
		fmt.Fprintln(os.Stderr, "  - VM not fully started yet")
		fmt.Fprintln(os.Stderr, "  - Network connectivity issues")
		return nil
	}

	if strings.Contains(errStr, "unable to authenticate") || strings.Contains(errStr, "no supported methods remain") {
		fmt.Fprintf(os.Stderr, "SSH authentication to %s failed.\n", hostPort)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Ensure you have:")
		fmt.Fprintln(os.Stderr, "  1. SSH key loaded in agent (ssh-add ~/.ssh/your_key)")
		fmt.Fprintln(os.Stderr, "  2. Public key added to VM's authorized_keys")
		return nil
	}

	return err
}
