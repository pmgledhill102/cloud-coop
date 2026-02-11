package cli

import (
	"context"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// ensureSSHKeyAccess reads the user's SSH public key and ensures it is
// present in the VM's metadata. Errors are non-fatal (logged as debug).
func ensureSSHKeyAccess(ctx context.Context, cfg *config.Config, provider cloud.Provider, vmName string) {
	pubKey, err := ssh.ReadPublicKey()
	if err != nil {
		log.Debug("read SSH public key failed (non-fatal)", "error", err)
		return
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	if sshUser == "" {
		log.Debug("could not resolve SSH user (non-fatal)")
		return
	}

	if err := provider.EnsureSSHKeyOnVM(ctx, vmName, sshUser, pubKey); err != nil {
		log.Debug("ensure SSH key on VM failed (non-fatal)", "error", err)
	}
}
