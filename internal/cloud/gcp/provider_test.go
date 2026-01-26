package gcp

import (
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

func TestMapStatus(t *testing.T) {
	tests := []struct {
		gcpStatus string
		want      cloud.VMStatus
	}{
		{"RUNNING", cloud.VMStatusRunning},
		{"TERMINATED", cloud.VMStatusStopped},
		{"STOPPED", cloud.VMStatusStopped},
		{"STOPPING", cloud.VMStatusStopping},
		{"SUSPENDING", cloud.VMStatusStopping},
		{"STAGING", cloud.VMStatusStarting},
		{"PROVISIONING", cloud.VMStatusStarting},
		{"WEIRD_STATUS", cloud.VMStatusUnknown},
		{"", cloud.VMStatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.gcpStatus, func(t *testing.T) {
			if got := mapStatus(tt.gcpStatus); got != tt.want {
				t.Errorf("mapStatus(%q) = %v, want %v", tt.gcpStatus, got, tt.want)
			}
		})
	}
}

func TestExtractZoneName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"projects/my-project/zones/us-central1-a", "us-central1-a"},
		{"projects/my-project/zones/europe-north1-b", "europe-north1-b"},
		{"us-central1-a", "us-central1-a"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := extractZoneName(tt.input); got != tt.want {
				t.Errorf("extractZoneName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractMachineTypeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"projects/my-project/zones/us-central1-a/machineTypes/c4a-highcpu-4", "c4a-highcpu-4"},
		{"projects/my-project/zones/us-central1-a/machineTypes/n1-standard-1", "n1-standard-1"},
		{"c4a-highcpu-4", "c4a-highcpu-4"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := extractMachineTypeName(tt.input); got != tt.want {
				t.Errorf("extractMachineTypeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestProviderInterface ensures Provider implements cloud.Provider.
func TestProviderInterface(t *testing.T) {
	var _ cloud.Provider = (*Provider)(nil)
}
