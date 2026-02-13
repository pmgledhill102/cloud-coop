// Package gcp implements the cloud.Provider interface for Google Cloud Platform.
package gcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/googleapi"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
	"github.com/cloud-coop/cloudcoop/internal/version"
)

// Metadata keys for cloudcoop-managed VMs.
const (
	metadataKeyVersion    = "cloudcoop-version"
	metadataKeyCreated    = "cloudcoop-created"
	metadataKeyConfigHash = "cloudcoop-config-hash"
)

// DirectSSHFirewallRuleName is the name of the direct SSH firewall rule.
const DirectSSHFirewallRuleName = "cloudcoop-allow-ssh"

// Provider implements cloud.Provider for GCP.
type Provider struct {
	project   string
	zone      string
	client    instancesClient
	firewalls firewallsClient
}

// New creates a new GCP provider.
// It uses Application Default Credentials (ADC) for authentication.
func New(ctx context.Context, project, zone string) (*Provider, error) {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create compute client: %w", err)
	}

	fwClient, err := compute.NewFirewallsRESTClient(ctx)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("create firewalls client: %w", err)
	}

	return &Provider{
		project:   project,
		zone:      zone,
		client:    &realInstancesClient{client: client},
		firewalls: &realFirewallsClient{client: fwClient},
	}, nil
}

// newWithClient creates a provider with custom clients (for testing).
func newWithClient(project, zone string, client instancesClient) *Provider {
	return &Provider{
		project: project,
		zone:    zone,
		client:  client,
	}
}

// newWithClients creates a provider with all custom clients (for testing).
func newWithClients(project, zone string, client instancesClient, fw firewallsClient) *Provider {
	return &Provider{
		project:   project,
		zone:      zone,
		client:    client,
		firewalls: fw,
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "gcp"
}

// GetVMInfo retrieves information about a VM.
func (p *Provider) GetVMInfo(ctx context.Context, name string) (*cloud.VMInfo, error) {
	instance, err := p.client.Get(ctx, &computepb.GetInstanceRequest{
		Project:  p.project,
		Zone:     p.zone,
		Instance: name,
	})
	if err != nil {
		// Check if it's a "not found" error
		if isNotFoundError(err) {
			return &cloud.VMInfo{
				Name:   name,
				Status: cloud.VMStatusNotFound,
			}, nil
		}
		return nil, fmt.Errorf("get instance %s: %w", name, err)
	}

	info := &cloud.VMInfo{
		Name:        name,
		Status:      mapStatus(instance.GetStatus()),
		Zone:        extractZoneName(instance.GetZone()),
		MachineType: extractMachineTypeName(instance.GetMachineType()),
	}

	// Extract IP addresses from network interfaces
	if len(instance.NetworkInterfaces) > 0 {
		ni := instance.NetworkInterfaces[0]
		info.InternalIP = ni.GetNetworkIP()
		if len(ni.AccessConfigs) > 0 {
			info.ExternalIP = ni.AccessConfigs[0].GetNatIP()
		}
	}

	// Extract cloudcoop metadata if present
	if metadata := instance.GetMetadata(); metadata != nil {
		for _, item := range metadata.GetItems() {
			switch item.GetKey() {
			case metadataKeyVersion:
				info.CloudcoopVersion = item.GetValue()
			case metadataKeyCreated:
				info.CloudcoopCreated = item.GetValue()
			case metadataKeyConfigHash:
				info.CloudcoopConfigHash = item.GetValue()
			}
		}
	}

	return info, nil
}

// StartVM starts a stopped VM.
func (p *Provider) StartVM(ctx context.Context, name string) error {
	op, err := p.client.Start(ctx, &computepb.StartInstanceRequest{
		Project:  p.project,
		Zone:     p.zone,
		Instance: name,
	})
	if err != nil {
		return fmt.Errorf("start instance %s: %w", name, err)
	}

	// Wait for operation to complete
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("wait for start %s: %w", name, err)
	}

	return nil
}

