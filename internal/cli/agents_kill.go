package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/agent"
)

var agentsKillCmd = &cobra.Command{
	Use:   "kill <index>",
	Short: "Kill an agent session",
	Long: `Kill an agent session by its tmux window index.

The window index can be found using 'cloudcoop agents list'.

By default, this command will refuse to kill a window that has an active
process running (i.e., not just a shell). Use --force to override this
protection.

Examples:
  cloudcoop agents kill 0           # Kill agent at index 0
  cloudcoop agents kill 2 --force   # Force kill even if process is running`,
	Args: cobra.ExactArgs(1),
	RunE: runAgentsKill,
}

var killForce bool

func init() {
	agentsKillCmd.Flags().BoolVarP(&killForce, "force", "f", false, "force kill even if window has active process")
}

func runAgentsKill(cmd *cobra.Command, args []string) error {
	// Parse window index
	index, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid window index: %s", args[0])
	}

	conn, err := connectToVM(cmd)
	if err != nil {
		return err
	}
	if conn == nil {
		return nil
	}
	defer conn.Close()

	// Kill agent session
	opts := agent.KillSessionOptions{
		Index: index,
		Force: killForce,
	}

	err = agent.KillSession(conn.Client, resolveSessionName(), opts)
	if err != nil {
		if errors.Is(err, agent.ErrTmuxNotInstalled) {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			return nil
		}
		if errors.Is(err, agent.ErrNoSession) {
			fmt.Fprintln(os.Stderr, "No tmux session exists")
			return nil
		}
		if errors.Is(err, agent.ErrWindowNotFound) {
			fmt.Fprintf(os.Stderr, "No agent found at index %d\n", index)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "List agents with:")
			fmt.Fprintln(os.Stderr, "  cloudcoop agents list")
			return nil
		}
		if errors.Is(err, agent.ErrActiveProcess) {
			fmt.Fprintf(os.Stderr, "Agent at index %d has an active process running\n", index)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Use --force to kill anyway:")
			fmt.Fprintf(os.Stderr, "  cloudcoop agents kill %d --force\n", index)
			return nil
		}
		return fmt.Errorf("kill session: %w", err)
	}

	fmt.Printf("Killed agent at index %d\n", index)
	return nil
}
