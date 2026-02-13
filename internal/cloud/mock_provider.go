package cloud

import (
	"context"
	"sync"
)

// MockProvider is a test double for Provider interface.
// It allows configuring responses for each method call.
type MockProvider struct {
	mu sync.Mutex

	// ProviderName is returned by Name().
	ProviderName string

	// VMInfoResponse is returned by GetVMInfo().
	VMInfoResponse *VMInfo
	VMInfoError    error

	// StartVMError is returned by StartVM().
	StartVMError error

	// StopVMError is returned by StopVM().
	StopVMError error

	// CreateVMError is returned by CreateVM().
	CreateVMError error

	// DeleteVMError is returned by DeleteVM().
	DeleteVMError error

	// EnsureFirewallChanged is the changed value returned by EnsureFirewallAllowsSSH().
	EnsureFirewallChanged bool
	// EnsureFirewallError is returned by EnsureFirewallAllowsSSH().
	EnsureFirewallError error

	// EnsureSSHKeyError is returned by EnsureSSHKeyOnVM().
	EnsureSSHKeyError error

	// PreflightResponse is returned by Preflight().
	PreflightResponse *PreflightResult
	// PreflightError is returned by Preflight().
	PreflightError error

	// CallLog records method calls for verification.
	CallLog []MockCall
}

// MockCall represents a method call to the mock.
type MockCall struct {
	Method string
	Args   []interface{}
}

// Ensure MockProvider implements Provider.
var _ Provider = (*MockProvider)(nil)

// NewMockProvider creates a new MockProvider with sensible defaults.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		ProviderName: "mock",
		VMInfoResponse: &VMInfo{
			Name:       "test-vm",
			Status:     VMStatusRunning,
			Zone:       "us-central1-a",
			ExternalIP: "203.0.113.1",
			InternalIP: "10.0.0.1",
		},
	}
}

// Name returns the provider name.
func (m *MockProvider) Name() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallLog = append(m.CallLog, MockCall{Method: "Name"})
	return m.ProviderName
}

// GetVMInfo returns the configured VMInfo or error.
func (m *MockProvider) GetVMInfo(ctx context.Context, name string) (*VMInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallLog = append(m.CallLog, MockCall{Method: "GetVMInfo", Args: []interface{}{name}})
	if m.VMInfoError != nil {
		return nil, m.VMInfoError
	}
	return m.VMInfoResponse, nil
}

// StartVM returns the configured error (nil for success).
func (m *MockProvider) StartVM(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallLog = append(m.CallLog, MockCall{Method: "StartVM", Args: []interface{}{name}})
	return m.StartVMError
}

// StopVM returns the configured error (nil for success).
func (m *MockProvider) StopVM(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallLog = append(m.CallLog, MockCall{Method: "StopVM", Args: []interface{}{name}})
	return m.StopVMError
}

// CreateVM returns the configured error (nil for success).
func (m *MockProvider) CreateVM(ctx context.Context, config VMCreateConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallLog = append(m.CallLog, MockCall{Method: "CreateVM", Args: []interface{}{config}})
	return m.CreateVMError
}

// DeleteVM returns the configured error (nil for success).
func (m *MockProvider) DeleteVM(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallLog = append(m.CallLog, MockCall{Method: "DeleteVM", Args: []interface{}{name}})
	return m.DeleteVMError
}

// EnsureFirewallAllowsSSH returns the configured changed/error.
func (m *MockProvider) EnsureFirewallAllowsSSH(ctx context.Context, cfg FirewallConfig) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallLog = append(m.CallLog, MockCall{Method: "EnsureFirewallAllowsSSH", Args: []interface{}{cfg}})
	return m.EnsureFirewallChanged, m.EnsureFirewallError
}

// EnsureSSHKeyOnVM returns the configured error (nil for success).
func (m *MockProvider) EnsureSSHKeyOnVM(ctx context.Context, name, user, publicKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallLog = append(m.CallLog, MockCall{Method: "EnsureSSHKeyOnVM", Args: []interface{}{name, user, publicKey}})
	return m.EnsureSSHKeyError
}

// Preflight returns the configured PreflightResult or error.
func (m *MockProvider) Preflight(ctx context.Context, cfg PreflightConfig) (*PreflightResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallLog = append(m.CallLog, MockCall{Method: "Preflight", Args: []interface{}{cfg}})
	if m.PreflightError != nil {
		return nil, m.PreflightError
	}
	if m.PreflightResponse != nil {
		return m.PreflightResponse, nil
	}
	return &PreflightResult{}, nil
}

// WithVMInfo sets the VMInfo response and returns the mock for chaining.
func (m *MockProvider) WithVMInfo(info *VMInfo) *MockProvider {
	m.VMInfoResponse = info
	return m
}

// WithVMStatus sets the VM status and returns the mock for chaining.
func (m *MockProvider) WithVMStatus(status VMStatus) *MockProvider {
	if m.VMInfoResponse == nil {
		m.VMInfoResponse = &VMInfo{}
	}
	m.VMInfoResponse.Status = status
	return m
}

// WithVMInfoError sets the GetVMInfo error and returns the mock for chaining.
func (m *MockProvider) WithVMInfoError(err error) *MockProvider {
	m.VMInfoError = err
	return m
}

// WithStartVMError sets the StartVM error and returns the mock for chaining.
func (m *MockProvider) WithStartVMError(err error) *MockProvider {
	m.StartVMError = err
	return m
}

// WithStopVMError sets the StopVM error and returns the mock for chaining.
func (m *MockProvider) WithStopVMError(err error) *MockProvider {
	m.StopVMError = err
	return m
}

// WithCreateVMError sets the CreateVM error and returns the mock for chaining.
func (m *MockProvider) WithCreateVMError(err error) *MockProvider {
	m.CreateVMError = err
	return m
}

// WithDeleteVMError sets the DeleteVM error and returns the mock for chaining.
func (m *MockProvider) WithDeleteVMError(err error) *MockProvider {
	m.DeleteVMError = err
	return m
}

// GetCalls returns a copy of the call log.
func (m *MockProvider) GetCalls() []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]MockCall, len(m.CallLog))
	copy(calls, m.CallLog)
	return calls
}

// Reset clears the call log.
func (m *MockProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallLog = nil
}