// StopVM stops a running VM.
func (p *Provider) StopVM(ctx context.Context, name string) error {
	op, err := p.client.Stop(ctx, &computepb.StopInstanceRequest{
		Project:  p.project,
		Zone:     p.zone,
		Instance: name,
	})
	if err != nil {
		return fmt.Errorf("stop instance %s: %w", name, err)
	}

	// Wait for operation to complete
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("wait for stop %s: %w", name, err)
	}

	return nil
}

// CreateVM creates a new VM with the given configuration.
func (p *Provider) CreateVM(ctx context.Context, config cloud.VMCreateConfig) error {
	// Require service account for security (see docs/SECURITY.md)
	if config.ServiceAccount == "" {
		return fmt.Errorf("service_account is required in config for security; see docs/SETUP-FLOW.md")
	}

	// Build the machine type URL
	machineTypeURL := fmt.Sprintf("zones/%s/machineTypes/%s", p.zone, config.MachineType)

	// Select disk type based on machine type
	// c4a (Axion/ARM64) requires hyperdisk-balanced, others use pd-balanced
	diskType := "pd-balanced"
	if strings.HasPrefix(config.MachineType, "c4a-") {
		diskType = "hyperdisk-balanced"
	}

	// Build the instance specification
	instance := &computepb.Instance{
		Name:        &config.Name,
		MachineType: &machineTypeURL,
		Disks: []*computepb.AttachedDisk{
			{
				Boot:       ptr(true),
				AutoDelete: ptr(true), // Per ADR-0003: auto-delete only triggers on DELETE, not STOP
				InitializeParams: &computepb.AttachedDiskInitializeParams{
					SourceImage: &config.Image,
					DiskSizeGb:  &config.DiskSizeGB,
					DiskType:    ptr(fmt.Sprintf("zones/%s/diskTypes/%s", p.zone, diskType)),
				},
			},
		},
		NetworkInterfaces: []*computepb.NetworkInterface{
			{
				Network: ptr(fmt.Sprintf("global/networks/%s", config.Network)),
				AccessConfigs: []*computepb.AccessConfig{
					{
						Name: ptr("External NAT"),
						Type: ptr("ONE_TO_ONE_NAT"),
					},
				},
			},
		},
	}

	// Set subnet if specified (required for custom-mode VPC networks)
	if config.Subnet != "" {
		region := regionFromZone(p.zone)
		instance.NetworkInterfaces[0].Subnetwork = ptr(fmt.Sprintf("regions/%s/subnetworks/%s", region, config.Subnet))
	}

	// Configure spot instance with STOP on preemption (per ADR-0003)
	if config.Spot {
		instance.Scheduling = &computepb.Scheduling{
			ProvisioningModel:         ptr("SPOT"),
			InstanceTerminationAction: ptr("STOP"),
		}
	}

	// Set maxRunDuration to auto-stop the VM after N minutes (cost safety net)
	if config.MaxUptimeMinutes > 0 {
		if instance.Scheduling == nil {
			instance.Scheduling = &computepb.Scheduling{}
		}
		seconds := int64(config.MaxUptimeMinutes) * 60
		instance.Scheduling.MaxRunDuration = &computepb.Duration{Seconds: &seconds}
		if instance.Scheduling.InstanceTerminationAction == nil {
			instance.Scheduling.InstanceTerminationAction = ptr("STOP")
		}
	}

	// Add network tags if specified
	if len(config.Tags) > 0 {
		instance.Tags = &computepb.Tags{
			Items: config.Tags,
		}
	}

	// Attach service account with cloud-platform scope
	// This provides OAuth tokens for the configured service account
	instance.ServiceAccounts = []*computepb.ServiceAccount{{
		Email:  ptr(config.ServiceAccount),
		Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
	}}

	// Build startup script for VM configuration
	var startupScript string

	// Always create xterm-ghostty symlink for Ghostty terminal compatibility
	// Ubuntu 25.04 has 'ghostty' terminfo but Ghostty uses TERM=xterm-ghostty
	startupScript = `#!/bin/bash
set -e

# Create symlink for Ghostty terminal compatibility
# Ubuntu has terminfo for 'ghostty' but Ghostty sets TERM=xterm-ghostty
if [ -f /usr/share/terminfo/g/ghostty ] && [ ! -e /usr/share/terminfo/x/xterm-ghostty ]; then
    ln -sf /usr/share/terminfo/g/ghostty /usr/share/terminfo/x/xterm-ghostty
fi
`

	// Add SSH port configuration if non-standard
	if config.SSHPort > 0 && config.SSHPort != 22 {
		startupScript += fmt.Sprintf(`
# Configure SSH to listen on port %d
SSH_PORT=%d

# Wait for system to be ready
sleep 5

# Ubuntu 22.10+ uses systemd socket activation for SSH.
# Override the socket to listen on the custom port.
mkdir -p /etc/systemd/system/ssh.socket.d
cat > /etc/systemd/system/ssh.socket.d/port.conf << EOF
[Socket]
ListenStream=
ListenStream=0.0.0.0:${SSH_PORT}
ListenStream=[::]:${SSH_PORT}
EOF

# Also update sshd_config (covers non-socket-activated systems)
sed -i '/^#*Port /d' /etc/ssh/sshd_config
echo "Port ${SSH_PORT}" >> /etc/ssh/sshd_config

# Reload systemd and restart SSH.
# On socket-activated systems (Ubuntu 22.10+), only restart the socket.
# systemd auto-starts ssh.service when a connection arrives.
# Starting ssh.service directly would fail because sshd tries to bind
# the port that ssh.socket already holds.
systemctl daemon-reload
systemctl stop ssh.service 2>/dev/null || true
systemctl restart ssh.socket
`, config.SSHPort, config.SSHPort)
	}

	// Fetch and append provisioning script if URL is provided
	if config.ProvisionScriptURL != "" {
		provisionScript, err := provisioning.FetchScript(ctx, config.ProvisionScriptURL)
		if err != nil {
			return fmt.Errorf("fetch provisioning script: %w", err)
		}
		// Strip the shebang since we're appending to an existing script
		startupScript += "\n# Provisioning script from: " + config.ProvisionScriptURL + "\n"
		startupScript += provisioning.StripShebang(provisionScript)
	}

	// Build cloudcoop metadata for VM identification and upgrade detection
	configHash := version.ConfigHash(fmt.Sprintf(
		"%s|%s|%d|%s|%v|%s|%v|%d|%s|%s",
		config.Name, config.MachineType, config.DiskSizeGB, config.Image,
		config.Spot, config.Network, config.Tags, config.SSHPort,
		config.ServiceAccount, config.ProvisionScriptURL,
	))

	metadataItems := []*computepb.Items{
		{
			Key:   ptr("startup-script"),
			Value: ptr(startupScript),
		},
		{
			Key:   ptr(metadataKeyVersion),
			Value: ptr(version.Short()),
		},
		{
			Key:   ptr(metadataKeyCreated),
			Value: ptr(version.CreatedTimestamp()),
		},
		{
			Key:   ptr(metadataKeyConfigHash),
			Value: ptr(configHash),
		},
	}

	// Include SSH public key in metadata so the GCP guest agent provisions
	// it into ~/.ssh/authorized_keys on the VM.
	if config.SSHPublicKey != "" && config.SSHUser != "" {
		sshKeyEntry := fmt.Sprintf("%s:%s", config.SSHUser, config.SSHPublicKey)
		metadataItems = append(metadataItems, &computepb.Items{
			Key:   ptr("ssh-keys"),
			Value: ptr(sshKeyEntry),
		})
	}

	instance.Metadata = &computepb.Metadata{
		Items: metadataItems,
	}

	op, err := p.client.Insert(ctx, &computepb.InsertInstanceRequest{
		Project:          p.project,
		Zone:             p.zone,
		InstanceResource: instance,
	})
	if err != nil {
		return fmt.Errorf("create instance %s: %w", config.Name, err)
	}

	// Wait for operation to complete
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("wait for create %s: %w", config.Name, err)
	}

	return nil
}

