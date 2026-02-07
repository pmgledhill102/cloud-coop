// Package cli provides the command-line interface for cloudcoop.
package cli

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/cloud-coop/cloudcoop/internal/apperrors"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "cloudcoop",
	Short: "Manage sandboxed AI coding agents on cloud VMs",
	Long: `cloudcoop is a TUI/CLI for provisioning and managing cloud VMs
that host AI coding agents in isolated tmux sessions.

Run without arguments to launch the interactive TUI.
Use subcommands for scriptable automation.`,
	SilenceUsage:               true,
	SilenceErrors:              false, // Let Cobra print errors with suggestions
	SuggestionsMinimumDistance: 4,     // Suggest commands within 4 edit distance
	RunE:                       runTUI,
}

func init() {
	// Add persistent flags that apply to all commands
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(sshCmd)
	rootCmd.AddCommand(agentsCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(provisionCmd)
	rootCmd.AddCommand(terminalCmd)
}

// Execute runs the root command.
func Execute() error {
	err := rootCmd.Execute()
	if err == nil {
		return nil
	}

	// Wrap CLI usage errors so main.go doesn't log them
	// (Cobra already printed the error with suggestions via SilenceErrors: false)
	errStr := err.Error()
	if strings.Contains(errStr, "unknown command") ||
		strings.Contains(errStr, "unknown flag") ||
		strings.Contains(errStr, "unknown shorthand flag") {
		return apperrors.NewUsageError(errStr, nil, err)
	}

	return err
}

// printSetupRequired prints a helpful message when setup hasn't been completed.
func printSetupRequired(heading string, err error) error {
	globalPath, _ := config.DefaultConfigPath()
	projectPath := config.ProjectConfigPath(".")

	fmt.Fprintf(os.Stderr, "%s: %v\n", heading, err)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Run the setup wizard to configure cloudcoop:")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  cloudcoop setup")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Or edit the configuration files directly:")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  Global:  %s\n", globalPath)
	fmt.Fprintf(os.Stderr, "  Project: %s\n", projectPath)
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
	cfg, cfgErr := configLoader()
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
