package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var (
	provisionLogsFollow bool
	provisionLogsTail   int
)

var provisionLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show VM provisioning logs",
	Long: `Display the provisioning log from the VM.

This shows the full provisioning log from /var/log/cloudcoop/provision.log.

Use -f/--follow to continuously stream new log entries (like tail -f).
Use -n/--tail to show only the last N lines.`,
	RunE: runProvisionLogs,
}

func init() {
	provisionLogsCmd.Flags().BoolVarP(&provisionLogsFollow, "follow", "f", false, "Follow log output (like tail -f)")
	provisionLogsCmd.Flags().IntVarP(&provisionLogsTail, "tail", "n", 0, "Show only the last N lines (0 = show all)")
}

func runProvisionLogs(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := configLoader()
	if err != nil {
		return handleConfigError(err)
	}

	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Create provider
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
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

	// Check VM state
	if vmInfo.Status == cloud.VMStatusNotFound {
		return fmt.Errorf("VM %s: not found", cfg.VM.Name)
	}

	if vmInfo.Status != cloud.VMStatusRunning {
		return fmt.Errorf("VM %s: %s (must be running to view logs)", cfg.VM.Name, vmInfo.Status)
	}

	// Resolve SSH connection parameters
	ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	if err != nil {
		return fmt.Errorf("no IP address available for SSH")
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	port := ssh.ResolvePort(cfg.SSH.Port)

	// Build the command
	logFile := "/var/log/cloudcoop/provision.log"
	var remoteCmd string
	if provisionLogsFollow {
		if provisionLogsTail > 0 {
			remoteCmd = fmt.Sprintf("tail -n %d -f %s", provisionLogsTail, logFile)
		} else {
			remoteCmd = fmt.Sprintf("tail -f %s", logFile)
		}
	} else {
		if provisionLogsTail > 0 {
			remoteCmd = fmt.Sprintf("tail -n %d %s", provisionLogsTail, logFile)
		} else {
			remoteCmd = fmt.Sprintf("cat %s", logFile)
		}
	}

	// For follow mode, use interactive SSH (shells out to ssh command)
	if provisionLogsFollow {
		if err := ssh.CheckSSHAvailable(); err != nil {
			return err
		}

		// Ensure host key is in cloudcoop's managed known_hosts
		vm := ssh.NewVMIdentity(vmInfo.Name, vmInfo.CloudcoopCreated)
		if err := ssh.EnsureHostKeyPinned(ip, port, vm); err != nil {
			return fmt.Errorf("fetch host key: %w", err)
		}

		knownHostsPath, err := ssh.CloudcoopKnownHostsPath()
		if err != nil {
			return fmt.Errorf("get known_hosts path: %w", err)
		}

		sshArgs := []string{
			"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
			"-t", // Force PTY allocation
			"-p", fmt.Sprintf("%d", port),
			fmt.Sprintf("%s@%s", sshUser, ip),
			remoteCmd,
		}

		sshCmd := exec.Command("ssh", sshArgs...)
		sshCmd.Stdin = os.Stdin
		sshCmd.Stdout = os.Stdout
		sshCmd.Stderr = os.Stderr

		return sshCmd.Run()
	}

	// For non-follow mode, use SSH client
	sshCfg := ssh.SetupClientConfig(ip, sshUser, port)
	sshCfg.VM = ssh.NewVMIdentity(vmInfo.Name, vmInfo.CloudcoopCreated)
	client, err := ssh.NewClient(sshCfg)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	output, err := client.Run(remoteCmd)
	if err != nil {
		// Check if the log file doesn't exist
		if output == "" {
			return fmt.Errorf("provisioning log not found (provisioning may not have started)")
		}
		return fmt.Errorf("failed to read provisioning log: %w", err)
	}

	fmt.Print(output)
	return nil
}
