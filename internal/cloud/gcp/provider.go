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
)

// Provider implements cloud.Provider for GCP.
type Provider struct {
	project string
	zone    string
	client  instancesClient
}

// New creates a new GCP provider.
// It uses Application Default Credentials (ADC) for authentication.
func New(ctx context.Context, project, zone string) (*Provider, error) {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create compute client: %w", err)
	}

	return &Provider{
		project: project,
		zone:    zone,
		client:  &realInstancesClient{client: client},
	}, nil
}

// newWithClient creates a provider with a custom client (for testing).
func newWithClient(project, zone string, client instancesClient) *Provider {
	return &Provider{
		project: project,
		zone:    zone,
		client:  client,
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

// Close closes the provider's clients.
func (p *Provider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
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
