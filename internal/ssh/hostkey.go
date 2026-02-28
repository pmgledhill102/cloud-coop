package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// pinnedKey is a single entry in the pin store.
type pinnedKey struct {
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
	Created string `toml:"created"`
}

// pinnedKeysStore maps VM name to its pinned key metadata.
type pinnedKeysStore map[string]pinnedKey

// CloudcoopKnownHostsPath returns the path to cloudcoop's managed known_hosts file.
// This is separate from ~/.ssh/known_hosts to avoid polluting the user's file
// with ephemeral VM host keys.
func CloudcoopKnownHostsPath() (string, error) {
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

// keyscanMaxRetries is the number of times to retry ssh-keyscan before giving up.
const keyscanMaxRetries = 3

// keyscanRetryDelay is the delay between ssh-keyscan retries.
const keyscanRetryDelay = 5 * time.Second

// EnsureHostKey ensures the host key is in cloudcoop's known_hosts file.
// If the key doesn't exist or has changed, it fetches and stores the new key.
// This provides a seamless experience for ephemeral VMs.
// Retries up to keyscanMaxRetries times to handle VMs that are still booting.
func EnsureHostKey(host string, port int) error {
	khPath, err := CloudcoopKnownHostsPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(khPath), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Remove any existing entry for this host (handles IP reuse, VM recreation)
	_ = removeHostEntry(khPath, host, port)

	// Fetch the current host key using ssh-keyscan, with retries
	output, err := runKeyscanWithRetry(host, port)
	if err != nil {
		return err
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

// runKeyscanWithRetry runs ssh-keyscan with retries, capturing stderr for diagnostics.
func runKeyscanWithRetry(host string, port int) ([]byte, error) {
	args := []string{"-T", "5", "-t", "ed25519,rsa,ecdsa"}
	if port != 22 && port != 0 {
		args = append(args, "-p", fmt.Sprintf("%d", port))
	}
	args = append(args, host)

	hostPort := formatHostPort(host, port)
	var lastErr error

	for attempt := range keyscanMaxRetries {
		if attempt > 0 {
			time.Sleep(keyscanRetryDelay)
		}

		cmd := exec.Command("ssh-keyscan", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()

		if err != nil {
			// OpenSSH 9.4+ exits non-zero when no keys are found.
			// Check if we got valid output despite the error exit code.
			if len(bytes.TrimSpace(output)) > 0 {
				return output, nil
			}

			errLines := filterKeyscanErrors(stderr.String())
			if errLines != "" {
				lastErr = fmt.Errorf("ssh-keyscan failed for %s: %s", hostPort, errLines)
			} else {
				lastErr = fmt.Errorf("ssh-keyscan returned no keys for %s (SSH may not be listening on port %d)", hostPort, port)
			}
			continue
		}

		if len(output) == 0 {
			lastErr = fmt.Errorf("ssh-keyscan returned no keys for %s (VM may not be ready)", hostPort)
			continue
		}

		return output, nil
	}

	return nil, lastErr
}

// filterKeyscanErrors extracts non-comment error lines from ssh-keyscan stderr.
// ssh-keyscan writes informational "# host SSH-2.0-..." lines to stderr;
// actual errors (connection refused, etc.) are non-comment lines.
func filterKeyscanErrors(stderr string) string {
	var errLines []string
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			errLines = append(errLines, line)
		}
	}
	return strings.Join(errLines, "; ")
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
	return os.WriteFile(khPath, []byte(strings.Join(newLines, "\n")+"\n"), 0600) //nolint:gosec // G703: path is from CloudcoopKnownHostsPath(), not user input
}

// ClearHostKey removes a host's key from cloudcoop's known_hosts file.
// Useful when a VM is deleted.
func ClearHostKey(host string, port int) error {
	khPath, err := CloudcoopKnownHostsPath()
	if err != nil {
		return err
	}
	return removeHostEntry(khPath, host, port)
}

// CreateHostKeyCallback creates an SSH host key callback using cloudcoop's
// managed known_hosts file. This keeps VM host keys separate from the user's
// main known_hosts file.
func CreateHostKeyCallback(host string, port int) (ssh.HostKeyCallback, error) {
	khPath, err := CloudcoopKnownHostsPath()
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

// EnsureHostKeyPinned is the pin-aware variant of EnsureHostKey.
// When vm is non-nil it uses a TOML pin file to avoid unnecessary re-scans:
//   - Pin matches (same created, host, port) and known_hosts entry exists: skip
//   - Pin stale (different created or IP): remove old entry, re-scan, update pin
//   - No pin (first connection): scan, create pin
//
// When vm is nil it falls back to the unpinned EnsureHostKey behaviour.
func EnsureHostKeyPinned(host string, port int, vm *VMIdentity) error {
	if vm == nil {
		return EnsureHostKey(host, port)
	}

	store, err := loadPinnedKeys()
	if err != nil {
		return err
	}

	if pin, ok := store[vm.Name]; ok && pin.Created == vm.Created &&
		pin.Host == host && pin.Port == port {
		// Pin matches — check known_hosts still has the entry.
		if hostKeyExists(host, port) {
			return nil // nothing to do
		}
		// Entry disappeared; fall through to re-scan.
	}

	// Remove any stale known_hosts entry for the previous host (if any).
	if old, ok := store[vm.Name]; ok {
		khPath, err := CloudcoopKnownHostsPath()
		if err == nil {
			_ = removeHostEntry(khPath, old.Host, old.Port)
		}
	}

	// Fetch (or refresh) the host key.
	if err := EnsureHostKey(host, port); err != nil {
		return err
	}

	// Update the pin.
	store[vm.Name] = pinnedKey{Host: host, Port: port, Created: vm.Created}
	return savePinnedKeys(store)
}

// ClearPinnedKey removes the pin and the known_hosts entry for vmName.
func ClearPinnedKey(vmName string) error {
	store, err := loadPinnedKeys()
	if err != nil {
		return err
	}

	pin, ok := store[vmName]
	if !ok {
		return nil
	}

	// Remove from known_hosts.
	khPath, khErr := CloudcoopKnownHostsPath()
	if khErr == nil {
		_ = removeHostEntry(khPath, pin.Host, pin.Port)
	}

	delete(store, vmName)
	return savePinnedKeys(store)
}

// pinnedKeysPath returns the path to the TOML pin file.
func pinnedKeysPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "cloudcoop", "pinned_keys.toml"), nil
}

// loadPinnedKeys reads the pin store from disk.
// Returns an empty store on missing or corrupt file.
func loadPinnedKeys() (pinnedKeysStore, error) {
	path, err := pinnedKeysPath()
	if err != nil {
		return nil, err
	}

	store := make(pinnedKeysStore)
	if _, err := toml.DecodeFile(path, &store); err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		// Corrupt file — start fresh.
		return make(pinnedKeysStore), nil
	}
	return store, nil
}

// savePinnedKeys writes the pin store to disk atomically.
func savePinnedKeys(store pinnedKeysStore) error {
	path, err := pinnedKeysPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(store); err != nil {
		return fmt.Errorf("encode pinned keys: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0600)
}

// hostKeyExists returns true if cloudcoop's known_hosts file contains at least
// one entry for the given host and port.
func hostKeyExists(host string, port int) bool {
	khPath, err := CloudcoopKnownHostsPath()
	if err != nil {
		return false
	}

	content, err := os.ReadFile(khPath)
	if err != nil {
		return false
	}

	pattern := formatKnownHostsEntry(host, port)
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, pattern+" ") || strings.HasPrefix(line, pattern+",") {
			return true
		}
	}
	return false
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
