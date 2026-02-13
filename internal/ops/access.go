package ops

import (
	"context"
	"fmt"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/network"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// PublicIPDetector detects the workstation's public IP. Replaceable for testing.
var PublicIPDetector = network.DetectPublicIP

// SSHPublicKeyReader reads the user's SSH public key. Replaceable for testing.
var SSHPublicKeyReader = ssh.ReadPublicKey

// EnsureFirewall detects the workstation's public IP and ensures the cloud
// firewall allows SSH from it. Returns whether the rule was changed.
func EnsureFirewall(ctx context.Context, cfg *config.Config, provider cloud.Provider) (bool, error) {
	ip, err := PublicIPDetector(ctx)
	if err != nil {
		return false, fmt.Errorf("detect public IP: %w", err)
	}

	sshPort := ssh.ResolvePort(cfg.SSH.Port)
	netName := cfg.VM.Network
	if netName == "" {
		netName = "default"
	}

	return provider.EnsureFirewallAllowsSSH(ctx, cloud.FirewallConfig{
		SourceIP: ip,
		Port:     sshPort,
		Network:  netName,
	})
}

// EnsureSSHKey reads the user's SSH public key and ensures it is present
// in the VM's metadata.
func EnsureSSHKey(ctx context.Context, cfg *config.Config, provider cloud.Provider, vmName string) error {
	pubKey, err := SSHPublicKeyReader()
	if err != nil {
		return fmt.Errorf("read SSH public key: %w", err)
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	if sshUser == "" {
		return fmt.Errorf("could not resolve SSH user")
	}

	return provider.EnsureSSHKeyOnVM(ctx, vmName, sshUser, pubKey)
}

// EnsureVMAccess ensures both firewall and SSH key access to a VM.
// Individual errors are logged as debug messages but not returned (non-fatal).
func EnsureVMAccess(ctx context.Context, cfg *config.Config, provider cloud.Provider, vmName string) {
	changed, err := EnsureFirewall(ctx, cfg, provider)
	if err != nil {
		log.Debug("firewall check failed (non-fatal)", "error", err)
	} else if changed {
		log.Info("firewall updated")
	}

	if err := EnsureSSHKey(ctx, cfg, provider, vmName); err != nil {
		log.Debug("SSH key check failed (non-fatal)", "error", err)
	}
}
