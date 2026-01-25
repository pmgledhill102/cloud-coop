package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of VMs and agents",
	Long: `Display the current status of cloud VMs and running agents.

This shows:
- Active VMs and their state (running, stopped, etc.)
- Running agents and their sessions
- Resource usage and costs`,
	Run: func(cmd *cobra.Command, args []string) {
		// Mock output for iteration 1
		fmt.Println("cloudcoop status")
		fmt.Println()
		fmt.Println("VMs:")
		fmt.Println("  (no VMs configured)")
		fmt.Println()
		fmt.Println("Agents:")
		fmt.Println("  (no agents running)")
		fmt.Println()
		fmt.Println("Use 'cloudcoop' to launch the TUI and configure VMs.")
	},
}
