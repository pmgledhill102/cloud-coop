// Package gcp implements the setup.SetupProvider interface for Google Cloud Platform.
package gcp

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cloud-coop/cloudcoop/internal/setup"
)

// Provider implements setup.SetupProvider for GCP.
type Provider struct {
	projects     projectsClient
	serviceUsage serviceUsageClient
	iam          iamClient
	iamPolicy    iamPolicyClient
	firewalls    firewallsClient
}

// New creates a new GCP setup provider using real clients.
func New(ctx context.Context) (*Provider, error) {
	projects, err := newRealProjectsClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create projects client: %w", err)
	}

	su, err := newRealServiceUsageClient(ctx)
	if err != nil {
		_ = projects.Close()
		return nil, fmt.Errorf("create service usage client: %w", err)
	}

	iamC, err := newRealIAMClient(ctx)
	if err != nil {
		_ = projects.Close()
		_ = su.Close()
		return nil, fmt.Errorf("create IAM client: %w", err)
	}

	iamP, err := newRealIAMPolicyClient(ctx)
	if err != nil {
		_ = projects.Close()
		_ = su.Close()
		_ = iamC.Close()
		return nil, fmt.Errorf("create IAM policy client: %w", err)
	}

	fw, err := newRealFirewallsClient(ctx)
	if err != nil {
		_ = projects.Close()
		_ = su.Close()
		_ = iamC.Close()
		_ = iamP.Close()
		return nil, fmt.Errorf("create firewalls client: %w", err)
	}

	return &Provider{
		projects:     projects,
		serviceUsage: su,
		iam:          iamC,
		iamPolicy:    iamP,
		firewalls:    fw,
	}, nil
}

// newWithClients creates a provider with injected clients (for testing).
func newWithClients(
	projects projectsClient,
	su serviceUsageClient,
	iamC iamClient,
	iamP iamPolicyClient,
	fw firewallsClient,
) *Provider {
	return &Provider{
		projects:     projects,
		serviceUsage: su,
		iam:          iamC,
		iamPolicy:    iamP,
		firewalls:    fw,
	}
}

// Ensure Provider implements SetupProvider.
var _ setup.SetupProvider = (*Provider)(nil)

// ListProjects returns available GCP projects.
func (p *Provider) ListProjects(ctx context.Context) ([]setup.ProjectInfo, error) {
	results, err := p.projects.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	projects := make([]setup.ProjectInfo, len(results))
	for i, r := range results {
		projects[i] = setup.ProjectInfo{
			ID:   r.ProjectID,
			Name: r.DisplayName,
		}
	}
	return projects, nil
}

// CheckAPIs checks which of the given APIs are enabled.
func (p *Provider) CheckAPIs(ctx context.Context, project string, apis []string) ([]setup.APIStatus, error) {
	statuses := make([]setup.APIStatus, len(apis))
	for i, api := range apis {
		name := fmt.Sprintf("projects/%s/services/%s", project, api)
		state, err := p.serviceUsage.GetService(ctx, name)
		if err != nil {
			// If we get a not-found-style error, treat as disabled
			if isNotFoundError(err) {
				statuses[i] = setup.APIStatus{Name: api, Enabled: false}
				continue
			}
			return nil, fmt.Errorf("check API %s: %w", api, err)
		}
		statuses[i] = setup.APIStatus{
			Name:    api,
			Enabled: state == serviceStateEnabled,
		}
	}
	return statuses, nil
}

// EnableAPI enables a single API.
func (p *Provider) EnableAPI(ctx context.Context, project, api string) error {
	name := fmt.Sprintf("projects/%s/services/%s", project, api)
	if err := p.serviceUsage.EnableService(ctx, name); err != nil {
		return fmt.Errorf("enable API %s: %w", api, err)
	}
	return nil
}

// ServiceAccountExists checks if a service account exists.
func (p *Provider) ServiceAccountExists(ctx context.Context, project, name string) (bool, error) {
	fullName := fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com", project, name, project)
	err := p.iam.GetServiceAccount(ctx, fullName)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("check service account: %w", err)
	}
	return true, nil
}

// CreateServiceAccount creates a new service account and returns its email.
func (p *Provider) CreateServiceAccount(ctx context.Context, project, name, displayName string) (string, error) {
	email, err := p.iam.CreateServiceAccount(ctx, project, name, displayName)
	if err != nil {
		return "", fmt.Errorf("create service account: %w", err)
	}
	return email, nil
}

