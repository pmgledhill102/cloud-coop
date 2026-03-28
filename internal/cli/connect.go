package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

var connectCmd = &cobra.Command{
	Use:   "connect <index>",
	Short: "Connect to an agent session",
	Long: `Connect to an agent session by attaching to its tmux window.

The window index can be found using 'cloudcoop agents list'.

This command shells out to SSH and attaches to the tmux window,
giving you an interactive terminal session with the agent.

To detach from the session (return to cloudcoop) press: Ctrl+B D
To exit the agent completely, type: exit

Examples:
  cloudcoop connect 0           # Connect to agent at index 0
  cloudcoop connect 2           # Connect to agent at index 2`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("specify an agent index, e.g.: cloudcoop connect 0")
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	RunE: runConnect,
}

func runConnect(cmd *cobra.Command, args []string) error {
	// Check SSH is available
	if err := ssh.CheckSSHAvailable(); err != nil {
		return err
	}

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

	// Resolve session name once for all tmux operations
	sessionName := resolveSessionName()

	// Check agent exists
	result, err := agent.ListSessions(conn.Client, sessionName)
	conn.Close() // Close before interactive session

	if err != nil {
		if errors.Is(err, agent.ErrTmuxNotInstalled) {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			return nil
		}
		return fmt.Errorf("list sessions: %w", err)
	}

	if result.NoSession {
		fmt.Fprintln(os.Stderr, "No tmux session exists")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Create an agent first with:")
		fmt.Fprintln(os.Stderr, "  cloudcoop agents add")
		return nil
	}

	// Find the agent with the given index
	var found bool
	for _, s := range result.Sessions {
		if s.Index == index {
			found = true
			break
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "No agent found at index %d\n", index)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Available agents:")
		for _, s := range result.Sessions {
			fmt.Fprintf(os.Stderr, "  %d: %s (%s)\n", s.Index, s.Name, s.Command)
		}
		return nil
	}

	// Connect interactively
	fmt.Printf("Connecting to agent at index %d...\n", index)
	fmt.Println("(Detach with Ctrl+B D, exit with 'exit')")
	fmt.Println()

	opts := ssh.ConnectOptions{
		Host:        conn.IP,
		User:        conn.User,
		Port:        conn.Port,
		Session:     sessionName,
		WindowIndex: index,
		VM:          ssh.NewVMIdentity(conn.VMInfo.Name, conn.VMInfo.CloudcoopCreated),
	}

	err = ssh.ConnectInteractive(opts)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Disconnected from agent session.")
	return nil
}
