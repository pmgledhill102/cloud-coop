package ssh

import (
	"fmt"
	"os"
	"os/exec"
)

// ConnectOptions contains options for interactive SSH connection.
type ConnectOptions struct {
	Host           string // VM host (IP or hostname)
	User           string // SSH username
	Port           int    // SSH port (default: 22)
	Session        string // tmux session name
	WindowIndex    int    // tmux window index to attach to
	GroupedSession string // if set, attach to this grouped session instead
}

// ConnectInteractive shells out to SSH for an interactive terminal session.
// It attaches to the specified tmux window in the named session.
// This function blocks until the SSH session ends.
func ConnectInteractive(opts ConnectOptions) error {
	port := opts.Port
	if port == 0 {
		port = 22
	}

	// Ensure host key is available (uses cloudcoop's managed known_hosts)
	if err := EnsureHostKey(opts.Host, port); err != nil {
		return fmt.Errorf("fetch host key: %w", err)
	}

	// Get path to cloudcoop's known_hosts file for native ssh
	knownHostsPath, err := CloudcoopKnownHostsPath()
	if err != nil {
		return fmt.Errorf("get known_hosts path: %w", err)
	}

	// Build the tmux attach command
	var tmuxCmd string
	if opts.GroupedSession != "" {
		tmuxCmd = fmt.Sprintf("tmux attach -t %s", opts.GroupedSession)
	} else {
		// First select the window, then attach to the session
		tmuxCmd = fmt.Sprintf("tmux select-window -t %s:%d && tmux attach -t %s", opts.Session, opts.WindowIndex, opts.Session)
	}

	// Build SSH command
	// -t forces pseudo-terminal allocation (required for tmux)
	// -p specifies port
	// -o UserKnownHostsFile uses cloudcoop's managed known_hosts
	args := []string{
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-t", // Force PTY allocation
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("%s@%s", opts.User, opts.Host),
		tmuxCmd,
	}

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command - this blocks until SSH session ends
	err = cmd.Run()
	if err != nil {
		// Check if it's just an exit status from the remote command
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 often means tmux session/window not found
			if exitErr.ExitCode() == 1 {
				return fmt.Errorf("failed to attach to tmux window %d (session may not exist)", opts.WindowIndex)
			}
		}
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	return nil
}

// CheckSSHAvailable verifies that the ssh command is available.
func CheckSSHAvailable() error {
	_, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh command not found - please install OpenSSH client")
	}
	return nil
}
