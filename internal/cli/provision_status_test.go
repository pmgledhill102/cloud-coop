package cli

import (
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/provisioning"
)

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
