// Package ssh provides SSH client functionality for connecting to VMs.
package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// DefaultTimeout is the default SSH connection timeout.
const DefaultTimeout = 5 * time.Second

// Runner executes commands on a remote host via SSH.
// This interface allows for mocking in tests.
type Runner interface {
	// Run executes a command and returns combined stdout/stderr.
	Run(cmd string) (string, error)
	// Close closes the SSH connection.
	Close() error
}

// Config contains SSH connection configuration.
type Config struct {
	Host    string
	User    string
	Port    int
	Timeout time.Duration
}

// Client wraps an SSH connection and implements Runner.
type Client struct {
	conn *ssh.Client
	host string
	user string
}

// Ensure Client implements Runner.
var _ Runner = (*Client)(nil)

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(host, user string) Config {
	return Config{Host: host, User: user, Port: 22, Timeout: DefaultTimeout}
}

// NewClient creates a new SSH client connection.
func NewClient(cfg Config) (*Client, error) {
	authMethods := discoverAuthMethods()
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no SSH authentication methods available")
	}

	hostKeyCallback := loadKnownHostsOrInsecure()

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.Timeout,
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s: %w", cfg.Host, err)
	}

	return &Client{conn: conn, host: cfg.Host, user: cfg.User}, nil
}

// Run executes a command on the remote host and returns combined output.
func (c *Client) Run(cmd string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	output, err := session.CombinedOutput(cmd)
	return string(output), err
}

// Close closes the SSH connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// discoverAuthMethods finds available SSH authentication methods.
// It checks for SSH agent first (preferred), then falls back to key files.
func discoverAuthMethods() []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	// SSH agent (preferred)
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}

	// Key files fallback
	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		path := filepath.Join(home, ".ssh", name)
		if key, err := os.ReadFile(path); err == nil {
			if signer, err := ssh.ParsePrivateKey(key); err == nil {
				methods = append(methods, ssh.PublicKeys(signer))
			}
		}
	}

	return methods
}

// loadKnownHostsOrInsecure loads known_hosts file or falls back to insecure mode.
func loadKnownHostsOrInsecure() ssh.HostKeyCallback {
	home, _ := os.UserHomeDir()
	if cb, err := knownhosts.New(filepath.Join(home, ".ssh", "known_hosts")); err == nil {
		return cb
	}
	return ssh.InsecureIgnoreHostKey()
}
