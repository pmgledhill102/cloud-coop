package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/cloud/gcp"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
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

// providerFactory creates a cloud provider from configuration.
// This is a package-level variable to allow test injection.
var providerFactory func(ctx context.Context, cfg *config.Config) (cloud.Provider, func(), error) = createProviderImpl

// configLoader loads the application configuration.
// This is a package-level variable to allow test injection.
var configLoader func() (*config.Config, error) = config.LoadMerged

func runStatus(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := configLoader()
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
		return handleProviderError(err)
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

// createProvider creates a cloud provider using the configured factory.
func createProvider(ctx context.Context, cfg *config.Config) (cloud.Provider, func(), error) {
	return providerFactory(ctx, cfg)
}

// createProviderImpl is the default provider factory implementation.
func createProviderImpl(ctx context.Context, cfg *config.Config) (cloud.Provider, func(), error) {
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
		fmt.Fprintln(os.Stderr, "To get started, run the automated setup:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  cloudcoop setup")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Or create a configuration file manually:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  cloudcoop config init")
		fmt.Fprintln(os.Stderr)
		return nil // Don't propagate error, we've handled it
	}
	return err
}

func handleProviderError(err error) error {
	errStr := err.Error()

	// Check for credential errors
	if strings.Contains(errStr, "could not find default credentials") {
		fmt.Fprintln(os.Stderr, "GCP credentials not found.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Run the following commands to set up authentication:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  gcloud auth login")
		fmt.Fprintln(os.Stderr, "  gcloud auth application-default login")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "For more information, see:")
		fmt.Fprintln(os.Stderr, "  https://cloud.google.com/docs/authentication/application-default-credentials")
		fmt.Fprintln(os.Stderr)
		return nil
	}

	// Check for invalid credentials file
	if strings.Contains(errStr, "no such file or directory") && strings.Contains(errStr, "credentials") {
		fmt.Fprintln(os.Stderr, "GCP credentials file not found.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "If GOOGLE_APPLICATION_CREDENTIALS is set, verify the file exists.")
		fmt.Fprintln(os.Stderr, "Otherwise, run:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  gcloud auth application-default login")
		fmt.Fprintln(os.Stderr)
		return nil
	}

	// Default: return the error
	return fmt.Errorf("create cloud provider: %w", err)
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

	// Agents section
	fmt.Println("Agents:")
	if info.Status != cloud.VMStatusRunning {
		fmt.Println("  (VM not running)")
		return
	}

	// Query agents via SSH
	agentResult := queryAgents(cfg, info)
	if agentResult == nil {
		return
	}

	if agentResult.NoSession {
		fmt.Println("  No agents session")
	} else if len(agentResult.Sessions) == 0 {
		fmt.Println("  0 running")
	} else {
		for _, s := range agentResult.Sessions {
			fmt.Printf("  %d: %s (%s)\n", s.Index, s.Name, s.Command)
		}
		fmt.Printf("  (%d total)\n", len(agentResult.Sessions))
	}
}

func queryAgents(cfg *config.Config, info *cloud.VMInfo) *agent.ListResult {
	// Resolve SSH connection parameters using helpers
	ip, err := ssh.ResolveVMIP(info.ExternalIP, info.InternalIP)
	if err != nil {
		fmt.Println("  (no IP address for SSH)")
		return nil
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)

	// Connect via SSH
	client, err := ssh.NewClient(ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port))
	if err != nil {
		fmt.Printf("  (SSH error: %v)\n", err)
		return nil
	}
	defer func() { _ = client.Close() }()

	// List agent sessions
	result, err := agent.ListSessions(client, resolveSessionName())
	if err != nil {
		if errors.Is(err, agent.ErrTmuxNotInstalled) {
			fmt.Println("  (tmux not installed)")
		} else {
			fmt.Printf("  (error: %v)\n", err)
		}
		return nil
	}

	return result
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
