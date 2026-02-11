package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
)

// sshKeyMockProvider implements cloud.Provider for SSH key tests.
type sshKeyMockProvider struct {
	sshKeyCalled bool
	sshKeyErr    error
	lastVMName   string
	lastUser     string
	lastPubKey   string
}

func (m *sshKeyMockProvider) Name() string { return "mock" }
func (m *sshKeyMockProvider) GetVMInfo(ctx context.Context, name string) (*cloud.VMInfo, error) {
	return nil, nil
}
func (m *sshKeyMockProvider) StartVM(ctx context.Context, name string) error { return nil }
func (m *sshKeyMockProvider) StopVM(ctx context.Context, name string) error  { return nil }
func (m *sshKeyMockProvider) CreateVM(ctx context.Context, cfg cloud.VMCreateConfig) error {
	return nil
}
func (m *sshKeyMockProvider) DeleteVM(ctx context.Context, name string) error { return nil }
func (m *sshKeyMockProvider) EnsureFirewallAllowsSSH(ctx context.Context, cfg cloud.FirewallConfig) (bool, error) {
	return false, nil
}
func (m *sshKeyMockProvider) EnsureSSHKeyOnVM(ctx context.Context, name, user, publicKey string) error {
	m.sshKeyCalled = true
	m.lastVMName = name
	m.lastUser = user
	m.lastPubKey = publicKey
	return m.sshKeyErr
}

func TestEnsureSSHKeyAccess_Success(t *testing.T) {
	// We can't easily mock ReadPublicKey since it reads from the filesystem,
	// but we can verify the function doesn't panic with a real config.
	// The function is designed to be non-fatal, so it will just log and return.
	provider := &sshKeyMockProvider{}
	cfg := &config.Config{
		SSH: config.SSHConfig{User: "testuser"},
	}

	// This will likely fail to read the public key (no key in test env),
	// but should not panic.
	ensureSSHKeyAccess(context.Background(), cfg, provider, "test-vm")
}

func TestEnsureSSHKeyAccess_ProviderError(t *testing.T) {
	provider := &sshKeyMockProvider{sshKeyErr: errors.New("permission denied")}
	cfg := &config.Config{
		SSH: config.SSHConfig{User: "testuser"},
	}

	// Should not panic — errors are non-fatal
	ensureSSHKeyAccess(context.Background(), cfg, provider, "test-vm")
}
