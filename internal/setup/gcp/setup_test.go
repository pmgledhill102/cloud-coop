package gcp

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cloud-coop/cloudcoop/internal/setup"
)

// --- Mock implementations ---

type mockProjectsClient struct {
	projects []projectResult
	err      error
}

func (m *mockProjectsClient) ListProjects(ctx context.Context) ([]projectResult, error) {
	return m.projects, m.err
}

func (m *mockProjectsClient) Close() error { return nil }

type mockServiceUsageClient struct {
	services map[string]serviceState
	getErr   error
	enableOK bool
}

func (m *mockServiceUsageClient) GetService(ctx context.Context, name string) (serviceState, error) {
	if m.getErr != nil {
		return serviceStateDisabled, m.getErr
	}
	if state, ok := m.services[name]; ok {
		return state, nil
	}
	return serviceStateDisabled, nil
}

func (m *mockServiceUsageClient) EnableService(ctx context.Context, name string) error {
	if m.enableOK {
		m.services[name] = serviceStateEnabled
	}
	return nil
}

func (m *mockServiceUsageClient) Close() error { return nil }

type mockIAMClient struct {
	exists   bool
	getErr   error
	createOK bool
}

func (m *mockIAMClient) GetServiceAccount(ctx context.Context, name string) error {
	if m.getErr != nil {
		return m.getErr
	}
	if !m.exists {
		return &googleapi.Error{Code: 404, Message: "not found"}
	}
	return nil
}

func (m *mockIAMClient) CreateServiceAccount(ctx context.Context, project, accountID, displayName string) (string, error) {
	if m.createOK {
		return accountID + "@" + project + ".iam.gserviceaccount.com", nil
	}
	return "", errors.New("create failed")
}

func (m *mockIAMClient) Close() error { return nil }

type mockIAMPolicyClient struct {
	policy *cloudresourcemanager.Policy
	getErr error
	setErr error
}

func (m *mockIAMPolicyClient) GetIAMPolicy(ctx context.Context, project string) (*cloudresourcemanager.Policy, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.policy, nil
}

func (m *mockIAMPolicyClient) SetIAMPolicy(ctx context.Context, project string, policy *cloudresourcemanager.Policy) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.policy = policy
	return nil
}

func (m *mockIAMPolicyClient) Close() error { return nil }

type mockFirewallsClient struct {
	exists    bool
	getErr    error
	insertErr error
	waitErr   error
}

func (m *mockFirewallsClient) Get(ctx context.Context, req *computepb.GetFirewallRequest) (*computepb.Firewall, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if !m.exists {
		return nil, &googleapi.Error{Code: 404, Message: "not found"}
	}
	return &computepb.Firewall{}, nil
}

func (m *mockFirewallsClient) Insert(ctx context.Context, req *computepb.InsertFirewallRequest) (firewallOperation, error) {
	if m.insertErr != nil {
		return nil, m.insertErr
	}
	return &mockFirewallOp{waitErr: m.waitErr}, nil
}

func (m *mockFirewallsClient) Close() error { return nil }

type mockFirewallOp struct {
	waitErr error
}

func (m *mockFirewallOp) Wait(ctx context.Context) error { return m.waitErr }

// --- Tests ---

func TestListProjects(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{
			projects: []projectResult{
				{ProjectID: "proj-1", DisplayName: "Project 1"},
				{ProjectID: "proj-2", DisplayName: "Project 2"},
			},
		},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockFirewallsClient{},
	)

	projects, err := p.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("ListProjects() count = %d, want 2", len(projects))
	}
	if projects[0].ID != "proj-1" {
		t.Errorf("projects[0].ID = %q, want %q", projects[0].ID, "proj-1")
	}
}

func TestListProjects_Error(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{err: errors.New("auth error")},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockFirewallsClient{},
	)

	_, err := p.ListProjects(context.Background())
	if err == nil {
		t.Error("ListProjects() expected error, got nil")
	}
}

