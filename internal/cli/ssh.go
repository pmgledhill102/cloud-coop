package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
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

var sshUser string

func init() {
	sshCmd.Flags().StringVarP(&sshUser, "user", "u", "", "SSH username (default: current user)")
}

func runSSH(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
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

	// Determine user
	user := sshUser
	if user == "" {
		user = os.Getenv("USER")
		if user == "" {
			user = "root"
		}
	}

	log.Debug("connecting via SSH", "host", host, "user", user)

	// Connect and run command
	client, err := ssh.NewClient(ssh.DefaultConfig(host, user))
	if err != nil {
		return handleSSHError(err, host)
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

func handleSSHError(err error, host string) error {
	errStr := err.Error()

	if strings.Contains(errStr, "no SSH authentication methods") {
		fmt.Fprintln(os.Stderr, "No SSH authentication methods available.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Ensure you have:")
		fmt.Fprintln(os.Stderr, "  1. SSH agent running with key loaded (ssh-add), OR")
		fmt.Fprintln(os.Stderr, "  2. Unencrypted key at ~/.ssh/id_ed25519")
		return nil
	}

	if strings.Contains(errStr, "connection refused") {
		fmt.Fprintf(os.Stderr, "Connection refused to %s.\n", host)
		fmt.Fprintln(os.Stderr, "Check VM is running and firewall allows SSH.")
		return nil
	}

	return err
}
