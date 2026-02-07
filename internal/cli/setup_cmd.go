package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/setup"
	gcpsetup "github.com/cloud-coop/cloudcoop/internal/setup/gcp"
)

// setupProviderFactory creates a setup provider. Injectable for testing.
var setupProviderFactory func(ctx context.Context) (setup.SetupProvider, error) = defaultSetupProviderFactory

// sshKeyChecker checks for SSH key presence. Injectable for testing.
var sshKeyChecker func() setup.PrereqStatus = setup.CheckSSHKey

// sshKeyGenerator generates an SSH key. Injectable for testing.
var sshKeyGenerator func() (string, error) = setup.GenerateSSHKey

// saNameDeriver derives a service account name from a directory. Injectable for testing.
var saNameDeriver func(dir string) string = setup.ServiceAccountNameForDir

func defaultSetupProviderFactory(ctx context.Context) (setup.SetupProvider, error) {
	return gcpsetup.New(ctx)
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up a GCP project for cloudcoop",
	Long: `Automate GCP project provisioning for cloudcoop.

This command checks prerequisites, enables required APIs, creates a service
account with minimal permissions, and writes project configuration.

All operations are idempotent - re-running setup will skip already-completed
steps and fix any gaps.

Examples:
  cloudcoop setup                           # Interactive setup
  cloudcoop setup --project my-project      # Skip project selection
  cloudcoop setup --dry-run                 # Show what would be done`,
	RunE: runSetup,
}

func init() {
	setupCmd.Flags().String("project", "", "GCP project ID (skip project selection)")
	setupCmd.Flags().String("zone", "", "GCP zone (skip zone prompt)")
	setupCmd.Flags().String("network", "", "VPC network name (default: \"default\")")
	setupCmd.Flags().Bool("dry-run", false, "show what would be done without making changes")
}

