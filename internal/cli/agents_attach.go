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

var agentsAttachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Attach to an agent session",
	Long: `Attach to an agent session running in tmux on the cloud VM.

Use --next to automatically select the next unattached window, or
use --window to attach to a specific window by name or index.

Examples:
  cloudcoop agents attach --next              # auto-select next unattached window
  cloudcoop agents attach --window main       # attach to specific window by name
  cloudcoop agents attach --window 2          # attach to specific window by index`,
	RunE: runAgentsAttach,
}

var (
	attachNext   bool
	attachWindow string
)

func init() {
	agentsAttachCmd.Flags().BoolVar(&attachNext, "next", false, "auto-select next unattached window")
	agentsAttachCmd.Flags().StringVar(&attachWindow, "window", "", "window name or index to attach to")
	agentsAttachCmd.MarkFlagsMutuallyExclusive("next", "window")
}

func runAgentsAttach(cmd *cobra.Command, args []string) error {
	if !attachNext && attachWindow == "" {
		return fmt.Errorf("either --next or --window is required")
	}

	// Check that SSH is available for interactive connection
	if err := ssh.CheckSSHAvailable(); err != nil {
		return err
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

	// List sessions in the tmux session
	listResult, err := agent.ListSessions(conn.Client, sessionName)
	if err != nil {
		conn.Close()
		if errors.Is(err, agent.ErrTmuxNotInstalled) {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			return nil
		}
		return fmt.Errorf("list sessions: %w", err)
	}

	if listResult.NoSession || len(listResult.Sessions) == 0 {
		conn.Close()
		fmt.Fprintln(os.Stderr, "No agent sessions found")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Start an agent session with:")
		fmt.Fprintln(os.Stderr, "  cloudcoop agents add")
		return nil
	}

	var targetWindow agent.Session
	var groupedSessionName string

	if attachNext {
		// Get clients to find unattached window
		clients, err := agent.ListClients(conn.Client, sessionName)
		if err != nil {
			conn.Close()
			return fmt.Errorf("list clients: %w", err)
		}

		targetWindow, err = agent.FindNextWindow(listResult.Sessions, clients)
		if err != nil {
			conn.Close()
			if errors.Is(err, agent.ErrAllWindowsAttached) {
				fmt.Fprintln(os.Stderr, "All windows have attached clients")
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Create a new agent session with:")
				fmt.Fprintln(os.Stderr, "  cloudcoop agents add")
				return nil
			}
			return fmt.Errorf("find next window: %w", err)
		}

		// Create grouped session for this attachment
		groupedSessionName, err = agent.CreateGroupedSession(conn.Client, sessionName, targetWindow.Index)
		if err != nil {
			conn.Close()
			return fmt.Errorf("create grouped session: %w", err)
		}
	} else {
		// Find window by name or index
		targetWindow, err = findWindow(listResult.Sessions, attachWindow)
		if err != nil {
			conn.Close()
			return err
		}

		// Create grouped session for specific window too
		groupedSessionName, err = agent.CreateGroupedSession(conn.Client, sessionName, targetWindow.Index)
		if err != nil {
			conn.Close()
			return fmt.Errorf("create grouped session: %w", err)
		}
	}

	// Close Go SSH client before interactive attach
	conn.Close()

	fmt.Printf("Attaching to window %d (%s)...\n", targetWindow.Index, targetWindow.Name)

	// Connect interactively via native SSH
	connectOpts := ssh.ConnectOptions{
		Host:           conn.IP,
		User:           conn.User,
		Port:           conn.Port,
		Session:        sessionName,
		WindowIndex:    targetWindow.Index,
		GroupedSession: groupedSessionName,
		VM:             ssh.NewVMIdentity(conn.VMInfo.Name, conn.VMInfo.CloudcoopCreated),
	}

	return ssh.ConnectInteractive(connectOpts)
}

// findWindow looks up a window by name or index string.
func findWindow(sessions []agent.Session, nameOrIndex string) (agent.Session, error) {
	// Try as index first
	if idx, err := strconv.Atoi(nameOrIndex); err == nil {
		for _, s := range sessions {
			if s.Index == idx {
				return s, nil
			}
		}
		return agent.Session{}, fmt.Errorf("no window at index %d", idx)
	}

	// Try as name
	for _, s := range sessions {
		if s.Name == nameOrIndex {
			return s, nil
		}
	}
	return agent.Session{}, fmt.Errorf("no window named %q", nameOrIndex)
}
