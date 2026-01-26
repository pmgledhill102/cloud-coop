package cli

import (
	"github.com/spf13/cobra"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage agent sessions",
	Long: `Manage agent sessions running in tmux on the cloud VM.

Agents run in tmux windows within an "agents" session on the VM.
This command group provides tools to list, monitor, and manage these sessions.`,
}

func init() {
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsAddCmd)
	agentsCmd.AddCommand(agentsKillCmd)
}
