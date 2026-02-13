package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ops"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a VM",
	Long: `Delete a VM instance and its boot disk.

The VM must be stopped before it can be deleted.
This action is irreversible and will permanently delete the VM and its data.`,
	RunE: runDelete,
}

var deleteForce bool

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "skip confirmation prompt")
}

func runDelete(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := configLoader()
	if err != nil {
		return handleConfigError(err)
	}

	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Create provider with lifecycle timeout
	ctx, cancel := context.WithTimeout(cmd.Context(), ops.TimeoutVMLifecycle)
	defer cancel()

	provider, cleanup, err := createProvider(ctx, cfg)
	if err != nil {
		return handleProviderError(err)
	}
	defer cleanup()

	// Check current VM status
	log.Debug("checking VM status", "name", cfg.VM.Name, "provider", provider.Name())
	vmInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		return fmt.Errorf("get VM status: %w", err)
	}

	// Validate state
	if err := ops.ValidateForDelete(vmInfo); err != nil {
		if errors.Is(err, ops.ErrVMNotFound) {
			fmt.Printf("VM %s does not exist.\n", cfg.VM.Name)
			return nil
		}
		if errors.Is(err, ops.ErrVMRunning) {
			return fmt.Errorf("VM %s is running, stop it first with 'cloudcoop stop'", cfg.VM.Name)
		}
		return fmt.Errorf("VM %s: %w", cfg.VM.Name, err)
	}

	// Confirm deletion unless --force is used
	if !deleteForce {
		fmt.Printf("Delete VM %s? This will permanently delete the VM and its boot disk.\n", cfg.VM.Name)
		fmt.Print("Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read confirmation: %w", err)
		}

		if strings.TrimSpace(response) != "yes" {
			fmt.Println("Canceled.")
			return nil
		}
	}

	// Delete the VM
	fmt.Printf("Deleting VM %s...\n", cfg.VM.Name)
	if err := provider.DeleteVM(ctx, cfg.VM.Name); err != nil {
		return fmt.Errorf("delete VM: %w", err)
	}

	// Clean up pinned host key for the deleted VM.
	_ = ssh.ClearPinnedKey(cfg.VM.Name)

	// Verify deletion
	newInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		fmt.Println("VM delete initiated.")
		return nil
	}

	if newInfo.Status == cloud.VMStatusNotFound {
		fmt.Printf("VM %s deleted successfully.\n", cfg.VM.Name)
	} else {
		fmt.Printf("VM %s: %s\n", cfg.VM.Name, formatStatus(newInfo.Status))
	}
	return nil
}
