package ssh

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// cloudcoopKnownHostsPath returns the path to cloudcoop's managed known_hosts file.
// This is separate from ~/.ssh/known_hosts to avoid polluting the user's file
// with ephemeral VM host keys.
func cloudcoopKnownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "cloudcoop", "known_hosts"), nil
}

// formatHostPort formats host:port for display, handling standard port 22.
func formatHostPort(host string, port int) string {
	if port == 22 || port == 0 {
		return host
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// formatKnownHostsEntry formats a host entry for the known_hosts file.
// For non-standard ports, uses [host]:port format.
func formatKnownHostsEntry(host string, port int) string {
	if port == 22 || port == 0 {
		return host
	}
	return fmt.Sprintf("[%s]:%d", host, port)
}

// EnsureHostKey ensures the host key is in cloudcoop's known_hosts file.
// If the key doesn't exist or has changed, it fetches and stores the new key.
// This provides a seamless experience for ephemeral VMs.
func EnsureHostKey(host string, port int) error {
	khPath, err := cloudcoopKnownHostsPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(khPath), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Remove any existing entry for this host (handles IP reuse, VM recreation)
	_ = removeHostEntry(khPath, host, port)

	// Fetch the current host key using ssh-keyscan
	args := []string{"-t", "ed25519,rsa,ecdsa"}
	if port != 22 && port != 0 {
		args = append(args, "-p", fmt.Sprintf("%d", port))
	}
	args = append(args, host)

	cmd := exec.Command("ssh-keyscan", args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ssh-keyscan failed for %s: %w", formatHostPort(host, port), err)
	}

	if len(output) == 0 {
		return fmt.Errorf("ssh-keyscan returned no keys for %s (VM may not be ready)", formatHostPort(host, port))
	}

	// Append to known_hosts
	f, err := os.OpenFile(khPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open known_hosts: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(output); err != nil {
		return fmt.Errorf("write to known_hosts: %w", err)
	}

	return nil
}

// removeHostEntry removes all entries for a host from the known_hosts file.
func removeHostEntry(khPath, host string, port int) error {
	pattern := formatKnownHostsEntry(host, port)

	// Read existing file
	content, err := os.ReadFile(khPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Filter out matching lines
	var newLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		// Keep line if it doesn't match the host pattern
		if !strings.HasPrefix(line, pattern+" ") && !strings.HasPrefix(line, pattern+",") {
			newLines = append(newLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Write back
	return os.WriteFile(khPath, []byte(strings.Join(newLines, "\n")+"\n"), 0600)
}

// ClearHostKey removes a host's key from cloudcoop's known_hosts file.
// Useful when a VM is deleted.
func ClearHostKey(host string, port int) error {
	khPath, err := cloudcoopKnownHostsPath()
	if err != nil {
		return err
	}
	return removeHostEntry(khPath, host, port)
}

// CreateHostKeyCallback creates an SSH host key callback using cloudcoop's
// managed known_hosts file. This keeps VM host keys separate from the user's
// main known_hosts file.
func CreateHostKeyCallback(host string, port int) (ssh.HostKeyCallback, error) {
	khPath, err := cloudcoopKnownHostsPath()
	if err != nil {
		return nil, err
	}

	// Check if known_hosts file exists
	if _, err := os.Stat(khPath); os.IsNotExist(err) {
		// No known_hosts file yet - create directory and return callback
		// that will fail (caller should use EnsureHostKey first)
		if err := os.MkdirAll(filepath.Dir(khPath), 0700); err != nil {
			return nil, fmt.Errorf("create config directory: %w", err)
		}
		// Create empty file so knownhosts.New doesn't fail
		if err := os.WriteFile(khPath, []byte{}, 0600); err != nil {
			return nil, fmt.Errorf("create known_hosts file: %w", err)
		}
	}

	// Load cloudcoop's known_hosts
	callback, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}

	// Wrap to provide clearer error context
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := callback(hostname, remote, key)
		if err != nil {
			return fmt.Errorf("host key verification failed for %s: %w (try running 'cloudcoop ssh' to refresh)", formatHostPort(host, port), err)
		}
		return nil
	}, nil
}

// IsHostKeyError checks if an error is a host key related error.
func IsHostKeyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "key is unknown") ||
		strings.Contains(errStr, "key mismatch") ||
		strings.Contains(errStr, "host key verification failed")
}
