package cli

import (
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name   string
		status cloud.VMStatus
		want   string
	}{
		{
			name:   "running status",
			status: cloud.VMStatusRunning,
			want:   "● running",
		},
		{
			name:   "stopped status",
			status: cloud.VMStatusStopped,
			want:   "○ stopped",
		},
		{
			name:   "starting status",
			status: cloud.VMStatusStarting,
			want:   "◐ starting...",
		},
		{
			name:   "stopping status",
			status: cloud.VMStatusStopping,
			want:   "◑ stopping...",
		},
		{
			name:   "unknown status falls through",
			status: cloud.VMStatus("unknown"),
			want:   "unknown",
		},
		{
			name:   "not found status falls through",
			status: cloud.VMStatusNotFound,
			want:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatStatus(tt.status); got != tt.want {
				t.Errorf("formatStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