func TestCheckAPIs(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{
			services: map[string]serviceState{
				"projects/test-proj/services/compute.googleapis.com": serviceStateEnabled,
				"projects/test-proj/services/iam.googleapis.com":     serviceStateEnabled,
			},
		},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockFirewallsClient{},
	)

	statuses, err := p.CheckAPIs(context.Background(), "test-proj")
	if err != nil {
		t.Fatalf("CheckAPIs() error = %v", err)
	}
	if len(statuses) != len(setup.RequiredAPIs) {
		t.Fatalf("CheckAPIs() count = %d, want %d", len(statuses), len(setup.RequiredAPIs))
	}

	// compute and iam should be enabled
	for _, s := range statuses {
		switch s.Name {
		case "compute.googleapis.com", "iam.googleapis.com":
			if !s.Enabled {
				t.Errorf("API %s should be enabled", s.Name)
			}
		case "logging.googleapis.com", "monitoring.googleapis.com":
			if s.Enabled {
				t.Errorf("API %s should be disabled", s.Name)
			}
		}
	}
}

func TestEnableAPI(t *testing.T) {
	su := &mockServiceUsageClient{
		services: map[string]serviceState{},
		enableOK: true,
	}
	p := newWithClients(
		&mockProjectsClient{},
		su,
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockFirewallsClient{},
	)

	err := p.EnableAPI(context.Background(), "test-proj", "iam.googleapis.com")
	if err != nil {
		t.Fatalf("EnableAPI() error = %v", err)
	}
}

