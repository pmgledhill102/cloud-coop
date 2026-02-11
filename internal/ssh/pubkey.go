package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadPublicKey reads the user's SSH public key from standard locations.
// It checks ~/.ssh/id_ed25519.pub, ~/.ssh/id_rsa.pub, ~/.ssh/id_ecdsa.pub
// in order and returns the contents of the first one found.
func ReadPublicKey() (string, error) {
	return ReadPublicKeyFrom("")
}

// ReadPublicKeyFrom reads the user's SSH public key from a specific home directory.
// If homeDir is empty, uses the current user's home directory.
func ReadPublicKeyFrom(homeDir string) (string, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determine home directory: %w", err)
		}
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	keyNames := []string{"id_ed25519.pub", "id_rsa.pub", "id_ecdsa.pub"}

	for _, name := range keyNames {
		path := filepath.Join(sshDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content != "" {
			return content, nil
		}
	}

	return "", fmt.Errorf("no SSH public key found in %s (looked for id_ed25519.pub, id_rsa.pub, id_ecdsa.pub)", sshDir)
}
