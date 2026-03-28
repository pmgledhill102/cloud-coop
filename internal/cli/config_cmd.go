package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage cloudcoop configuration",
	Long: `View and modify cloudcoop configuration settings.

Configuration is stored in ~/.config/cloudcoop/cloudcoop.toml.

Examples:
  cloudcoop config get                        # Show all settings
  cloudcoop config get cloud.gcp.project      # Show specific setting
  cloudcoop config set cloud.gcp.project foo  # Set a value
  cloudcoop config init                       # Run setup wizard`,
}

var configShowCmd = &cobra.Command{
	Use:     "get [key]",
	Aliases: []string{"show"},
	Short:   "Display configuration settings",
	Long: `Display current configuration settings.

Without arguments, shows all configuration.
With a key argument, shows just that setting.

Examples:
  cloudcoop config get                   # Show all settings
  cloudcoop config get cloud.gcp.project # Show specific setting`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:         "set <key> <value>",
	Short:       "Set a configuration value",
	Annotations: map[string]string{"skip-config": "true"},
	Long: `Set a configuration value and save to the config file.

Available keys:
  cloud.provider         Cloud provider (gcp, aws, azure)
  cloud.gcp.project      GCP project ID
  cloud.gcp.zone         GCP zone (e.g., us-central1-a)
  vm.name                VM instance name
  ssh.port               SSH port (default: 22)
  ssh.user               SSH username (default: current user)
  agents.default_command Default command for new agents

Examples:
  cloudcoop config set cloud.gcp.project my-project
  cloudcoop config set cloud.gcp.zone us-central1-a
  cloudcoop config set vm.name claude-sandbox
  cloudcoop config set ssh.port 2222`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("requires key and value, e.g.: cloudcoop config set cloud.gcp.zone us-central1-a")
		}
		return cobra.ExactArgs(2)(cmd, args)
	},
	RunE: runConfigSet,
}

