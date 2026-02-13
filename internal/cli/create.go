package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ops"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new VM",
	Long: `Create a new VM instance with the configured settings.

If no size is specified, 'small' is used by default.
Available sizes are defined in the config file under vm.machine_sizes.`,
	RunE: runCreate,
}

var (
	createSize      string
	createMaxUptime int
)

func init() {
	createCmd.Flags().StringVarP(&createSize, "size", "s", "small", "VM size (small, medium, large, xlarge)")
	createCmd.Flags().IntVar(&createMaxUptime, "max-uptime", 0, "Auto-stop VM after N minutes (0=disabled)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	cfg, err := configFromCmd(cmd)
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

	// Create provider with create timeout (can take several minutes)
	ctx, cancel := context.WithTimeout(cmd.Context(), ops.TimeoutVMCreate)
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

	// Validate state
	if err := ops.ValidateForCreate(vmInfo); err != nil {
		if errors.Is(err, ops.ErrVMExists) {
			fmt.Printf("VM %s already exists (status: %s).\n", cfg.VM.Name, vmInfo.Status)
			return nil
		}
		return fmt.Errorf("VM %s: %w", cfg.VM.Name, err)
	}

	// Read SSH public key (non-fatal if missing — key can be pushed later)
	pubKey, _ := ssh.ReadPublicKey()
	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)

	// Build create config
	maxUptime := cfg.VM.MaxUptimeMinutes
	if cmd.Flags().Changed("max-uptime") {
		maxUptime = createMaxUptime
	}

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
		SSHUser:            sshUser,
		SSHPublicKey:       pubKey,
		ServiceAccount:     cfg.Cloud.GCP.ServiceAccount,
		ProvisionScriptURL: cfg.Provisioning.ScriptURL,
		MaxUptimeMinutes:   maxUptime,
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

		// Wait for SSH to become reachable (non-fatal).
		fmt.Print("Waiting for SSH...")
		waitCfg := ssh.SetupClientConfig(newInfo.ExternalIP, sshUser, cfg.SSH.Port)
		waitCfg.VM = ssh.NewVMIdentity(newInfo.Name, newInfo.CloudcoopCreated)
		if err := ssh.WaitForSSH(waitCfg, 30*time.Second); err != nil {
			fmt.Printf(" not ready (%v)\n", err)
		} else {
			fmt.Println(" ready")
		}
	}
	return nil
}
