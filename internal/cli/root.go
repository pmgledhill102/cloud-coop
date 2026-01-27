// Package cli provides the command-line interface for cloudcoop.
package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/cloud-coop/cloudcoop/internal/apperrors"
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
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runTUI,
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
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(provisionCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
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

	log.Debug("launching TUI")

	// Create and run the TUI application
	app := tui.New()
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return apperrors.Wrap(err, "TUI failed")
	}

	return nil
}
