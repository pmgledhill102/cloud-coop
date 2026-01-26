// Package cloud provides the interface for cloud provider operations.
package cloud

import (
	"context"
)

// VMStatus represents the status of a virtual machine.
type VMStatus string

const (
	// VMStatusRunning indicates the VM is running.
	VMStatusRunning VMStatus = "running"
	// VMStatusStopped indicates the VM is stopped.
	VMStatusStopped VMStatus = "stopped"
	// VMStatusStopping indicates the VM is stopping.
	VMStatusStopping VMStatus = "stopping"
	// VMStatusStarting indicates the VM is starting.
	VMStatusStarting VMStatus = "starting"
	// VMStatusNotFound indicates the VM does not exist.
	VMStatusNotFound VMStatus = "not_found"
	// VMStatusUnknown indicates an unknown or unexpected status.
	VMStatusUnknown VMStatus = "unknown"
)

// String returns the string representation of the status.
func (s VMStatus) String() string {
	return string(s)
}

// IsTerminal returns true if this is a stable (non-transitional) state.
func (s VMStatus) IsTerminal() bool {
	switch s {
	case VMStatusRunning, VMStatusStopped, VMStatusNotFound:
		return true
	default:
		return false
	}
}

// VMInfo contains information about a virtual machine.
type VMInfo struct {
	// Name is the VM instance name.
	Name string
	// Status is the current VM status.
	Status VMStatus
	// Zone is the availability zone (e.g., "us-central1-a").
	Zone string
	// MachineType is the instance type (e.g., "c4a-highcpu-4").
	MachineType string
	// ExternalIP is the public IP address, if any.
	ExternalIP string
	// InternalIP is the private IP address.
	InternalIP string
}

// VMCreateConfig contains configuration for creating a new VM.
type VMCreateConfig struct {
	// Name is the VM instance name.
	Name string
	// MachineType is the instance type (e.g., "c4a-highcpu-4").
	MachineType string
	// DiskSizeGB is the boot disk size in gigabytes.
	DiskSizeGB int64
	// Image is the boot disk image (e.g., "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64").
	Image string
	// Spot indicates whether to use spot/preemptible instances.
	Spot bool
	// Network is the VPC network name (e.g., "default").
	Network string
	// Tags are network tags for firewall rules.
	Tags []string
}

// Provider defines the interface for cloud provider operations.
type Provider interface {
	// Name returns the provider name (e.g., "gcp", "aws", "azure").
	Name() string

	// GetVMInfo retrieves information about a VM.
	// Returns a VMInfo with Status=VMStatusNotFound if the VM does not exist.
	GetVMInfo(ctx context.Context, name string) (*VMInfo, error)

	// StartVM starts a stopped VM.
	StartVM(ctx context.Context, name string) error

	// StopVM stops a running VM.
	StopVM(ctx context.Context, name string) error

	// CreateVM creates a new VM with the given configuration.
	CreateVM(ctx context.Context, config VMCreateConfig) error

	// DeleteVM deletes a VM by name.
	DeleteVM(ctx context.Context, name string) error
}