// DeleteVM deletes a VM by name.
// The boot disk is created with AutoDelete=true (per ADR-0003), so GCP automatically
// deletes the disk when the instance is deleted. Auto-delete only triggers on DELETE,
// not STOP, so disks are preserved across preemption and manual stops.
func (p *Provider) DeleteVM(ctx context.Context, name string) error {
	op, err := p.client.Delete(ctx, &computepb.DeleteInstanceRequest{
		Project:  p.project,
		Zone:     p.zone,
		Instance: name,
	})
	if err != nil {
		return fmt.Errorf("delete instance %s: %w", name, err)
	}

	// Wait for instance deletion to complete (disk is auto-deleted)
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("wait for delete %s: %w", name, err)
	}

	return nil
}

// EnsureSSHKeyOnVM ensures the given SSH public key is present in the VM's
// instance metadata (ssh-keys). The GCP guest agent reads this metadata and
// provisions the key into ~/.ssh/authorized_keys on the VM.
// The operation is idempotent: if the key is already present, it is a no-op.
func (p *Provider) EnsureSSHKeyOnVM(ctx context.Context, name, user, publicKey string) error {
	instance, err := p.client.Get(ctx, &computepb.GetInstanceRequest{
		Project:  p.project,
		Zone:     p.zone,
		Instance: name,
	})
	if err != nil {
		return fmt.Errorf("get instance %s: %w", name, err)
	}

	metadata := instance.GetMetadata()
	fingerprint := metadata.GetFingerprint()

	// Build the key entry in GCP's format: "user:ssh-key-content"
	newEntry := fmt.Sprintf("%s:%s", user, publicKey)

	// Find existing ssh-keys metadata item
	var existingValue string
	existingIdx := -1
	for i, item := range metadata.GetItems() {
		if item.GetKey() == "ssh-keys" {
			existingValue = item.GetValue()
			existingIdx = i
			break
		}
	}

	// Check if this key is already present (match the public key content)
	if existingValue != "" && strings.Contains(existingValue, publicKey) {
		return nil // Already present
	}

	// Build the new ssh-keys value
	var newValue string
	if existingValue == "" {
		newValue = newEntry
	} else {
		newValue = existingValue + "\n" + newEntry
	}

	// Build the updated metadata items list
	items := make([]*computepb.Items, len(metadata.GetItems()))
	copy(items, metadata.GetItems())

	if existingIdx >= 0 {
		items[existingIdx] = &computepb.Items{
			Key:   ptr("ssh-keys"),
			Value: ptr(newValue),
		}
	} else {
		items = append(items, &computepb.Items{
			Key:   ptr("ssh-keys"),
			Value: ptr(newValue),
		})
	}

	op, err := p.client.SetMetadata(ctx, &computepb.SetMetadataInstanceRequest{
		Project:  p.project,
		Zone:     p.zone,
		Instance: name,
		MetadataResource: &computepb.Metadata{
			Fingerprint: ptr(fingerprint),
			Items:       items,
		},
	})
	if err != nil {
		return fmt.Errorf("set metadata on %s: %w", name, err)
	}
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("wait for metadata update on %s: %w", name, err)
	}

	return nil
}

