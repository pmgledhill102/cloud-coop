//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

// TestPhase1_Firewall tests firewall rule creation and updates against GCP.
func TestPhase1_Firewall(t *testing.T) {
	if env.provider == nil {
		t.Fatal("provider not initialized — Phase 0 must pass first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	t.Run("create_rule", func(t *testing.T) {
		changed, err := env.provider.EnsureFirewallAllowsSSH(ctx, cloud.FirewallConfig{
			SourceIP: "203.0.113.50", // TEST-NET-3: reserved for documentation
			Port:     env.cfg.SSH.Port,
			Network:  env.cfg.VM.Network,
		})
		if err != nil {
			t.Fatalf("EnsureFirewallAllowsSSH (create): %v", err)
		}
		t.Logf("Firewall rule created/updated: changed=%v", changed)
	})

	t.Run("idempotent_check", func(t *testing.T) {
		// Same call again should be a no-op
		changed, err := env.provider.EnsureFirewallAllowsSSH(ctx, cloud.FirewallConfig{
			SourceIP: "203.0.113.50",
			Port:     env.cfg.SSH.Port,
			Network:  env.cfg.VM.Network,
		})
		if err != nil {
			t.Fatalf("EnsureFirewallAllowsSSH (idempotent): %v", err)
		}
		if changed {
			t.Error("Expected no change on second call with same IP, got changed=true")
		}
	})

	t.Run("update_ip", func(t *testing.T) {
		// Update to a different IP
		changed, err := env.provider.EnsureFirewallAllowsSSH(ctx, cloud.FirewallConfig{
			SourceIP: "198.51.100.1", // TEST-NET-2: reserved for documentation
			Port:     env.cfg.SSH.Port,
			Network:  env.cfg.VM.Network,
		})
		if err != nil {
			t.Fatalf("EnsureFirewallAllowsSSH (update): %v", err)
		}
		if !changed {
			t.Error("Expected changed=true when IP differs")
		}
	})
}
