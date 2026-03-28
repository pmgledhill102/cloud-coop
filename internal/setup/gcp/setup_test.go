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

type mockNetworksClient struct {
	names []string
	err   error
}

func (m *mockNetworksClient) List(ctx context.Context, project string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.names, nil
}

func (m *mockNetworksClient) Close() error { return nil }

type mockSubnetworksClient struct {
	names []string
	err   error
}

func (m *mockSubnetworksClient) List(ctx context.Context, project, region, network string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.names, nil
}

func (m *mockSubnetworksClient) Close() error { return nil }

type mockFirewallsClient struct {
	exists        bool
	rule          *computepb.Firewall
	getErr        error
	insertErr     error
	patchErr      error
	waitErr       error
	lastInsertReq *computepb.InsertFirewallRequest
	lastPatchReq  *computepb.PatchFirewallRequest
}

func (m *mockFirewallsClient) Get(ctx context.Context, req *computepb.GetFirewallRequest) (*computepb.Firewall, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if !m.exists {
		return nil, &googleapi.Error{Code: 404, Message: "not found"}
	}
	if m.rule != nil {
		return m.rule, nil
	}
	return &computepb.Firewall{}, nil
}

func (m *mockFirewallsClient) Insert(ctx context.Context, req *computepb.InsertFirewallRequest) (firewallOperation, error) {
	m.lastInsertReq = req
	if m.insertErr != nil {
		return nil, m.insertErr
	}
	return &mockFirewallOp{waitErr: m.waitErr}, nil
}

func (m *mockFirewallsClient) Patch(ctx context.Context, req *computepb.PatchFirewallRequest) (firewallOperation, error) {
	m.lastPatchReq = req
	if m.patchErr != nil {
		return nil, m.patchErr
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
		&mockNetworksClient{},
		&mockSubnetworksClient{},
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
		&mockNetworksClient{},
		&mockSubnetworksClient{},
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
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		&mockFirewallsClient{},
	)

	statuses, err := p.CheckAPIs(context.Background(), "test-proj", setup.RequiredAPIs)
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
		&mockNetworksClient{},
		&mockSubnetworksClient{},
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
				&mockNetworksClient{},
				&mockSubnetworksClient{},
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
		&mockNetworksClient{},
		&mockSubnetworksClient{},
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
				&mockNetworksClient{},
				&mockSubnetworksClient{},
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
		&mockNetworksClient{},
		&mockSubnetworksClient{},
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
				&mockNetworksClient{},
				&mockSubnetworksClient{},
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
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		&mockFirewallsClient{},
	)

	err := p.CreateIAPFirewallRule(context.Background(), "test-proj", "default", 22)
	if err != nil {
		t.Fatalf("CreateIAPFirewallRule() error = %v", err)
	}
}

func TestCreateIAPFirewallRule_CustomPort(t *testing.T) {
	mock := &mockFirewallsClient{}
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		mock,
	)

	err := p.CreateIAPFirewallRule(context.Background(), "test-proj", "my-vpc", 2222)
	if err != nil {
		t.Fatalf("CreateIAPFirewallRule() error = %v", err)
	}

	// Verify the firewall rule uses port 2222
	req := mock.lastInsertReq
	if req == nil {
		t.Fatal("Insert() was not called")
	}
	rule := req.GetFirewallResource()
	if len(rule.GetAllowed()) != 1 {
		t.Fatalf("Allowed count = %d, want 1", len(rule.GetAllowed()))
	}
	ports := rule.GetAllowed()[0].GetPorts()
	if len(ports) != 1 || ports[0] != "2222" {
		t.Errorf("Allowed ports = %v, want [2222]", ports)
	}
}