// EnsureFirewallAllowsSSH checks/creates/updates a firewall rule to allow
// SSH from the given source IP on the given port.
func (p *Provider) EnsureFirewallAllowsSSH(ctx context.Context, cfg cloud.FirewallConfig) (bool, error) {
	if p.firewalls == nil {
		return false, fmt.Errorf("firewalls client not initialised")
	}

	wantRange := cfg.SourceIP + "/32"
	wantPort := fmt.Sprintf("%d", cfg.Port)

	// Check if the rule already exists
	rule, err := p.firewalls.Get(ctx, &computepb.GetFirewallRequest{
		Project:  p.project,
		Firewall: DirectSSHFirewallRuleName,
	})
	if err != nil {
		if !isNotFoundError(err) {
			return false, fmt.Errorf("get firewall rule: %w", err)
		}

		// Rule doesn't exist — create it
		newRule := &computepb.Firewall{
			Name:         ptr(DirectSSHFirewallRuleName),
			Description:  ptr("Allow direct SSH from workstation IP for cloudcoop"),
			Network:      ptr(fmt.Sprintf("global/networks/%s", cfg.Network)),
			Direction:    ptr("INGRESS"),
			Priority:     ptr(int32(1000)),
			SourceRanges: []string{wantRange},
			Allowed: []*computepb.Allowed{
				{
					IPProtocol: ptr("tcp"),
					Ports:      []string{wantPort},
				},
			},
		}

		op, insertErr := p.firewalls.Insert(ctx, &computepb.InsertFirewallRequest{
			Project:          p.project,
			FirewallResource: newRule,
		})
		if insertErr != nil {
			return false, fmt.Errorf("create direct SSH firewall rule: %w", insertErr)
		}
		if waitErr := op.Wait(ctx); waitErr != nil {
			return false, fmt.Errorf("wait for firewall rule: %w", waitErr)
		}
		return true, nil
	}

	// Rule exists — check if it matches
	currentRanges := rule.GetSourceRanges()
	currentPort := ""
	if allowed := rule.GetAllowed(); len(allowed) > 0 {
		if ports := allowed[0].GetPorts(); len(ports) > 0 {
			currentPort = ports[0]
		}
	}

	if len(currentRanges) == 1 && currentRanges[0] == wantRange && currentPort == wantPort {
		return false, nil // Already correct
	}

	// Rule exists but needs updating
	op, err := p.firewalls.Patch(ctx, &computepb.PatchFirewallRequest{
		Project:  p.project,
		Firewall: DirectSSHFirewallRuleName,
		FirewallResource: &computepb.Firewall{
			SourceRanges: []string{wantRange},
			Allowed: []*computepb.Allowed{
				{
					IPProtocol: ptr("tcp"),
					Ports:      []string{wantPort},
				},
			},
		},
	})
	if err != nil {
		return false, fmt.Errorf("patch direct SSH firewall rule: %w", err)
	}
	if err := op.Wait(ctx); err != nil {
		return false, fmt.Errorf("wait for firewall patch: %w", err)
	}
	return true, nil
}