func TestServiceAccountExists(t *testing.T) {
	tests := []struct {
		name   string
		exists bool
		getErr error
		want   bool
	}{
		{
			name:   "exists",
			exists: true,
			want:   true,
		},
		{
			name:   "not found via REST 404",
			exists: false,
			want:   false,
		},
		{
			name:   "not found via gRPC",
			exists: false,
			getErr: status.Error(codes.NotFound, "Unknown service account"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newWithClients(
				&mockProjectsClient{},
				&mockServiceUsageClient{services: map[string]serviceState{}},
				&mockIAMClient{exists: tt.exists, getErr: tt.getErr},
				&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
				&mockFirewallsClient{},
			)

			got, err := p.ServiceAccountExists(context.Background(), "test-proj", "cloudcoop-vm")
			if err != nil {
				t.Fatalf("ServiceAccountExists() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ServiceAccountExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateServiceAccount(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{createOK: true},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockFirewallsClient{},
	)

	email, err := p.CreateServiceAccount(context.Background(), "test-proj", "cloudcoop-vm", "test display")
	if err != nil {
		t.Fatalf("CreateServiceAccount() error = %v", err)
	}
	if email != "cloudcoop-vm@test-proj.iam.gserviceaccount.com" {
		t.Errorf("email = %q, want %q", email, "cloudcoop-vm@test-proj.iam.gserviceaccount.com")
	}
}

func TestCheckIAMBinding(t *testing.T) {
	tests := []struct {
		name   string
		policy *cloudresourcemanager.Policy
		member string
		role   string
		want   bool
	}{
		{
			name: "binding exists",
			policy: &cloudresourcemanager.Policy{
				Bindings: []*cloudresourcemanager.Binding{
					{
						Role:    "roles/logging.logWriter",
						Members: []string{"serviceAccount:sa@proj.iam.gserviceaccount.com"},
					},
				},
			},
			member: "serviceAccount:sa@proj.iam.gserviceaccount.com",
			role:   "roles/logging.logWriter",
			want:   true,
		},
		{
			name: "binding does not exist",
			policy: &cloudresourcemanager.Policy{
				Bindings: []*cloudresourcemanager.Binding{
					{
						Role:    "roles/logging.logWriter",
						Members: []string{"serviceAccount:other@proj.iam.gserviceaccount.com"},
					},
				},
			},
			member: "serviceAccount:sa@proj.iam.gserviceaccount.com",
			role:   "roles/logging.logWriter",
			want:   false,
		},
		{
			name:   "empty policy",
			policy: &cloudresourcemanager.Policy{},
			member: "serviceAccount:sa@proj.iam.gserviceaccount.com",
			role:   "roles/logging.logWriter",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newWithClients(
				&mockProjectsClient{},
				&mockServiceUsageClient{services: map[string]serviceState{}},
				&mockIAMClient{},
				&mockIAMPolicyClient{policy: tt.policy},
				&mockFirewallsClient{},
			)

			got, err := p.CheckIAMBinding(context.Background(), "test-proj", tt.member, tt.role)
			if err != nil {
				t.Fatalf("CheckIAMBinding() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("CheckIAMBinding() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGrantIAMRole(t *testing.T) {
	policyClient := &mockIAMPolicyClient{
		policy: &cloudresourcemanager.Policy{},
	}
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		policyClient,
		&mockFirewallsClient{},
	)

	err := p.GrantIAMRole(context.Background(), "test-proj",
		"serviceAccount:sa@test-proj.iam.gserviceaccount.com",
		"roles/logging.logWriter")
	if err != nil {
		t.Fatalf("GrantIAMRole() error = %v", err)
	}

	// Verify binding was added
	found := false
	for _, b := range policyClient.policy.Bindings {
		if b.Role == "roles/logging.logWriter" {
			for _, m := range b.Members {
				if m == "serviceAccount:sa@test-proj.iam.gserviceaccount.com" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("GrantIAMRole() did not add the binding")
	}
}

func TestFirewallRuleExists(t *testing.T) {
	tests := []struct {
		name   string
		exists bool
		want   bool
	}{
		{"exists", true, true},
		{"not found", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newWithClients(
				&mockProjectsClient{},
				&mockServiceUsageClient{services: map[string]serviceState{}},
				&mockIAMClient{},
				&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
				&mockFirewallsClient{exists: tt.exists},
			)

			got, err := p.FirewallRuleExists(context.Background(), "test-proj", setup.IAPFirewallRuleName)
			if err != nil {
				t.Fatalf("FirewallRuleExists() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("FirewallRuleExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateIAPFirewallRule(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockFirewallsClient{},
	)

	err := p.CreateIAPFirewallRule(context.Background(), "test-proj", "default")
	if err != nil {
		t.Fatalf("CreateIAPFirewallRule() error = %v", err)
	}
}

func TestCreateIAPFirewallRule_InsertError(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockFirewallsClient{insertErr: errors.New("insert failed")},
	)

	err := p.CreateIAPFirewallRule(context.Background(), "test-proj", "default")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestClose(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockFirewallsClient{},
	)

	err := p.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestCheckADCCredentials(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{projects: []projectResult{{ProjectID: "proj-1"}}},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockFirewallsClient{},
	)

	err := p.CheckADCCredentials(context.Background())
	if err != nil {
		t.Errorf("CheckADCCredentials() error = %v", err)
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "googleapi 404",
			err:  &googleapi.Error{Code: 404, Message: "not found"},
			want: true,
		},
		{
			name: "googleapi 403",
			err:  &googleapi.Error{Code: 403, Message: "forbidden"},
			want: false,
		},
		{
			name: "gRPC NotFound",
			err:  status.Error(codes.NotFound, "Unknown service account"),
			want: true,
		},
		{
			name: "gRPC PermissionDenied",
			err:  status.Error(codes.PermissionDenied, "denied"),
			want: false,
		},
		{
			name: "generic error",
			err:  errors.New("something failed"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNotFoundError(tt.err)
			if got != tt.want {
				t.Errorf("isNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckADCCredentials_Error(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{err: errors.New("auth error")},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockFirewallsClient{},
	)

	err := p.CheckADCCredentials(context.Background())
	if err == nil {
		t.Error("expected error, got nil")
	}
}
