// Package setup provides automated cloud project provisioning for cloudcoop.
// It defines a provider interface for cloud-agnostic setup operations and
// orchestrates prerequisite checks, API enablement, service account creation,
// IAM bindings, and firewall rules.
package setup

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
)

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

// NetworkInfo describes a VPC network.
type NetworkInfo struct {
	Name string
}

// SubnetInfo describes a VPC subnet.
type SubnetInfo struct {
	Name string
}

// SetupProvider defines the interface for cloud setup operations.
// GCP implements it; AWS/Azure can implement it later.
type SetupProvider interface {
	// ListProjects returns available cloud projects.
	ListProjects(ctx context.Context) ([]ProjectInfo, error)

	// ListNetworks returns available VPC networks in the project.
	ListNetworks(ctx context.Context, project string) ([]NetworkInfo, error)

	// ListSubnets returns available subnets in the given network and region.
	ListSubnets(ctx context.Context, project, region, network string) ([]SubnetInfo, error)

	// CheckAPIs checks which of the given APIs are enabled in the project.
	CheckAPIs(ctx context.Context, project string, apis []string) ([]APIStatus, error)

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
	// The sshPort parameter specifies which TCP port to allow (typically 22 or 2222).
	CreateIAPFirewallRule(ctx context.Context, project, network string, sshPort int) error

	// GetFirewallRulePort returns the allowed TCP port from the named firewall rule.
	GetFirewallRulePort(ctx context.Context, project, name string) (int, error)

	// UpdateIAPFirewallRule updates the allowed port on the IAP firewall rule.
	UpdateIAPFirewallRule(ctx context.Context, project string, sshPort int) error

	// CreateDirectSSHFirewallRule creates a firewall rule allowing SSH from a specific IP.
	CreateDirectSSHFirewallRule(ctx context.Context, project, network, sourceIP string, sshPort int) error

	// GetFirewallRuleSourceIP returns the first source range and port from the named rule.
	GetFirewallRuleSourceIP(ctx context.Context, project, name string) (string, int, error)

	// UpdateDirectSSHFirewallRule updates the source IP and port on the direct SSH rule.
	UpdateDirectSSHFirewallRule(ctx context.Context, project, sourceIP string, sshPort int) error

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

// MergedAPIs returns RequiredAPIs plus any extra APIs, deduplicated.
func MergedAPIs(extraAPIs []string) []string {
	return mergeUnique(RequiredAPIs, extraAPIs)
}

// MergedIAMRoles returns RequiredIAMRoles plus any extra roles, deduplicated.
func MergedIAMRoles(extraRoles []string) []string {
	return mergeUnique(RequiredIAMRoles, extraRoles)
}

func mergeUnique(base, extra []string) []string {
	seen := make(map[string]bool, len(base))
	result := make([]string, 0, len(base)+len(extra))
	for _, s := range base {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range extra {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// ServiceAccountDisplayName is the display name for the service account.
const ServiceAccountDisplayName = "cloudcoop VM service account"

// saNameRegexp matches characters NOT allowed in GCP service account IDs.
var saNameRegexp = regexp.MustCompile(`[^a-z0-9-]`)

// ServiceAccountNameForDir derives a service account name from a directory path.
// The name is based on the directory basename, prefixed with "cc-" and sanitised
// for GCP constraints (6-30 chars, lowercase alphanumeric and hyphens).
func ServiceAccountNameForDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "cc-cloudcoop"
	}
	base := filepath.Base(abs)
	name := "cc-" + strings.ToLower(base)

	// Replace invalid characters with hyphens
	name = saNameRegexp.ReplaceAllString(name, "-")

	// Collapse consecutive hyphens
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// Trim trailing hyphens
	name = strings.TrimRight(name, "-")

	// Truncate to 30 chars (GCP max)
	if len(name) > 30 {
		name = strings.TrimRight(name[:30], "-")
	}

	// Ensure minimum length of 6 (GCP min)
	if len(name) < 6 {
		name = name + "-vm"
	}

	return name
}

// IAPFirewallRuleName is the name of the IAP SSH firewall rule.
const IAPFirewallRuleName = "cloudcoop-allow-iap-ssh"

// DirectSSHFirewallRuleName is the name of the direct SSH firewall rule.
// This rule allows SSH from the user's workstation IP address.
const DirectSSHFirewallRuleName = "cloudcoop-allow-ssh"

// ServiceAccountEmail returns the full email for a service account.
func ServiceAccountEmail(project, name string) string {
	return name + "@" + project + ".iam.gserviceaccount.com"
}
