package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/provisioning"
)

var errProvisionTest = errors.New("test error")

func TestFormatProvisionStatus(t *testing.T) {
	tests := []struct {
		name   string
		status provisioning.ProvisionStatus
		want   string
	}{
		{
			name:   "pending status",
			status: provisioning.StatusPending,
			want:   "○ pending",
		},
		{
			name:   "running status",
			status: provisioning.StatusRunning,
			want:   "◐ running",
		},
		{
			name:   "completed status",
			status: provisioning.StatusCompleted,
			want:   "● completed",
		},
		{
			name:   "failed status",
			status: provisioning.StatusFailed,
			want:   "✗ failed",
		},
		{
			name:   "unknown status",
			status: provisioning.StatusUnknown,
			want:   "? unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &provisioning.StatusInfo{Status: tt.status}
			got := formatProvisionStatus(info)
			if got != tt.want {
				t.Errorf("formatProvisionStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunProvisionStatus_VMNotFound(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusNotFound)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	// Should handle gracefully (VM not found)
	err := runProvisionStatus(cmd, []string{})
	if err != nil {
		t.Errorf("runProvisionStatus() with VM not found should return nil, got: %v", err)
	}
}

func TestRunProvisionStatus_VMStopped(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusStopped)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	// Should handle gracefully (VM stopped, cannot check provisioning)
	err := runProvisionStatus(cmd, []string{})
	if err != nil {
		t.Errorf("runProvisionStatus() with VM stopped should return nil, got: %v", err)
	}
}

func TestRunProvisionStatus_VMStarting(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusStarting)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	// Should handle gracefully (VM starting, cannot check provisioning)
	err := runProvisionStatus(cmd, []string{})
	if err != nil {
		t.Errorf("runProvisionStatus() with VM starting should return nil, got: %v", err)
	}
}

func TestRunProvisionStatus_GetVMInfoError(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMInfoError(errProvisionTest)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runProvisionStatus(cmd, []string{})
	if err == nil {
		t.Error("runProvisionStatus() with GetVMInfo error should return error")
	}
}
