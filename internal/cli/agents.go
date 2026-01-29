package cli

import (
	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/workspace"
)

// defaultSessionName is the fallback tmux session name when workspace
// detection is unavailable (e.g., not in a git repo).
const defaultSessionName = "agents"

// resolveSessionName returns the tmux session name for the current repo.
// If run from a git repo with an origin remote, uses the repo slug.
// Otherwise falls back to the default "agents" session.
func resolveSessionName() string {
	info, err := workspace.Detect(workspace.NewGitRunner("."))
	if err != nil {
		return defaultSessionName
	}
	return info.Slug
}

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage agent sessions",
	Long: `Manage agent sessions running in tmux on the cloud VM.

Agents run in tmux windows within a tmux session on the VM.
This command group provides tools to list, monitor, and manage these sessions.`,
}

func init() {
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsAddCmd)
	agentsCmd.AddCommand(agentsKillCmd)
	agentsCmd.AddCommand(agentsAttachCmd)
	agentsCmd.AddCommand(agentsSyncCmd)
}