// ptr returns a pointer to the given value.
func ptr[T any](v T) *T { return &v }

// Close closes the provider's clients.
func (p *Provider) Close() error {
	var errs []error
	if p.client != nil {
		errs = append(errs, p.client.Close())
	}
	if p.firewalls != nil {
		errs = append(errs, p.firewalls.Close())
	}
	return errors.Join(errs...)
}

// mapStatus converts GCP instance status to cloud.VMStatus.
func mapStatus(status string) cloud.VMStatus {
	// GCP status values: https://cloud.google.com/compute/docs/instances/instance-life-cycle
	switch status {
	case "RUNNING":
		return cloud.VMStatusRunning
	case "TERMINATED", "STOPPED":
		return cloud.VMStatusStopped
	case "STOPPING", "SUSPENDING":
		return cloud.VMStatusStopping
	case "STAGING", "PROVISIONING":
		return cloud.VMStatusStarting
	default:
		return cloud.VMStatusUnknown
	}
}

// regionFromZone derives the region from a zone by stripping the trailing component.
// e.g., "us-central1-a" -> "us-central1"
func regionFromZone(zone string) string {
	if i := strings.LastIndex(zone, "-"); i >= 0 {
		return zone[:i]
	}
	return zone
}

// extractZoneName extracts the zone name from a full zone URL.
// e.g., "projects/my-project/zones/us-central1-a" -> "us-central1-a"
func extractZoneName(zoneURL string) string {
	parts := strings.Split(zoneURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return zoneURL
}

// extractMachineTypeName extracts the machine type name from a full URL.
// e.g., "projects/my-project/zones/us-central1-a/machineTypes/c4a-highcpu-4" -> "c4a-highcpu-4"
func extractMachineTypeName(machineTypeURL string) string {
	parts := strings.Split(machineTypeURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return machineTypeURL
}

// isNotFoundError checks if the error is a "not found" error.
func isNotFoundError(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == 404
	}
	return false
}
