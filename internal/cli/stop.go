package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/log"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a running VM",
	Long: `Stop a running VM instance.

If the VM is already stopped, this command does nothing.
If the VM is in a transitional state (starting/stopping), an error is returned.`,
	RunE: runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := configLoader()
	if err != nil {
		return handleConfigError(err)
	}

	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Create provider with 180s timeout (stop operations can take 90s+ on laden VMs)
	ctx, cancel := context.WithTimeout(cmd.Context(), 180*time.Second)
	defer cancel()

	provider, cleanup, err := createProvider(ctx, cfg)
	if err != nil {
		return handleProviderError(err)
	}
	defer cleanup()

	// Get current VM status for idempotency check
	log.Debug("checking VM status", "name", cfg.VM.Name, "provider", provider.Name())
	vmInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		return fmt.Errorf("get VM status: %w", err)
	}

	// Handle states
	switch vmInfo.Status {
	case cloud.VMStatusStopped:
		fmt.Printf("VM %s is already stopped.\n", cfg.VM.Name)
		return nil
	case cloud.VMStatusStopping:
		fmt.Printf("VM %s is already stopping...\n", cfg.VM.Name)
		return nil
	case cloud.VMStatusStarting:
		return fmt.Errorf("VM %s is currently starting, please wait and try again", cfg.VM.Name)
	case cloud.VMStatusNotFound:
		return fmt.Errorf("VM %s not found", cfg.VM.Name)
	case cloud.VMStatusRunning:
		// Proceed with stop
	default:
		return fmt.Errorf("VM %s is in unexpected state: %s", cfg.VM.Name, vmInfo.Status)
	}

	// Stop the VM
	fmt.Printf("Stopping VM %s...\n", cfg.VM.Name)
	if err := provider.StopVM(ctx, cfg.VM.Name); err != nil {
		return fmt.Errorf("stop VM: %w", err)
	}

	// Get new status
	newInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		fmt.Println("VM stop initiated.")
		return nil
	}

	fmt.Printf("VM %s: %s\n", cfg.VM.Name, formatStatus(newInfo.Status))
	return nil
}
