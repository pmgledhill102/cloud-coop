package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ops"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var provisionStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show VM provisioning status",
	Long: `Display the current provisioning status of the VM.

This shows:
- Whether provisioning is pending, running, completed, or failed
- Progress information when available
- Error details if provisioning failed`,
	RunE: runProvisionStatus,
}

func runProvisionStatus(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := configLoader()
	if err != nil {
		return handleConfigError(err)
	}

	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Create provider
	ctx, cancel := context.WithTimeout(cmd.Context(), ops.TimeoutProvision)
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
		fmt.Printf("VM %s: not found\n", cfg.VM.Name)
		return nil
	}

	if vmInfo.Status != cloud.VMStatusRunning {
		fmt.Printf("VM %s: %s (cannot check provisioning status)\n", cfg.VM.Name, vmInfo.Status)
		return nil
	}

	// Resolve SSH connection parameters
	ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	if err != nil {
		return fmt.Errorf("no IP address available for SSH")
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)

	// Connect via SSH
	client, err := ssh.NewClient(ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port))
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Check provisioning status
	status, err := provisioning.CheckStatus(client)
	if err != nil {
		return fmt.Errorf("check provisioning status: %w", err)
	}

	// Display status
	fmt.Printf("VM %s provisioning status:\n\n", cfg.VM.Name)
	fmt.Printf("  Status: %s\n", formatProvisionStatus(status))
	if status.Progress != "" && status.Status == provisioning.StatusRunning {
		fmt.Printf("  Progress: %s\n", status.Progress)
	}
	if status.Error != "" {
		fmt.Printf("  Error: %s\n", status.Error)
	}

	return nil
}

// formatProvisionStatus returns a formatted string for the provisioning status.
func formatProvisionStatus(info *provisioning.StatusInfo) string {
	switch info.Status {
	case provisioning.StatusPending:
		return "○ pending"
	case provisioning.StatusRunning:
		return "◐ running"
	case provisioning.StatusCompleted:
		return "● completed"
	case provisioning.StatusFailed:
		return "✗ failed"
	default:
		return "? unknown"
	}
}
