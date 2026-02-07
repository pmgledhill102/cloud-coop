package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/log"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new VM",
	Long: `Create a new VM instance with the configured settings.

If no size is specified, 'small' is used by default.
Available sizes are defined in the config file under vm.machine_sizes.`,
	RunE: runCreate,
}

var createSize string

func init() {
	createCmd.Flags().StringVarP(&createSize, "size", "s", "small", "VM size (small, medium, large, xlarge)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := configLoader()
	if err != nil {
		return handleConfigError(err)
	}

	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Validate the size
	machineType, ok := cfg.VM.MachineSizes[createSize]
	if !ok {
		var sizes []string
		for k := range cfg.VM.MachineSizes {
			sizes = append(sizes, k)
		}
		sort.Strings(sizes)
		return fmt.Errorf("invalid size %q, available sizes: %v", createSize, sizes)
	}

	// Create provider with 300s timeout (create operations can take a while)
	ctx, cancel := context.WithTimeout(cmd.Context(), 300*time.Second)
	defer cancel()

	provider, cleanup, err := createProvider(ctx, cfg)
	if err != nil {
		return handleProviderError(err)
	}
	defer cleanup()

	// Check if VM already exists
	log.Debug("checking VM status", "name", cfg.VM.Name, "provider", provider.Name())
	vmInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		return fmt.Errorf("get VM status: %w", err)
	}

	// Handle states
	switch vmInfo.Status {
	case cloud.VMStatusNotFound:
		// Proceed with create
	case cloud.VMStatusRunning, cloud.VMStatusStopped:
		fmt.Printf("VM %s already exists (status: %s).\n", cfg.VM.Name, vmInfo.Status)
		return nil
	default:
		return fmt.Errorf("VM %s is in state: %s, cannot create", cfg.VM.Name, vmInfo.Status)
	}

	// Build create config
	createCfg := cloud.VMCreateConfig{
		Name:               cfg.VM.Name,
		MachineType:        machineType,
		DiskSizeGB:         cfg.VM.DiskSizeGB,
		Image:              cfg.VM.Image,
		Spot:               cfg.VM.Spot,
		Network:            cfg.VM.Network,
		Subnet:             cfg.VM.Subnet,
		Tags:               cfg.VM.Tags,
		SSHPort:            cfg.SSH.Port,
		ServiceAccount:     cfg.Cloud.GCP.ServiceAccount,
		ProvisionScriptURL: cfg.Provisioning.ScriptURL,
	}

	// Create the VM
	fmt.Printf("Creating VM %s (size: %s, machine: %s, ssh port: %d)...\n", cfg.VM.Name, createSize, machineType, cfg.SSH.Port)
	log.Debug("creating VM",
		"name", createCfg.Name,
		"machineType", createCfg.MachineType,
		"diskSizeGB", createCfg.DiskSizeGB,
		"image", createCfg.Image,
		"spot", createCfg.Spot,
		"network", createCfg.Network,
		"tags", createCfg.Tags,
		"sshPort", createCfg.SSHPort,
	)

	if err := provider.CreateVM(ctx, createCfg); err != nil {
		return fmt.Errorf("create VM: %w", err)
	}

	// Get new status
	newInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		fmt.Println("VM create initiated.")
		return nil
	}

	fmt.Printf("VM %s: %s\n", cfg.VM.Name, formatStatus(newInfo.Status))
	if newInfo.ExternalIP != "" {
		fmt.Printf("External IP: %s\n", newInfo.ExternalIP)
	}
	return nil
}
