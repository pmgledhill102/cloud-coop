// Package cli provides the command-line interface for cloudcoop.
package cli

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/cloud-coop/cloudcoop/internal/apperrors"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/tui"
)

// configResult holds config loaded by PersistentPreRunE.
type configResult struct {
	cfg *config.Config
	err error
}

// contextKey is used for storing values in command context.
type contextKey string

const configCtxKey contextKey = "config"

var rootCmd = &cobra.Command{
	Use:   "cloudcoop",
	Short: "Manage sandboxed AI coding agents on cloud VMs",
	Long: `cloudcoop is a TUI/CLI for provisioning and managing cloud VMs
that host AI coding agents in isolated tmux sessions.

Run without arguments to launch the interactive TUI.
Use subcommands for scriptable automation.`,
	SilenceUsage:               true,
	SilenceErrors:              true,
	SuggestionsMinimumDistance: 4, // Suggest commands within 4 edit distance
	PersistentPreRunE:          persistentPreRunE,
	RunE:                       runTUI,
}

// persistentPreRunE runs before every command to handle shared setup.
func persistentPreRunE(cmd *cobra.Command, _ []string) error {
	// Wire up --verbose flag to enable debug logging.
	if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
		log.SetVerbose()
	}

	// Skip config loading for commands that don't need it.
	if cmd.Annotations != nil && cmd.Annotations["skip-config"] == "true" {
		return nil
	}

	// Load config and store in context (commands handle errors themselves).
	cfg, err := configLoader()
	ctx := context.WithValue(cmd.Context(), configCtxKey, &configResult{cfg: cfg, err: err})
	cmd.SetContext(ctx)
	return nil
}

// configFromCmd retrieves config loaded by PersistentPreRunE.
// Falls back to configLoader for test compatibility when PersistentPreRunE
// hasn't run (e.g., tests calling RunE directly).
func configFromCmd(cmd *cobra.Command) (*config.Config, error) {
	if result, ok := cmd.Context().Value(configCtxKey).(*configResult); ok {
		return result.cfg, result.err
	}
	return configLoader()
}

func init() {
	// Add persistent flags that apply to all commands
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose output")

	// Define command groups for organised help output
	rootCmd.AddGroup(
		&cobra.Group{ID: "vm", Title: "VM Lifecycle:"},
		&cobra.Group{ID: "agents", Title: "Agent Management:"},
		&cobra.Group{ID: "config", Title: "Configuration & Setup:"},
	)

	// Assign groups
	statusCmd.GroupID = "vm"
	startCmd.GroupID = "vm"
	stopCmd.GroupID = "vm"
	createCmd.GroupID = "vm"
	deleteCmd.GroupID = "vm"

	agentsCmd.GroupID = "agents"
	connectCmd.GroupID = "agents"

	configCmd.GroupID = "config"
	setupCmd.GroupID = "config"

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(sshCmd)
	rootCmd.AddCommand(agentsCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(provisionCmd)
	rootCmd.AddCommand(terminalCmd)
	rootCmd.AddCommand(completionCmd)
}

// Execute runs the root command.
func Execute() error {
	err := rootCmd.Execute()
	if err == nil {
		return nil
	}

	return err
}

// printSetupRequired prints a helpful message when setup hasn't been completed.
func printSetupRequired(heading string, err error) error {
	globalPath, _ := config.DefaultConfigPath()
	projectPath := config.ProjectConfigPath(".")
	instancePath := config.InstanceConfigPath(".")

	fmt.Fprintf(os.Stderr, "%s: %v\n", heading, err)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Run the setup wizard to configure cloudcoop:")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  cloudcoop setup")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Or edit the configuration files directly:")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  Global:   %s\n", globalPath)
	fmt.Fprintf(os.Stderr, "  Project:  %s\n", projectPath)
	fmt.Fprintf(os.Stderr, "  Instance: %s\n", instancePath)
	fmt.Fprintln(os.Stderr)
	return nil
}

// runTUI launches the interactive terminal UI.
func runTUI(cmd *cobra.Command, args []string) error {
	// Check if we're running in an interactive terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "The TUI requires an interactive terminal.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Use 'cloudcoop status' for non-interactive output,")
		fmt.Fprintln(os.Stderr, "or run cloudcoop in a terminal emulator.")
		return nil
	}

	// Check that setup has been completed before launching the TUI
	cfg, cfgErr := configFromCmd(cmd)
	if cfgErr != nil {
		return printSetupRequired("Configuration not found", cfgErr)
	}
	if validErr := cfg.Validate(); validErr != nil {
		return printSetupRequired("Configuration incomplete", validErr)
	}

	log.Debug("launching TUI")

	// Create and run the TUI application
	app := tui.New()
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return apperrors.Wrap(err, "TUI failed")
	}

	return nil
}