func runSetup(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	flagProject, _ := cmd.Flags().GetString("project")
	flagZone, _ := cmd.Flags().GetString("zone")
	flagNetwork, _ := cmd.Flags().GetString("network")

	fmt.Println("cloudcoop setup")
	fmt.Println("================")
	fmt.Println()

	// Phase 0: Check prerequisites
	fmt.Println("Checking prerequisites...")
	fmt.Println()

	sshStatus := sshKeyChecker()
	if sshStatus.OK {
		fmt.Printf("  [ok] SSH key found (%s)\n", sshStatus.Detail)
	} else {
		fmt.Printf("  [--] %s\n", sshStatus.Detail)
		fmt.Print("  Generating ed25519 SSH key...")
		pubPath, genErr := sshKeyGenerator()
		if genErr != nil {
			fmt.Println(" failed")
			return fmt.Errorf("generate SSH key: %w", genErr)
		}
		fmt.Println(" done")
		fmt.Printf("  [ok] SSH key created (%s)\n", pubPath)
	}

	// Create setup provider (tests ADC)
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()

	provider, err := setupProviderFactory(ctx)
	if err != nil {
		fmt.Println("  [!!] GCP credentials (Application Default Credentials)")
		fmt.Println()
		fmt.Println("  Run the following commands to authenticate:")
		fmt.Println()
		fmt.Println("    gcloud auth login")
		fmt.Println("    gcloud auth application-default login")
		fmt.Println()
		return nil
	}
	defer func() { _ = provider.Close() }()

	fmt.Println("  [ok] GCP credentials (Application Default Credentials)")
	fmt.Println()

	// Phase 1: Select project
	var projectID string

	// Check if we already have a project config
	existingProject := ""
	existingNetwork := ""
	projectPath := config.ProjectConfigPath(".")
	if existingCfg, loadErr := config.LoadFile(projectPath); loadErr == nil {
		existingProject = existingCfg.Cloud.GCP.Project
		existingNetwork = existingCfg.VM.Network
	}

	if flagProject != "" {
		projectID = flagProject
	} else if existingProject != "" {
		projectID = existingProject
		fmt.Printf("Using project from existing config: %s\n", projectID)
	} else {
		// List projects for selection
		projects, listErr := provider.ListProjects(ctx)
		if listErr != nil {
			return fmt.Errorf("list GCP projects: %w", listErr)
		}

		if len(projects) == 0 {
			fmt.Println("No GCP projects found.")
			fmt.Println("Create a project at: https://console.cloud.google.com/projectcreate")
			return nil
		}

		fmt.Println("Available GCP projects:")
		for i, p := range projects {
			if p.Name != "" {
				fmt.Printf("  %d) %s (%s)\n", i+1, p.ID, p.Name)
			} else {
				fmt.Printf("  %d) %s\n", i+1, p.ID)
			}
		}
		fmt.Println()

		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Select project [1]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		idx := 1
		if input != "" {
			parsed, parseErr := strconv.Atoi(input)
			if parseErr != nil || parsed < 1 || parsed > len(projects) {
				return fmt.Errorf("invalid selection: %s", input)
			}
			idx = parsed
		}
		projectID = projects[idx-1].ID
	}

	// Resolve network name: flag → existing config → global config → "default"
	network := flagNetwork
	if network == "" {
		network = existingNetwork
	}
	if network == "" {
		if merged, mergeErr := config.LoadMerged(); mergeErr == nil && merged.VM.Network != "" {
			network = merged.VM.Network
		}
	}
	if network == "" {
		network = "default"
	}

	fmt.Println()
	fmt.Printf("Checking project %q...\n", projectID)

	// Derive service account name from repo directory
	saName := saNameDeriver(".")

	// Phase 1: Check current state
	apiStatuses, err := provider.CheckAPIs(ctx, projectID)
	if err != nil {
		return fmt.Errorf("check APIs: %w", err)
	}

	saExists, err := provider.ServiceAccountExists(ctx, projectID, saName)
	if err != nil {
		return fmt.Errorf("check service account: %w", err)
	}

	saEmail := setup.ServiceAccountEmail(projectID, saName)
	saMember := "serviceAccount:" + saEmail

	// Check IAM bindings (only if SA exists)
	iamBindings := make(map[string]bool)
	if saExists {
		for _, role := range setup.RequiredIAMRoles {
			bound, bindErr := provider.CheckIAMBinding(ctx, projectID, saMember, role)
			if bindErr != nil {
				return fmt.Errorf("check IAM binding: %w", bindErr)
			}
			iamBindings[role] = bound
		}
	}

	// Check firewall rule
	fwExists, err := provider.FirewallRuleExists(ctx, projectID, setup.IAPFirewallRuleName)
	if err != nil {
		return fmt.Errorf("check firewall rule: %w", err)
	}

	// Display current state
	for _, s := range apiStatuses {
		if s.Enabled {
			fmt.Printf("  [ok] %s (already enabled)\n", s.Name)
		} else {
			fmt.Printf("  [--] %s (not enabled)\n", s.Name)
		}
	}

	if saExists {
		fmt.Printf("  [ok] Service account %s (exists)\n", saName)
	} else {
		fmt.Printf("  [--] Service account %s (not found)\n", saName)
	}

	if saExists {
		for _, role := range setup.RequiredIAMRoles {
			if iamBindings[role] {
				fmt.Printf("  [ok] IAM %s (bound)\n", role)
			} else {
				fmt.Printf("  [--] IAM %s (not bound)\n", role)
			}
		}
	}

	if fwExists {
		fmt.Printf("  [ok] Firewall rule %s (exists)\n", setup.IAPFirewallRuleName)
	} else {
		fmt.Printf("  [--] Firewall rule %s (not found)\n", setup.IAPFirewallRuleName)
	}

	fmt.Println()

	// Determine what needs to be done
	var apisToEnable []string
	for _, s := range apiStatuses {
		if !s.Enabled {
			apisToEnable = append(apisToEnable, s.Name)
		}
	}

	var rolesToGrant []string
	if saExists {
		for _, role := range setup.RequiredIAMRoles {
			if !iamBindings[role] {
				rolesToGrant = append(rolesToGrant, role)
			}
		}
	} else {
		rolesToGrant = setup.RequiredIAMRoles
	}

	needsSA := !saExists
	needsFW := !fwExists

	// If everything is in place, skip to config phase
	if len(apisToEnable) == 0 && !needsSA && len(rolesToGrant) == 0 && !needsFW {
		fmt.Println("All GCP resources are in place.")
	} else {
		// Show what will be done
		fmt.Println("The following changes will be made:")
		fmt.Println()

		if len(apisToEnable) > 0 {
			fmt.Println("  Enable APIs:")
			for _, api := range apisToEnable {
				fmt.Printf("    - %s\n", api)
			}
			fmt.Println()
		}

		if needsSA {
			fmt.Println("  Create service account:")
			fmt.Printf("    - %s\n", saEmail)
			fmt.Println()
		}

		if len(rolesToGrant) > 0 {
			fmt.Printf("  Grant IAM roles to %s:\n", saName)
			for _, role := range rolesToGrant {
				fmt.Printf("    - %s\n", role)
			}
			fmt.Println()
		}

		if needsFW {
			fmt.Println("  Create firewall rule:")
			fmt.Printf("    - %s (allow SSH from Google IAP)\n", setup.IAPFirewallRuleName)
			fmt.Println()
		}

		if dryRun {
			fmt.Println("Dry run - no changes made.")
			return nil
		}

		// Confirm
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Proceed? [y/N]: ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Canceled.")
			return nil
		}

		fmt.Println()

		// Execute changes
		if len(apisToEnable) > 0 {
			fmt.Print("Enabling APIs...")
			for _, api := range apisToEnable {
				if enableErr := provider.EnableAPI(ctx, projectID, api); enableErr != nil {
					fmt.Println(" failed")
					return fmt.Errorf("enable API %s: %w", api, enableErr)
				}
			}
			fmt.Println(" done")
		}

		if needsSA {
			fmt.Print("Creating service account...")
			email, createErr := provider.CreateServiceAccount(ctx, projectID, saName, setup.ServiceAccountDisplayName)
			if createErr != nil {
				fmt.Println(" failed")
				return fmt.Errorf("create service account: %w", createErr)
			}
			saEmail = email
			saMember = "serviceAccount:" + saEmail
			fmt.Println(" done")
		}

		if len(rolesToGrant) > 0 {
			fmt.Print("Granting IAM roles...")
			for _, role := range rolesToGrant {
				if grantErr := provider.GrantIAMRole(ctx, projectID, saMember, role); grantErr != nil {
					fmt.Println(" failed")
					return fmt.Errorf("grant IAM role %s: %w", role, grantErr)
				}
			}
			fmt.Println(" done")
		}

		if needsFW {
			fmt.Print("Creating firewall rule...")
			if fwErr := provider.CreateIAPFirewallRule(ctx, projectID, network); fwErr != nil {
				fmt.Println(" failed")
				return fmt.Errorf("create firewall rule: %w", fwErr)
			}
			fmt.Println(" done")
		}
	}

	fmt.Println()

	// Phase 2: Project config
	zone := flagZone
	if zone == "" && existingProject != "" {
		// Try to read from existing config
		if existingCfg, loadErr := config.LoadFile(projectPath); loadErr == nil && existingCfg.Cloud.GCP.Zone != "" {
			zone = existingCfg.Cloud.GCP.Zone
		}
	}

	if zone == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("GCP zone [us-central1-a]: ")
		input, _ := reader.ReadString('\n')
		zone = strings.TrimSpace(input)
		if zone == "" {
			zone = "us-central1-a"
		}
	}

	vmName := ""
	if existingProject != "" {
		if existingCfg, loadErr := config.LoadFile(projectPath); loadErr == nil && existingCfg.VM.Name != "" {
			vmName = existingCfg.VM.Name
		}
	}

	if vmName == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("VM instance name [claude-sandbox]: ")
		input, _ := reader.ReadString('\n')
		vmName = strings.TrimSpace(input)
		if vmName == "" {
			vmName = "claude-sandbox"
		}
	}

	if dryRun {
		fmt.Println()
		fmt.Println("Dry run - would save config to .cloudcoop/config.toml:")
		fmt.Printf("  cloud.gcp.project = %q\n", projectID)
		fmt.Printf("  cloud.gcp.zone = %q\n", zone)
		fmt.Printf("  cloud.gcp.service_account = %q\n", saEmail)
		fmt.Printf("  vm.name = %q\n", vmName)
		return nil
	}

	// Save project config
	if err := config.SaveProject(".", projectID, zone, saEmail, vmName); err != nil {
		return fmt.Errorf("save project config: %w", err)
	}

	fmt.Println()
	fmt.Printf("Configuration saved to %s\n", config.ProjectConfigPath("."))
	fmt.Println()
	fmt.Println("Setup complete! Next steps:")
	fmt.Println("  cloudcoop create    # Create your VM")
	fmt.Println("  cloudcoop           # Launch TUI")

	return nil
}
