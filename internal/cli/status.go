package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/cloud/gcp"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of VMs and agents",
	Long: `Display the current status of cloud VMs and running agents.

This shows:
- Active VMs and their state (running, stopped, etc.)
- Running agents and their sessions
- Resource usage and costs`,
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return handleConfigError(err)
	}

	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Create provider
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	provider, cleanup, err := createProvider(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create cloud provider: %w", err)
	}
	defer cleanup()

	// Get VM info
	log.Debug("querying VM status", "name", cfg.VM.Name, "provider", provider.Name())
	vmInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		return fmt.Errorf("get VM status: %w", err)
	}

	// Display results
	printStatus(cfg, vmInfo)
	return nil
}

func createProvider(ctx context.Context, cfg *config.Config) (cloud.Provider, func(), error) {
	switch cfg.Cloud.Provider {
	case "gcp":
		p, err := gcp.New(ctx, cfg.Cloud.GCP.Project, cfg.Cloud.GCP.Zone)
		if err != nil {
			return nil, nil, err
		}
		return p, func() { _ = p.Close() }, nil
	default:
		return nil, nil, fmt.Errorf("unsupported provider: %s", cfg.Cloud.Provider)
	}
}

func handleConfigError(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) || errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "Configuration not found.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "To get started, create a configuration file at:")
		path, _ := config.DefaultConfigPath()
		fmt.Fprintf(os.Stderr, "  %s\n", path)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Example configuration:")
		fmt.Fprintln(os.Stderr, `
[cloud]
provider = "gcp"

[cloud.gcp]
project = "your-gcp-project-id"
zone = "us-central1-a"

[vm]
name = "your-vm-name"`)
		fmt.Fprintln(os.Stderr)
		return nil // Don't propagate error, we've handled it
	}
	return err
}

func printStatus(cfg *config.Config, info *cloud.VMInfo) {
	fmt.Println("cloudcoop status")
	fmt.Println()

	// Show cloud context
	fmt.Printf("Cloud:    %s\n", cfg.Cloud.Provider)
	if cfg.Cloud.Provider == "gcp" {
		fmt.Printf("Project:  %s\n", cfg.Cloud.GCP.Project)
	}
	fmt.Println()

	// Show VM status
	fmt.Println("VM:")
	if info.Status == cloud.VMStatusNotFound {
		fmt.Printf("  %s: not found\n", info.Name)
		fmt.Println()
		fmt.Println("  The configured VM does not exist.")
		fmt.Println("  Create it in the GCP Console or use 'cloudcoop create'.")
	} else {
		fmt.Printf("  Name:         %s\n", info.Name)
		fmt.Printf("  Status:       %s\n", formatStatus(info.Status))
		fmt.Printf("  Zone:         %s\n", info.Zone)
		fmt.Printf("  Machine Type: %s\n", info.MachineType)
		if info.ExternalIP != "" {
			fmt.Printf("  External IP:  %s\n", info.ExternalIP)
		}
		if info.InternalIP != "" {
			fmt.Printf("  Internal IP:  %s\n", info.InternalIP)
		}
	}
	fmt.Println()

	// Agents section (placeholder for future)
	fmt.Println("Agents:")
	if info.Status == cloud.VMStatusRunning {
		fmt.Println("  (querying agents not yet implemented)")
	} else {
		fmt.Println("  (VM not running)")
	}
}

func formatStatus(status cloud.VMStatus) string {
	switch status {
	case cloud.VMStatusRunning:
		return "● running"
	case cloud.VMStatusStopped:
		return "○ stopped"
	case cloud.VMStatusStarting:
		return "◐ starting..."
	case cloud.VMStatusStopping:
		return "◑ stopping..."
	default:
		return string(status)
	}
}