// CheckIAMBinding checks if a specific IAM binding exists.
func (p *Provider) CheckIAMBinding(ctx context.Context, project, member, role string) (bool, error) {
	policy, err := p.iamPolicy.GetIAMPolicy(ctx, project)
	if err != nil {
		return false, fmt.Errorf("get IAM policy: %w", err)
	}

	for _, binding := range policy.Bindings {
		if binding.Role != role {
			continue
		}
		for _, m := range binding.Members {
			if m == member {
				return true, nil
			}
		}
	}
	return false, nil
}

// GrantIAMRole adds an IAM role binding for the given member.
func (p *Provider) GrantIAMRole(ctx context.Context, project, member, role string) error {
	policy, err := p.iamPolicy.GetIAMPolicy(ctx, project)
	if err != nil {
		return fmt.Errorf("get IAM policy: %w", err)
	}

	// Check if binding already exists
	var found bool
	for _, binding := range policy.Bindings {
		if binding.Role != role {
			continue
		}
		for _, m := range binding.Members {
			if m == member {
				found = true
				break
			}
		}
		if !found {
			binding.Members = append(binding.Members, member)
			found = true
		}
		break
	}

	if !found {
		policy.Bindings = append(policy.Bindings, &cloudresourcemanager.Binding{
			Role:    role,
			Members: []string{member},
		})
	}

	if err := p.iamPolicy.SetIAMPolicy(ctx, project, policy); err != nil {
		return fmt.Errorf("set IAM policy: %w", err)
	}
	return nil
}

// FirewallRuleExists checks if the IAP firewall rule exists.
func (p *Provider) FirewallRuleExists(ctx context.Context, project, name string) (bool, error) {
	_, err := p.firewalls.Get(ctx, &computepb.GetFirewallRequest{
		Project:  project,
		Firewall: name,
	})
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("check firewall rule: %w", err)
	}
	return true, nil
}

// CreateIAPFirewallRule creates a firewall rule allowing SSH from Google IAP.
func (p *Provider) CreateIAPFirewallRule(ctx context.Context, project, network string) error {
	rule := &computepb.Firewall{
		Name:        ptr(setup.IAPFirewallRuleName),
		Description: ptr("Allow SSH from Google IAP for cloudcoop"),
		Network:     ptr(fmt.Sprintf("global/networks/%s", network)),
		Direction:   ptr("INGRESS"),
		Priority:    ptr(int32(1000)),
		SourceRanges: []string{
			"35.235.240.0/20", // Google IAP IP range
		},
		Allowed: []*computepb.Allowed{
			{
				IPProtocol: ptr("tcp"),
				Ports:      []string{"22"},
			},
		},
	}

	op, err := p.firewalls.Insert(ctx, &computepb.InsertFirewallRequest{
		Project:          project,
		FirewallResource: rule,
	})
	if err != nil {
		return fmt.Errorf("create IAP firewall rule: %w", err)
	}

	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("wait for firewall rule: %w", err)
	}

	return nil
}

// CheckADCCredentials verifies that Application Default Credentials are available.
func (p *Provider) CheckADCCredentials(ctx context.Context) error {
	// The fact that we have working clients means ADC is available.
	// Do a lightweight check by listing projects (which requires valid credentials).
	_, err := p.projects.ListProjects(ctx)
	if err != nil {
		return fmt.Errorf("GCP credentials not found or invalid: %w", err)
	}
	return nil
}

// Close releases all GCP clients.
func (p *Provider) Close() error {
	var errs []error
	if p.projects != nil {
		errs = append(errs, p.projects.Close())
	}
	if p.serviceUsage != nil {
		errs = append(errs, p.serviceUsage.Close())
	}
	if p.iam != nil {
		errs = append(errs, p.iam.Close())
	}
	if p.iamPolicy != nil {
		errs = append(errs, p.iamPolicy.Close())
	}
	if p.firewalls != nil {
		errs = append(errs, p.firewalls.Close())
	}
	return errors.Join(errs...)
}

// ptr returns a pointer to the given value.
func ptr[T any](v T) *T { return &v }

// isNotFoundError checks if the error is a "not found" error.
// It handles both REST (googleapi.Error 404) and gRPC (codes.NotFound) errors.
func isNotFoundError(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == 404
	}
	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.NotFound
	}
	return false
}