func TestCreateIAPFirewallRule_InsertError(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		&mockFirewallsClient{insertErr: errors.New("insert failed")},
	)

	err := p.CreateIAPFirewallRule(context.Background(), "test-proj", "default", 22)
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
		&mockNetworksClient{},
		&mockSubnetworksClient{},
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
		&mockNetworksClient{},
		&mockSubnetworksClient{},
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
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		&mockFirewallsClient{},
	)

	err := p.CheckADCCredentials(context.Background())
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetFirewallRulePort(t *testing.T) {
	port22 := "22"
	port2222 := "2222"
	tests := []struct {
		name    string
		rule    *computepb.Firewall
		getErr  error
		want    int
		wantErr bool
	}{
		{
			name: "port 22",
			rule: &computepb.Firewall{
				Allowed: []*computepb.Allowed{
					{IPProtocol: &port22, Ports: []string{"22"}},
				},
			},
			want: 22,
		},
		{
			name: "port 2222",
			rule: &computepb.Firewall{
				Allowed: []*computepb.Allowed{
					{IPProtocol: &port2222, Ports: []string{"2222"}},
				},
			},
			want: 2222,
		},
		{
			name:    "get error",
			getErr:  errors.New("permission denied"),
			wantErr: true,
		},
		{
			name:    "no allowed entries",
			rule:    &computepb.Firewall{},
			wantErr: true,
		},
		{
			name: "no ports",
			rule: &computepb.Firewall{
				Allowed: []*computepb.Allowed{
					{IPProtocol: &port22},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newWithClients(
				&mockProjectsClient{},
				&mockServiceUsageClient{services: map[string]serviceState{}},
				&mockIAMClient{},
				&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
				&mockNetworksClient{},
				&mockSubnetworksClient{},
				&mockFirewallsClient{exists: true, rule: tt.rule, getErr: tt.getErr},
			)

			got, err := p.GetFirewallRulePort(context.Background(), "test-proj", setup.IAPFirewallRuleName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetFirewallRulePort() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetFirewallRulePort() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUpdateIAPFirewallRule(t *testing.T) {
	mock := &mockFirewallsClient{}
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		mock,
	)

	err := p.UpdateIAPFirewallRule(context.Background(), "test-proj", 2222)
	if err != nil {
		t.Fatalf("UpdateIAPFirewallRule() error = %v", err)
	}

	req := mock.lastPatchReq
	if req == nil {
		t.Fatal("Patch() was not called")
	}
	if req.GetFirewall() != setup.IAPFirewallRuleName {
		t.Errorf("Firewall name = %q, want %q", req.GetFirewall(), setup.IAPFirewallRuleName)
	}
	rule := req.GetFirewallResource()
	ports := rule.GetAllowed()[0].GetPorts()
	if len(ports) != 1 || ports[0] != "2222" {
		t.Errorf("Patched ports = %v, want [2222]", ports)
	}
}

func TestUpdateIAPFirewallRule_PatchError(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		&mockFirewallsClient{patchErr: errors.New("patch failed")},
	)

	err := p.UpdateIAPFirewallRule(context.Background(), "test-proj", 2222)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCreateDirectSSHFirewallRule(t *testing.T) {
	mock := &mockFirewallsClient{}
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		mock,
	)

	err := p.CreateDirectSSHFirewallRule(context.Background(), "test-proj", "default", "203.0.113.50", 2222)
	if err != nil {
		t.Fatalf("CreateDirectSSHFirewallRule() error = %v", err)
	}

	req := mock.lastInsertReq
	if req == nil {
		t.Fatal("Insert() was not called")
	}
	rule := req.GetFirewallResource()
	if rule.GetName() != setup.DirectSSHFirewallRuleName {
		t.Errorf("rule name = %q, want %q", rule.GetName(), setup.DirectSSHFirewallRuleName)
	}
	ranges := rule.GetSourceRanges()
	if len(ranges) != 1 || ranges[0] != "203.0.113.50/32" {
		t.Errorf("source ranges = %v, want [203.0.113.50/32]", ranges)
	}
	ports := rule.GetAllowed()[0].GetPorts()
	if len(ports) != 1 || ports[0] != "2222" {
		t.Errorf("ports = %v, want [2222]", ports)
	}
}

func TestCreateDirectSSHFirewallRule_InsertError(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		&mockFirewallsClient{insertErr: errors.New("insert failed")},
	)

	err := p.CreateDirectSSHFirewallRule(context.Background(), "test-proj", "default", "203.0.113.50", 22)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetFirewallRuleSourceIP(t *testing.T) {
	tcp := "tcp"
	tests := []struct {
		name     string
		rule     *computepb.Firewall
		getErr   error
		wantIP   string
		wantPort int
		wantErr  bool
	}{
		{
			name: "standard rule",
			rule: &computepb.Firewall{
				SourceRanges: []string{"203.0.113.50/32"},
				Allowed: []*computepb.Allowed{
					{IPProtocol: &tcp, Ports: []string{"2222"}},
				},
			},
			wantIP:   "203.0.113.50",
			wantPort: 2222,
		},
		{
			name: "port 22",
			rule: &computepb.Firewall{
				SourceRanges: []string{"10.0.0.1/32"},
				Allowed: []*computepb.Allowed{
					{IPProtocol: &tcp, Ports: []string{"22"}},
				},
			},
			wantIP:   "10.0.0.1",
			wantPort: 22,
		},
		{
			name:    "get error",
			getErr:  errors.New("permission denied"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newWithClients(
				&mockProjectsClient{},
				&mockServiceUsageClient{services: map[string]serviceState{}},
				&mockIAMClient{},
				&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
				&mockNetworksClient{},
				&mockSubnetworksClient{},
				&mockFirewallsClient{exists: true, rule: tt.rule, getErr: tt.getErr},
			)

			ip, port, err := p.GetFirewallRuleSourceIP(context.Background(), "test-proj", setup.DirectSSHFirewallRuleName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetFirewallRuleSourceIP() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if ip != tt.wantIP {
					t.Errorf("IP = %q, want %q", ip, tt.wantIP)
				}
				if port != tt.wantPort {
					t.Errorf("Port = %d, want %d", port, tt.wantPort)
				}
			}
		})
	}
}

func TestUpdateDirectSSHFirewallRule(t *testing.T) {
	mock := &mockFirewallsClient{}
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		mock,
	)

	err := p.UpdateDirectSSHFirewallRule(context.Background(), "test-proj", "203.0.113.50", 2222)
	if err != nil {
		t.Fatalf("UpdateDirectSSHFirewallRule() error = %v", err)
	}

	req := mock.lastPatchReq
	if req == nil {
		t.Fatal("Patch() was not called")
	}
	if req.GetFirewall() != setup.DirectSSHFirewallRuleName {
		t.Errorf("Firewall name = %q, want %q", req.GetFirewall(), setup.DirectSSHFirewallRuleName)
	}
	rule := req.GetFirewallResource()
	ranges := rule.GetSourceRanges()
	if len(ranges) != 1 || ranges[0] != "203.0.113.50/32" {
		t.Errorf("patched source ranges = %v, want [203.0.113.50/32]", ranges)
	}
	ports := rule.GetAllowed()[0].GetPorts()
	if len(ports) != 1 || ports[0] != "2222" {
		t.Errorf("Patched ports = %v, want [2222]", ports)
	}
}

func TestUpdateDirectSSHFirewallRule_PatchError(t *testing.T) {
	p := newWithClients(
		&mockProjectsClient{},
		&mockServiceUsageClient{services: map[string]serviceState{}},
		&mockIAMClient{},
		&mockIAMPolicyClient{policy: &cloudresourcemanager.Policy{}},
		&mockNetworksClient{},
		&mockSubnetworksClient{},
		&mockFirewallsClient{patchErr: errors.New("patch failed")},
	)

	err := p.UpdateDirectSSHFirewallRule(context.Background(), "test-proj", "203.0.113.50", 2222)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
