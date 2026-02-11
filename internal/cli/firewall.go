package cli

import (
	"context"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/network"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// publicIPDetector detects the workstation's public IP. Injectable for testing.
var publicIPDetector func(ctx context.Context) (string, error) = network.DetectPublicIP

// ensureFirewallAccess detects the workstation's public IP and ensures the
// cloud firewall allows SSH from it. Errors are non-fatal (logged as warnings).
func ensureFirewallAccess(ctx context.Context, cfg *config.Config, provider cloud.Provider) {
	ip, err := publicIPDetector(ctx)
	if err != nil {
		log.Debug("detect public IP failed (non-fatal)", "error", err)
		return
	}

	sshPort := ssh.ResolvePort(cfg.SSH.Port)
	network := cfg.VM.Network
	if network == "" {
		network = "default"
	}

	changed, err := provider.EnsureFirewallAllowsSSH(ctx, cloud.FirewallConfig{
		SourceIP: ip,
		Port:     sshPort,
		Network:  network,
	})
	if err != nil {
		log.Debug("firewall check failed (non-fatal)", "error", err)
		return
	}

	if changed {
		log.Info("firewall updated", "ip", ip, "port", sshPort)
	}
}
