package setup

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// PrereqStatus describes the result of a prerequisite check.
type PrereqStatus struct {
	Name    string
	OK      bool
	Detail  string // e.g., the path found, or what's missing
	HelpMsg string // guidance on how to fix
}

// CheckSSHKey checks if an SSH key exists in the default locations.
func CheckSSHKey() PrereqStatus {
	return CheckSSHKeyAt("")
}

// CheckSSHKeyAt checks for SSH keys in a specific home directory.
// If homeDir is empty, uses the current user's home directory.
func CheckSSHKeyAt(homeDir string) PrereqStatus {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return PrereqStatus{
				Name:    "SSH key",
				OK:      false,
				Detail:  "cannot determine home directory",
				HelpMsg: "Set the HOME environment variable",
			}
		}
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	keyNames := []string{"id_ed25519", "id_rsa", "id_ecdsa"}

	for _, name := range keyNames {
		pubPath := filepath.Join(sshDir, name+".pub")
		if _, err := os.Stat(pubPath); err == nil {
			return PrereqStatus{
				Name:   "SSH key",
				OK:     true,
				Detail: pubPath,
			}
		}
	}

	return PrereqStatus{
		Name:    "SSH key",
		OK:      false,
		Detail:  "no SSH key found in " + sshDir,
		HelpMsg: "Generate an SSH key with: ssh-keygen -t ed25519",
	}
}

// GenerateSSHKey creates an ed25519 SSH key pair at ~/.ssh/id_ed25519.
// It returns the path to the public key file.
func GenerateSSHKey() (string, error) {
	return GenerateSSHKeyAt("")
}

// GenerateSSHKeyAt creates an ed25519 SSH key pair in the given home directory.
// If homeDir is empty, uses the current user's home directory.
func GenerateSSHKeyAt(homeDir string) (string, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determine home directory: %w", err)
		}
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", fmt.Errorf("create .ssh directory: %w", err)
	}

	privPath := filepath.Join(sshDir, "id_ed25519")
	pubPath := privPath + ".pub"

	// Generate key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generate ed25519 key: %w", err)
	}

	// Marshal private key to OpenSSH format
	privPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return "", fmt.Errorf("marshal private key: %w", err)
	}

	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0o600); err != nil {
		return "", fmt.Errorf("write private key: %w", err)
	}

	// Marshal public key to authorized_keys format
	sshPub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("convert public key: %w", err)
	}

	pubBytes := ssh.MarshalAuthorizedKey(sshPub)
	if err := os.WriteFile(pubPath, pubBytes, 0o600); err != nil {
		return "", fmt.Errorf("write public key: %w", err)
	}

	return pubPath, nil
}
