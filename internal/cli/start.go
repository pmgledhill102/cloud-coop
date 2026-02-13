package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ops"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a stopped VM",
	Long: `Start a stopped VM instance.

If the VM is already running, this command does nothing.
If the VM is in a transitional state (starting/stopping), an error is returned.`,
	RunE: runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	cfg, err := configFromCmd(cmd)
	if err != nil {
		return handleConfigError(err)
	}

	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Create provider with lifecycle timeout (start operations can take 60-90s)
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
	if err := ops.ValidateForStart(vmInfo); err != nil {
		if errors.Is(err, ops.ErrVMAlreadyRunning) {
			fmt.Printf("VM %s is already running.\n", cfg.VM.Name)
			return nil
		}
		if errors.Is(err, ops.ErrVMStarting) {
			fmt.Printf("VM %s is already starting...\n", cfg.VM.Name)
			return nil
		}
		return fmt.Errorf("VM %s: %w", cfg.VM.Name, err)
	}

	// Start the VM
	fmt.Printf("Starting VM %s...\n", cfg.VM.Name)
	if err := provider.StartVM(ctx, cfg.VM.Name); err != nil {
		return fmt.Errorf("start VM: %w", err)
	}

	// Get new status
	newInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		fmt.Println("VM start initiated.")
		return nil
	}

	fmt.Printf("VM %s: %s\n", cfg.VM.Name, formatStatus(newInfo.Status))
	return nil
}
