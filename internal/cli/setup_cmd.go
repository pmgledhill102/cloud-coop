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
	"github.com/cloud-coop/cloudcoop/internal/ops"
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
	Short: "Set up a cloud project for cloudcoop",
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

	// Check if we already have a project configured (instance config takes priority)
	existingProject := ""
	instancePath := config.InstanceConfigPath(".")
	if existingCfg, loadErr := config.LoadFile(instancePath); loadErr == nil {
		existingProject = existingCfg.Cloud.GCP.Project
	}
	if existingProject == "" {
		projectPath := config.ProjectConfigPath(".")
		if existingCfg, loadErr := config.LoadFile(projectPath); loadErr == nil {
			existingProject = existingCfg.Cloud.GCP.Project
		}
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

	// Resolve network name: flag → merged config (global + project) → "default"
	network := flagNetwork
	if network == "" {
		if merged, mergeErr := config.LoadMerged(); mergeErr == nil {
			network = merged.VM.Network
		}
	}
	if network == "" {
		network = "default"
	}

	// Resolve SSH port from merged config (default: 22)
	sshPort := 22
	if merged, mergeErr := config.LoadMerged(); mergeErr == nil && merged.SSH.Port != 0 {
		sshPort = merged.SSH.Port
	}

	// Load merged config for extra APIs/IAM roles
	mergedCfg, _ := config.LoadMerged()
	var extraAPIs, extraRoles []string
	if mergedCfg != nil {
		extraAPIs = mergedCfg.Setup.ExtraAPIs
		extraRoles = mergedCfg.Setup.ExtraIAMRoles
	}
	apis := setup.MergedAPIs(extraAPIs)
	iamRoles := setup.MergedIAMRoles(extraRoles)

	fmt.Println()
	fmt.Printf("Checking project %q...\n", projectID)

	// Derive service account name from repo directory
	saName := saNameDeriver(".")

	// Phase 1: Check current state
	apiStatuses, err := provider.CheckAPIs(ctx, projectID, apis)
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
		for _, role := range iamRoles {
			bound, bindErr := provider.CheckIAMBinding(ctx, projectID, saMember, role)
			if bindErr != nil {
				return fmt.Errorf("check IAM binding: %w", bindErr)
			}
			iamBindings[role] = bound
		}
	}

	// Check IAP firewall rule
	fwExists, err := provider.FirewallRuleExists(ctx, projectID, setup.IAPFirewallRuleName)
	if err != nil {
		return fmt.Errorf("check firewall rule: %w", err)
	}

	// Check IAP firewall port if rule exists
	var fwPort int
	var needsFWUpdate bool
	if fwExists {
		fwPort, err = provider.GetFirewallRulePort(ctx, projectID, setup.IAPFirewallRuleName)
		if err != nil {
			return fmt.Errorf("check firewall port: %w", err)
		}
		needsFWUpdate = fwPort != sshPort
	}

	// Check direct SSH firewall rule
	directFWExists, err := provider.FirewallRuleExists(ctx, projectID, setup.DirectSSHFirewallRuleName)
	if err != nil {
		return fmt.Errorf("check direct SSH firewall rule: %w", err)
	}

	// Detect public IP for direct SSH rule
	publicIP, ipErr := ops.PublicIPDetector(ctx)

	var directFWSourceIP string
	var directFWPort int
	var needsDirectFWUpdate bool
	if directFWExists {
		directFWSourceIP, directFWPort, err = provider.GetFirewallRuleSourceIP(ctx, projectID, setup.DirectSSHFirewallRuleName)
		if err != nil {
			return fmt.Errorf("check direct SSH firewall: %w", err)
		}
		if ipErr == nil {
			needsDirectFWUpdate = directFWSourceIP != publicIP || directFWPort != sshPort
		}
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
		for _, role := range iamRoles {
			if iamBindings[role] {
				fmt.Printf("  [ok] IAM %s (bound)\n", role)
			} else {
				fmt.Printf("  [--] IAM %s (not bound)\n", role)
			}
		}
	}

	if fwExists {
		if needsFWUpdate {
			fmt.Printf("  [!!] Firewall rule %s (port %d, expected %d)\n", setup.IAPFirewallRuleName, fwPort, sshPort)
		} else {
			fmt.Printf("  [ok] Firewall rule %s (port %d)\n", setup.IAPFirewallRuleName, fwPort)
		}
	} else {
		fmt.Printf("  [--] Firewall rule %s (not found)\n", setup.IAPFirewallRuleName)
	}

	if ipErr != nil {
		fmt.Printf("  [--] Firewall rule %s (could not detect public IP)\n", setup.DirectSSHFirewallRuleName)
	} else if directFWExists {
		if needsDirectFWUpdate {
			fmt.Printf("  [!!] Firewall rule %s (IP %s port %d, expected %s port %d)\n",
				setup.DirectSSHFirewallRuleName, directFWSourceIP, directFWPort, publicIP, sshPort)
		} else {
			fmt.Printf("  [ok] Firewall rule %s (IP %s port %d)\n", setup.DirectSSHFirewallRuleName, directFWSourceIP, directFWPort)
		}
	} else {
		fmt.Printf("  [--] Firewall rule %s (not found)\n", setup.DirectSSHFirewallRuleName)
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
		for _, role := range iamRoles {
			if !iamBindings[role] {
				rolesToGrant = append(rolesToGrant, role)
			}
		}
	} else {
		rolesToGrant = iamRoles
	}

	needsSA := !saExists
	needsFW := !fwExists
	needsDirectFW := !directFWExists && ipErr == nil

	// If everything is in place, skip to config phase
	if len(apisToEnable) == 0 && !needsSA && len(rolesToGrant) == 0 && !needsFW && !needsFWUpdate && !needsDirectFW && !needsDirectFWUpdate {
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
			fmt.Printf("    - %s (allow SSH port %d from Google IAP)\n", setup.IAPFirewallRuleName, sshPort)
			fmt.Println()
		}

		if needsFWUpdate {
			fmt.Println("  Update firewall rule:")
			fmt.Printf("    - %s (change port %d → %d)\n", setup.IAPFirewallRuleName, fwPort, sshPort)
			fmt.Println()
		}

		if needsDirectFW {
			fmt.Println("  Create firewall rule:")
			fmt.Printf("    - %s (allow SSH port %d from %s)\n", setup.DirectSSHFirewallRuleName, sshPort, publicIP)
			fmt.Println()
		}

		if needsDirectFWUpdate {
			fmt.Println("  Update firewall rule:")
			fmt.Printf("    - %s (IP %s → %s, port %d)\n", setup.DirectSSHFirewallRuleName, directFWSourceIP, publicIP, sshPort)
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
			fmt.Printf("Creating firewall rule (port %d)...", sshPort)
			if fwErr := provider.CreateIAPFirewallRule(ctx, projectID, network, sshPort); fwErr != nil {
				fmt.Println(" failed")
				return fmt.Errorf("create firewall rule: %w", fwErr)
			}
			fmt.Println(" done")
		}

		if needsFWUpdate {
			fmt.Printf("Updating firewall rule (port %d → %d)...", fwPort, sshPort)
			if fwErr := provider.UpdateIAPFirewallRule(ctx, projectID, sshPort); fwErr != nil {
				fmt.Println(" failed")
				return fmt.Errorf("update firewall rule: %w", fwErr)
			}
			fmt.Println(" done")
		}

		if needsDirectFW {
			fmt.Printf("Creating direct SSH firewall rule (%s port %d)...", publicIP, sshPort)
			if fwErr := provider.CreateDirectSSHFirewallRule(ctx, projectID, network, publicIP, sshPort); fwErr != nil {
				fmt.Println(" failed")
				return fmt.Errorf("create direct SSH firewall rule: %w", fwErr)
			}
			fmt.Println(" done")
		}

		if needsDirectFWUpdate {
			fmt.Printf("Updating direct SSH firewall rule (%s port %d)...", publicIP, sshPort)
			if fwErr := provider.UpdateDirectSSHFirewallRule(ctx, projectID, publicIP, sshPort); fwErr != nil {
				fmt.Println(" failed")
				return fmt.Errorf("update direct SSH firewall rule: %w", fwErr)
			}
			fmt.Println(" done")
		}
	}

	fmt.Println()

	// Phase 2: Project config — resolve zone and VM name from flag → merged config → prompt
	mergedForPhase2, _ := config.LoadMerged()

	zone := flagZone
	if zone == "" && mergedForPhase2 != nil && mergedForPhase2.Cloud.GCP.Zone != "" {
		zone = mergedForPhase2.Cloud.GCP.Zone
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
	if mergedForPhase2 != nil && mergedForPhase2.VM.Name != "" {
		vmName = mergedForPhase2.VM.Name
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
		fmt.Println("Dry run - would save config to .cloudcoop/local.toml:")
		fmt.Printf("  cloud.gcp.project = %q\n", projectID)
		fmt.Printf("  cloud.gcp.zone = %q\n", zone)
		fmt.Printf("  cloud.gcp.service_account = %q\n", saEmail)
		fmt.Printf("  vm.name = %q\n", vmName)
		return nil
	}

	// Save instance config (per-developer, gitignored)
	if err := config.SaveInstance(".", projectID, zone, saEmail, vmName); err != nil {
		return fmt.Errorf("save instance config: %w", err)
	}

	fmt.Println()
	fmt.Printf("Configuration saved to %s\n", config.InstanceConfigPath("."))
	fmt.Println()
	fmt.Println("Setup complete! Next steps:")
	fmt.Println("  cloudcoop create    # Create your VM")
	fmt.Println("  cloudcoop           # Launch TUI")

	return nil
}
