package ops

import (
	"context"
	"errors"
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
)

func TestEnsureFirewall_Success(t *testing.T) {
	origDetector := PublicIPDetector
	defer func() { PublicIPDetector = origDetector }()
	PublicIPDetector = func(ctx context.Context) (string, error) {
		return "203.0.113.50", nil
	}

	provider := cloud.NewMockProvider()
	provider.EnsureFirewallChanged = true

	cfg := &config.Config{
		SSH: config.SSHConfig{Port: 2222},
		VM:  config.VMConfig{Network: "my-vpc"},
	}

	changed, err := EnsureFirewall(context.Background(), cfg, provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed=true")
	}

	calls := provider.GetCalls()
	found := false
	for _, c := range calls {
		if c.Method == "EnsureFirewallAllowsSSH" {
			found = true
			fwCfg := c.Args[0].(cloud.FirewallConfig)
			if fwCfg.SourceIP != "203.0.113.50" {
				t.Errorf("SourceIP = %q, want %q", fwCfg.SourceIP, "203.0.113.50")
			}
			if fwCfg.Port != 2222 {
				t.Errorf("Port = %d, want %d", fwCfg.Port, 2222)
			}
			if fwCfg.Network != "my-vpc" {
				t.Errorf("Network = %q, want %q", fwCfg.Network, "my-vpc")
			}
		}
	}
	if !found {
		t.Error("EnsureFirewallAllowsSSH was not called")
	}
}

func TestEnsureFirewall_DefaultNetwork(t *testing.T) {
	origDetector := PublicIPDetector
	defer func() { PublicIPDetector = origDetector }()
	PublicIPDetector = func(ctx context.Context) (string, error) {
		return "10.0.0.1", nil
	}

	provider := cloud.NewMockProvider()
	cfg := &config.Config{
		SSH: config.SSHConfig{Port: 22},
		VM:  config.VMConfig{},
	}

	_, err := EnsureFirewall(context.Background(), cfg, provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := provider.GetCalls()
	for _, c := range calls {
		if c.Method == "EnsureFirewallAllowsSSH" {
			fwCfg := c.Args[0].(cloud.FirewallConfig)
			if fwCfg.Network != "default" {
				t.Errorf("Network = %q, want %q", fwCfg.Network, "default")
			}
		}
	}
}

func TestEnsureFirewall_IPDetectionFailure(t *testing.T) {
	origDetector := PublicIPDetector
	defer func() { PublicIPDetector = origDetector }()
	PublicIPDetector = func(ctx context.Context) (string, error) {
		return "", errors.New("network error")
	}

	provider := cloud.NewMockProvider()
	cfg := &config.Config{}

	_, err := EnsureFirewall(context.Background(), cfg, provider)
	if err == nil {
		t.Error("expected error when IP detection fails")
	}
}

func TestEnsureFirewall_FirewallError(t *testing.T) {
	origDetector := PublicIPDetector
	defer func() { PublicIPDetector = origDetector }()
	PublicIPDetector = func(ctx context.Context) (string, error) {
		return "203.0.113.50", nil
	}

	provider := cloud.NewMockProvider()
	provider.EnsureFirewallError = errors.New("permission denied")
	cfg := &config.Config{}

	_, err := EnsureFirewall(context.Background(), cfg, provider)
	if err == nil {
		t.Error("expected error when firewall check fails")
	}
}
