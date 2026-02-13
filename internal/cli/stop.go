package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ops"
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

	// Create provider with lifecycle timeout (stop operations can take 90s+ on laden VMs)
	ctx, cancel := context.WithTimeout(cmd.Context(), ops.TimeoutVMLifecycle)
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

	// Validate state
	if err := ops.ValidateForStop(vmInfo); err != nil {
		if errors.Is(err, ops.ErrVMAlreadyStopped) {
			fmt.Printf("VM %s is already stopped.\n", cfg.VM.Name)
			return nil
		}
		if errors.Is(err, ops.ErrVMStopping) {
			fmt.Printf("VM %s is already stopping...\n", cfg.VM.Name)
			return nil
		}
		return fmt.Errorf("VM %s: %w", cfg.VM.Name, err)
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
