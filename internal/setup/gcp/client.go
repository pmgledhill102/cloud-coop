package gcp

import (
	"context"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	iam "cloud.google.com/go/iam/admin/apiv1"
	"cloud.google.com/go/iam/admin/apiv1/adminpb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	serviceusage "cloud.google.com/go/serviceusage/apiv1"
	"cloud.google.com/go/serviceusage/apiv1/serviceusagepb"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iterator"
)

// projectsClient lists GCP projects.
type projectsClient interface {
	ListProjects(ctx context.Context) ([]projectResult, error)
	Close() error
}

type projectResult struct {
	ProjectID   string
	DisplayName string
}

// serviceUsageClient manages GCP API enablement.
type serviceUsageClient interface {
	GetService(ctx context.Context, name string) (serviceState, error)
	EnableService(ctx context.Context, name string) error
	Close() error
}

type serviceState int

const (
	serviceStateDisabled serviceState = iota
	serviceStateEnabled
)

// iamClient manages GCP service accounts.
type iamClient interface {
	GetServiceAccount(ctx context.Context, name string) error
	CreateServiceAccount(ctx context.Context, project, accountID, displayName string) (string, error)
	Close() error
}

// iamPolicyClient manages IAM policy bindings.
type iamPolicyClient interface {
	GetIAMPolicy(ctx context.Context, project string) (*cloudresourcemanager.Policy, error)
	SetIAMPolicy(ctx context.Context, project string, policy *cloudresourcemanager.Policy) error
	Close() error
}

// firewallsClient manages GCP firewall rules.
type firewallsClient interface {
	Get(ctx context.Context, req *computepb.GetFirewallRequest) (*computepb.Firewall, error)
	Insert(ctx context.Context, req *computepb.InsertFirewallRequest) (firewallOperation, error)
	Patch(ctx context.Context, req *computepb.PatchFirewallRequest) (firewallOperation, error)
	Close() error
}

type firewallOperation interface {
	Wait(ctx context.Context) error
}

// --- Real implementations ---

// realProjectsClient wraps the GCP Resource Manager client.
type realProjectsClient struct {
	client *resourcemanager.ProjectsClient
}

func newRealProjectsClient(ctx context.Context) (*realProjectsClient, error) {
	c, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return nil, err
	}
	return &realProjectsClient{client: c}, nil
}

func (r *realProjectsClient) ListProjects(ctx context.Context) ([]projectResult, error) {
	it := r.client.SearchProjects(ctx, &resourcemanagerpb.SearchProjectsRequest{})
	var results []projectResult
	for {
		proj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if proj.State == resourcemanagerpb.Project_ACTIVE {
			results = append(results, projectResult{
				ProjectID:   proj.ProjectId,
				DisplayName: proj.DisplayName,
			})
		}
	}
	return results, nil
}

func (r *realProjectsClient) Close() error {
	return r.client.Close()
}

// realServiceUsageClient wraps the GCP Service Usage client.
type realServiceUsageClient struct {
	client *serviceusage.Client
}

func newRealServiceUsageClient(ctx context.Context) (*realServiceUsageClient, error) {
	c, err := serviceusage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &realServiceUsageClient{client: c}, nil
}

func (r *realServiceUsageClient) GetService(ctx context.Context, name string) (serviceState, error) {
	svc, err := r.client.GetService(ctx, &serviceusagepb.GetServiceRequest{
		Name: name,
	})
	if err != nil {
		return serviceStateDisabled, err
	}
	if svc.State == serviceusagepb.State_ENABLED {
		return serviceStateEnabled, nil
	}
	return serviceStateDisabled, nil
}

func (r *realServiceUsageClient) EnableService(ctx context.Context, name string) error {
	op, err := r.client.EnableService(ctx, &serviceusagepb.EnableServiceRequest{
		Name: name,
	})
	if err != nil {
		return err
	}
	_, err = op.Wait(ctx)
	return err
}

func (r *realServiceUsageClient) Close() error {
	return r.client.Close()
}

// realIAMClient wraps the GCP IAM admin client.
type realIAMClient struct {
	client *iam.IamClient
}

func newRealIAMClient(ctx context.Context) (*realIAMClient, error) {
	c, err := iam.NewIamClient(ctx)
	if err != nil {
		return nil, err
	}
	return &realIAMClient{client: c}, nil
}

func (r *realIAMClient) GetServiceAccount(ctx context.Context, name string) error {
	_, err := r.client.GetServiceAccount(ctx, &adminpb.GetServiceAccountRequest{
		Name: name,
	})
	return err
}

func (r *realIAMClient) CreateServiceAccount(ctx context.Context, project, accountID, displayName string) (string, error) {
	sa, err := r.client.CreateServiceAccount(ctx, &adminpb.CreateServiceAccountRequest{
		Name:      "projects/" + project,
		AccountId: accountID,
		ServiceAccount: &adminpb.ServiceAccount{
			DisplayName: displayName,
		},
	})
	if err != nil {
		return "", err
	}
	return sa.Email, nil
}

func (r *realIAMClient) Close() error {
	return r.client.Close()
}

// realIAMPolicyClient wraps the v1 Cloud Resource Manager for IAM policies.
type realIAMPolicyClient struct {
	svc *cloudresourcemanager.Service
}

func newRealIAMPolicyClient(ctx context.Context) (*realIAMPolicyClient, error) {
	svc, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return nil, err
	}
	return &realIAMPolicyClient{svc: svc}, nil
}

func (r *realIAMPolicyClient) GetIAMPolicy(ctx context.Context, project string) (*cloudresourcemanager.Policy, error) {
	return r.svc.Projects.GetIamPolicy(project, &cloudresourcemanager.GetIamPolicyRequest{}).Context(ctx).Do()
}

func (r *realIAMPolicyClient) SetIAMPolicy(ctx context.Context, project string, policy *cloudresourcemanager.Policy) error {
	_, err := r.svc.Projects.SetIamPolicy(project, &cloudresourcemanager.SetIamPolicyRequest{
		Policy: policy,
	}).Context(ctx).Do()
	return err
}

func (r *realIAMPolicyClient) Close() error {
	return nil // REST client, no Close needed
}

// realFirewallsClient wraps the GCP Compute firewalls client.
type realFirewallsClient struct {
	client *compute.FirewallsClient
}

func newRealFirewallsClient(ctx context.Context) (*realFirewallsClient, error) {
	c, err := compute.NewFirewallsRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	return &realFirewallsClient{client: c}, nil
}

func (r *realFirewallsClient) Get(ctx context.Context, req *computepb.GetFirewallRequest) (*computepb.Firewall, error) {
	return r.client.Get(ctx, req)
}

func (r *realFirewallsClient) Insert(ctx context.Context, req *computepb.InsertFirewallRequest) (firewallOperation, error) {
	op, err := r.client.Insert(ctx, req)
	if err != nil {
		return nil, err
	}
	return &realFirewallOp{op: op}, nil
}

func (r *realFirewallsClient) Patch(ctx context.Context, req *computepb.PatchFirewallRequest) (firewallOperation, error) {
	op, err := r.client.Patch(ctx, req)
	if err != nil {
		return nil, err
	}
	return &realFirewallOp{op: op}, nil
}

func (r *realFirewallsClient) Close() error {
	return r.client.Close()
}

type realFirewallOp struct {
	op *compute.Operation
}

func (r *realFirewallOp) Wait(ctx context.Context) error {
	return r.op.Wait(ctx)
}
