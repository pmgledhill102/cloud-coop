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
// It automatically manages host keys in cloudcoop's own known_hosts file,
// so users don't need to manually handle host key verification for VMs.
func NewClient(cfg Config) (*Client, error) {
	authMethods := discoverAuthMethods()
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no SSH authentication methods available")
	}

	// Ensure we have the host key before connecting
	// This fetches/updates the key automatically for cloudcoop-managed VMs
	if err := EnsureHostKey(cfg.Host, cfg.Port); err != nil {
		return nil, fmt.Errorf("fetch host key: %w", err)
	}

	hostKeyCallback, err := CreateHostKeyCallback(cfg.Host, cfg.Port)
	if err != nil {
		return nil, fmt.Errorf("create host key callback: %w", err)
	}

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

	// SSH agent (preferred, but only if it has keys — an empty agent
	// consumes the server's publickey attempt and prevents file-based
	// keys from being tried).
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			agentClient := agent.NewClient(conn)
			if signers, err := agentClient.Signers(); err == nil && len(signers) > 0 {
				methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
			}
		}
	}

	// Key files fallback
	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa", "google_compute_engine"} {
		path := filepath.Join(home, ".ssh", name)
		if key, err := os.ReadFile(path); err == nil {
			if signer, err := ssh.ParsePrivateKey(key); err == nil {
				methods = append(methods, ssh.PublicKeys(upgradeRSASigner(signer)))
			}
		}
	}

	return methods
}

// upgradeRSASigner upgrades an RSA signer to prefer rsa-sha2-512 and
// rsa-sha2-256 over the legacy ssh-rsa (SHA-1) algorithm. Modern OpenSSH
// servers disable ssh-rsa by default, so without this upgrade RSA keys
// (including GCP's google_compute_engine) are rejected.
// Non-RSA signers are returned unchanged.
func upgradeRSASigner(signer ssh.Signer) ssh.Signer {
	if signer.PublicKey().Type() != ssh.KeyAlgoRSA {
		return signer
	}
	algSigner, ok := signer.(ssh.AlgorithmSigner)
	if !ok {
		return signer
	}
	upgraded, err := ssh.NewSignerWithAlgorithms(algSigner, []string{
		ssh.KeyAlgoRSASHA512,
		ssh.KeyAlgoRSASHA256,
	})
	if err != nil {
		return signer
	}
	return upgraded
}