var configInitCmd = &cobra.Command{
	Use:         "init",
	Short:       "Initialise configuration with setup wizard",
	Annotations: map[string]string{"skip-config": "true"},
	Long: `Run the interactive setup wizard to create a new configuration file.

This will prompt you for:
  - GCP project ID
  - GCP zone
  - VM instance name

The configuration will be saved to ~/.config/cloudcoop/cloudcoop.toml.

For automated GCP project setup (enabling APIs, creating service accounts,
and IAM permissions), use 'cloudcoop setup' instead.

Example:
  cloudcoop config init`,
	RunE: runConfigInit,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configInitCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := configFromCmd(cmd)
	if err != nil {
		return handleConfigError(err)
	}

	// If specific key requested, show just that
	if len(args) == 1 {
		key := args[0]
		value, err := cfg.GetValue(key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unknown key: %s\n", key)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Available keys:")
			for _, k := range config.AllKeys() {
				fmt.Fprintf(os.Stderr, "  %s\n", k)
			}
			return nil
		}
		if value == "" {
			fmt.Println("(not set)")
		} else {
			fmt.Println(value)
		}
		return nil
	}

	// Show all configuration
	fmt.Println("cloudcoop configuration")
	fmt.Println()

	path, _ := config.DefaultConfigPath()
	fmt.Printf("Config file: %s\n", path)
	fmt.Println()

	fmt.Println("[cloud]")
	fmt.Printf("  provider = %q\n", cfg.Cloud.Provider)
	fmt.Println()

	fmt.Println("[cloud.gcp]")
	fmt.Printf("  project = %q\n", cfg.Cloud.GCP.Project)
	fmt.Printf("  zone = %q\n", cfg.Cloud.GCP.Zone)
	fmt.Println()

	fmt.Println("[vm]")
	fmt.Printf("  name = %q\n", cfg.VM.Name)
	fmt.Println()

	fmt.Println("[ssh]")
	if cfg.SSH.Port == 0 {
		fmt.Printf("  port = 22 (default)\n")
	} else {
		fmt.Printf("  port = %d\n", cfg.SSH.Port)
	}
	if cfg.SSH.User == "" {
		fmt.Printf("  user = (current user)\n")
	} else {
		fmt.Printf("  user = %q\n", cfg.SSH.User)
	}
	fmt.Println()

	fmt.Println("[agents]")
	if cfg.Agents.DefaultCommand == "" {
		fmt.Printf("  default_command = (not set)\n")
	} else {
		fmt.Printf("  default_command = %q\n", cfg.Agents.DefaultCommand)
	}
	if len(cfg.Agents.PreCommands) == 0 {
		fmt.Printf("  pre_commands = (none)\n")
	} else {
		fmt.Printf("  pre_commands = %v\n", cfg.Agents.PreCommands)
	}
	if len(cfg.Agents.Repos) > 0 {
		fmt.Println()
		for slug, rc := range cfg.Agents.Repos {
			fmt.Printf("[agents.repos.%s]\n", slug)
			if rc.Command != "" {
				fmt.Printf("  command = %q\n", rc.Command)
			}
			if len(rc.PreCommands) > 0 {
				fmt.Printf("  pre_commands = %v\n", rc.PreCommands)
			}
		}
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// Load existing config or create new one
	cfg, err := config.Load()
	if err != nil {
		// If config doesn't exist, create a new one with defaults
		cfg = &config.Config{
			Cloud: config.CloudConfig{
				Provider: "gcp",
			},
			SSH: config.SSHConfig{
				Port: 22,
			},
		}
	}

	// Set the value
	if err := cfg.SetValue(key, value); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Available keys:")
		for _, k := range config.AllKeys() {
			fmt.Fprintf(os.Stderr, "  %s\n", k)
		}
		return nil
	}

	// Save to default location
	path, err := config.DefaultConfigPath()
	if err != nil {
		return err
	}

	if err := cfg.Save(path); err != nil {
		return err
	}

	fmt.Printf("Set %s = %q\n", key, value)
	fmt.Printf("Saved to %s\n", path)

	return nil
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	path, err := config.DefaultConfigPath()
	if err != nil {
		return err
	}

	// Check if config already exists
	if config.Exists() {
		fmt.Printf("Configuration already exists at %s\n", path)
		fmt.Println()
		fmt.Println("To view current settings:  cloudcoop config get")
		fmt.Println("To modify a setting:       cloudcoop config set <key> <value>")
		fmt.Println()
		fmt.Print("Overwrite existing configuration? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Canceled.")
			return nil
		}
		fmt.Println()
	}

	fmt.Println("cloudcoop setup wizard")
	fmt.Println("======================")
	fmt.Println()
	fmt.Println("This will create a configuration file for cloudcoop.")
	fmt.Println("Press Enter to accept the default value shown in brackets.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Collect GCP project
	fmt.Print("GCP project ID: ")
	project, _ := reader.ReadString('\n')
	project = strings.TrimSpace(project)
	if project == "" {
		fmt.Fprintln(os.Stderr, "Error: GCP project ID is required")
		return nil
	}

	// Collect GCP zone
	fmt.Print("GCP zone [us-central1-a]: ")
	zone, _ := reader.ReadString('\n')
	zone = strings.TrimSpace(zone)
	if zone == "" {
		zone = "us-central1-a"
	}

	// Collect VM name
	fmt.Print("VM instance name [claude-sandbox]: ")
	vmName, _ := reader.ReadString('\n')
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		vmName = "claude-sandbox"
	}

	// Create configuration
	cfg := &config.Config{
		Cloud: config.CloudConfig{
			Provider: "gcp",
			GCP: config.GCPConfig{
				Project: project,
				Zone:    zone,
			},
		},
		VM: config.VMConfig{
			Name: vmName,
		},
		SSH: config.SSHConfig{
			Port: 22,
		},
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Save
	if err := cfg.Save(path); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Configuration saved!")
	fmt.Printf("  File: %s\n", path)
	fmt.Println()
	fmt.Println("Settings:")
	fmt.Printf("  cloud.gcp.project = %q\n", project)
	fmt.Printf("  cloud.gcp.zone    = %q\n", zone)
	fmt.Printf("  vm.name           = %q\n", vmName)
	fmt.Println()
	fmt.Println("You can now run 'cloudcoop' to start the TUI,")
	fmt.Println("or 'cloudcoop status' to check your VM status.")

	return nil
}
