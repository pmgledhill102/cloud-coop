// Package setup provides automated cloud project provisioning for cloudcoop.
// It defines a provider interface for cloud-agnostic setup operations and
// orchestrates prerequisite checks, API enablement, service account creation,
// IAM bindings, and firewall rules.
package setup

import "context"

// ProjectInfo describes a cloud project.
type ProjectInfo struct {
	ID   string
	Name string
}

// APIStatus describes whether a specific API is enabled.
type APIStatus struct {
	Name    string // e.g., "compute.googleapis.com"
	Enabled bool
}

// SetupProvider defines the interface for cloud setup operations.
// GCP implements it; AWS/Azure can implement it later.
type SetupProvider interface {
	// ListProjects returns available cloud projects.
	ListProjects(ctx context.Context) ([]ProjectInfo, error)

	// CheckAPIs checks which required APIs are enabled in the project.
	CheckAPIs(ctx context.Context, project string) ([]APIStatus, error)

	// EnableAPI enables a single API in the project.
	EnableAPI(ctx context.Context, project, api string) error

	// ServiceAccountExists checks if a service account exists.
	ServiceAccountExists(ctx context.Context, project, name string) (bool, error)

	// CreateServiceAccount creates a new service account and returns its email.
	CreateServiceAccount(ctx context.Context, project, name, displayName string) (string, error)

	// CheckIAMBinding checks if a specific IAM binding exists.
	CheckIAMBinding(ctx context.Context, project, member, role string) (bool, error)

	// GrantIAMRole adds an IAM role binding for the given member.
	GrantIAMRole(ctx context.Context, project, member, role string) error

	// FirewallRuleExists checks if the IAP firewall rule exists.
	FirewallRuleExists(ctx context.Context, project, name string) (bool, error)

	// CreateIAPFirewallRule creates a firewall rule allowing SSH from Google IAP.
	CreateIAPFirewallRule(ctx context.Context, project, network string) error

	// CheckADCCredentials verifies that Application Default Credentials are available.
	CheckADCCredentials(ctx context.Context) error

	// Close releases any resources held by the provider.
	Close() error
}

// RequiredAPIs is the list of GCP APIs that cloudcoop requires.
var RequiredAPIs = []string{
	"compute.googleapis.com",
	"iam.googleapis.com",
	"logging.googleapis.com",
	"monitoring.googleapis.com",
}

// RequiredIAMRoles is the list of IAM roles for the VM service account.
var RequiredIAMRoles = []string{
	"roles/logging.logWriter",
	"roles/monitoring.metricWriter",
}

// ServiceAccountName is the default name for the cloudcoop VM service account.
const ServiceAccountName = "cloudcoop-vm"

// ServiceAccountDisplayName is the display name for the service account.
const ServiceAccountDisplayName = "cloudcoop VM service account"

// IAPFirewallRuleName is the name of the IAP SSH firewall rule.
const IAPFirewallRuleName = "cloudcoop-allow-iap-ssh"

// ServiceAccountEmail returns the full email for a service account.
func ServiceAccountEmail(project, name string) string {
	return name + "@" + project + ".iam.gserviceaccount.com"
}
