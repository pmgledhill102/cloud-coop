package workspace

import (
	"fmt"

	"github.com/cloud-coop/cloudcoop/internal/shell"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// GitIdentity holds the user's git identity.
type GitIdentity struct {
	Name  string
	Email string
}

// LocalGitIdentity reads user.name and user.email from local git config.
// Returns the identity and true if both are set, or a zero value and false if either is missing.
func LocalGitIdentity(runner GitRunner) (GitIdentity, bool) {
	name, err := runner.Run("config", "user.name")
	if err != nil || name == "" {
		return GitIdentity{}, false
	}
	email, err := runner.Run("config", "user.email")
	if err != nil || email == "" {
		return GitIdentity{}, false
	}
	return GitIdentity{Name: name, Email: email}, true
}

// SetupVMGitIdentity sets git user.name and user.email globally on the VM.
func SetupVMGitIdentity(runner ssh.Runner, id GitIdentity) error {
	cmd := fmt.Sprintf(
		"git config --global user.name %s && git config --global user.email %s",
		shell.Escape(id.Name), shell.Escape(id.Email),
	)
	_, err := runner.Run(cmd)
	if err != nil {
		return fmt.Errorf("set git identity on VM: %w", err)
	}
	return nil
}
