package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/agent"
)

var agentsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new agent session",
	Long: `Add a new agent session in a tmux window on the cloud VM.

Creates a new tmux window in the tmux session. If no session exists,
one will be created automatically.

Examples:
  cloudcoop agents add                          # Add with default command (bash)
  cloudcoop agents add --name=claude-1          # Add with custom name
  cloudcoop agents add --command="claude"       # Add with custom command
  cloudcoop agents add --name=ai --command="claude --dangerously-skip-permissions"`,
	RunE: runAgentsAdd,
}

var (
	addName    string
	addCommand string
)

func init() {
	agentsAddCmd.Flags().StringVar(&addName, "name", "", "window name (auto-generated if not specified)")
	agentsAddCmd.Flags().StringVar(&addCommand, "command", "", "command to run (default: bash)")
}

func runAgentsAdd(cmd *cobra.Command, args []string) error {
	conn, err := connectToVM(cmd)
	if err != nil {
		return err
	}
	if conn == nil {
		return nil
	}
	defer conn.Close()

	// Determine command: flag > config > default
	command := addCommand
	if command == "" && conn.Config.Agents.DefaultCommand != "" {
		command = conn.Config.Agents.DefaultCommand
	}

	// Create agent session
	opts := agent.CreateSessionOptions{
		Name:    addName,
		Command: command,
	}

	session, err := agent.CreateSession(conn.Client, resolveSessionName(), opts)
	if err != nil {
		if err == agent.ErrTmuxNotInstalled {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Install tmux:")
			fmt.Fprintln(os.Stderr, "  sudo apt install tmux    # Debian/Ubuntu")
			fmt.Fprintln(os.Stderr, "  sudo yum install tmux    # RHEL/CentOS")
			return nil
		}
		return fmt.Errorf("create session: %w", err)
	}

	// Report success
	displayName := session.Name
	if displayName == "" {
		displayName = fmt.Sprintf("agent-%d", session.Index)
	}
	displayCmd := session.Command
	if displayCmd == "" {
		displayCmd = "bash"
	}

	fmt.Printf("Created agent session: %s (index %d)\n", displayName, session.Index)
	fmt.Printf("Command: %s\n", displayCmd)
	fmt.Println()
	fmt.Println("Connect with:")
	fmt.Printf("  cloudcoop agents connect %d\n", session.Index)

	return nil
}
