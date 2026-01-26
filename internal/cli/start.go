package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
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
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return handleConfigError(err)
	}

	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Create provider with 60s timeout (start operations take 20-40s)
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
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
	case cloud.VMStatusRunning:
		fmt.Printf("VM %s is already running.\n", cfg.VM.Name)
		return nil
	case cloud.VMStatusStarting:
		fmt.Printf("VM %s is already starting...\n", cfg.VM.Name)
		return nil
	case cloud.VMStatusStopping:
		return fmt.Errorf("VM %s is currently stopping, please wait and try again", cfg.VM.Name)
	case cloud.VMStatusNotFound:
		return fmt.Errorf("VM %s not found", cfg.VM.Name)
	case cloud.VMStatusStopped:
		// Proceed with start
	default:
		return fmt.Errorf("VM %s is in unexpected state: %s", cfg.VM.Name, vmInfo.Status)
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
