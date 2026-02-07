package setup

import (
	"os"
	"path/filepath"
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
