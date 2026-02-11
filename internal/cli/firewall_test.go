package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
)

// mockProvider implements cloud.Provider for firewall tests.
type mockProvider struct {
	fwChanged bool
	fwErr     error
	fwCalled  bool
	lastFWCfg cloud.FirewallConfig
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) GetVMInfo(ctx context.Context, name string) (*cloud.VMInfo, error) {
	return nil, nil
}
func (m *mockProvider) StartVM(ctx context.Context, name string) error               { return nil }
func (m *mockProvider) StopVM(ctx context.Context, name string) error                { return nil }
func (m *mockProvider) CreateVM(ctx context.Context, cfg cloud.VMCreateConfig) error { return nil }
func (m *mockProvider) DeleteVM(ctx context.Context, name string) error              { return nil }
func (m *mockProvider) EnsureFirewallAllowsSSH(ctx context.Context, cfg cloud.FirewallConfig) (bool, error) {
	m.fwCalled = true
	m.lastFWCfg = cfg
	return m.fwChanged, m.fwErr
}
func (m *mockProvider) EnsureSSHKeyOnVM(ctx context.Context, name, user, publicKey string) error {
	return nil
}

func TestEnsureFirewallAccess_Success(t *testing.T) {
	origDetector := publicIPDetector
	defer func() { publicIPDetector = origDetector }()
	publicIPDetector = func(ctx context.Context) (string, error) {
		return "203.0.113.50", nil
	}

	provider := &mockProvider{fwChanged: true}
	cfg := &config.Config{
		SSH: config.SSHConfig{Port: 2222},
		VM:  config.VMConfig{Network: "my-vpc"},
	}

	ensureFirewallAccess(context.Background(), cfg, provider)

	if !provider.fwCalled {
		t.Error("EnsureFirewallAllowsSSH was not called")
	}
	if provider.lastFWCfg.SourceIP != "203.0.113.50" {
		t.Errorf("SourceIP = %q, want %q", provider.lastFWCfg.SourceIP, "203.0.113.50")
	}
	if provider.lastFWCfg.Port != 2222 {
		t.Errorf("Port = %d, want %d", provider.lastFWCfg.Port, 2222)
	}
	if provider.lastFWCfg.Network != "my-vpc" {
		t.Errorf("Network = %q, want %q", provider.lastFWCfg.Network, "my-vpc")
	}
}

func TestEnsureFirewallAccess_DefaultNetwork(t *testing.T) {
	origDetector := publicIPDetector
	defer func() { publicIPDetector = origDetector }()
	publicIPDetector = func(ctx context.Context) (string, error) {
		return "10.0.0.1", nil
	}

	provider := &mockProvider{}
	cfg := &config.Config{
		SSH: config.SSHConfig{Port: 22},
		VM:  config.VMConfig{},
	}

	ensureFirewallAccess(context.Background(), cfg, provider)

	if provider.lastFWCfg.Network != "default" {
		t.Errorf("Network = %q, want %q", provider.lastFWCfg.Network, "default")
	}
}

func TestEnsureFirewallAccess_IPDetectionFailure(t *testing.T) {
	origDetector := publicIPDetector
	defer func() { publicIPDetector = origDetector }()
	publicIPDetector = func(ctx context.Context) (string, error) {
		return "", errors.New("network error")
	}

	provider := &mockProvider{}
	cfg := &config.Config{}

	// Should not panic or call provider
	ensureFirewallAccess(context.Background(), cfg, provider)

	if provider.fwCalled {
		t.Error("EnsureFirewallAllowsSSH should not be called when IP detection fails")
	}
}

func TestEnsureFirewallAccess_FirewallError(t *testing.T) {
	origDetector := publicIPDetector
	defer func() { publicIPDetector = origDetector }()
	publicIPDetector = func(ctx context.Context) (string, error) {
		return "203.0.113.50", nil
	}

	provider := &mockProvider{fwErr: errors.New("permission denied")}
	cfg := &config.Config{}

	// Should not panic — error is non-fatal
	ensureFirewallAccess(context.Background(), cfg, provider)

	if !provider.fwCalled {
		t.Error("EnsureFirewallAllowsSSH should have been called")
	}
}
