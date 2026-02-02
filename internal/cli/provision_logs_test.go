package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

func TestBuildLogCommand(t *testing.T) {
	tests := []struct {
		name   string
		follow bool
		tail   int
		want   string
	}{
		{
			name:   "no flags shows full log",
			follow: false,
			tail:   0,
			want:   "cat /var/log/cloudcoop/provision.log",
		},
		{
			name:   "follow only",
			follow: true,
			tail:   0,
			want:   "tail -f /var/log/cloudcoop/provision.log",
		},
		{
			name:   "tail only",
			follow: false,
			tail:   50,
			want:   "tail -n 50 /var/log/cloudcoop/provision.log",
		},
		{
			name:   "follow with tail",
			follow: true,
			tail:   100,
			want:   "tail -n 100 -f /var/log/cloudcoop/provision.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLogCommand(tt.follow, tt.tail)
			if got != tt.want {
				t.Errorf("buildLogCommand(%v, %d) = %q, want %q", tt.follow, tt.tail, got, tt.want)
			}
		})
	}
}

func TestRunProvisionLogs_VMNotFound(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusNotFound)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runProvisionLogs(cmd, nil)
	if err == nil {
		t.Error("expected error for VM not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want containing 'not found'", err)
	}
}

func TestRunProvisionLogs_VMNotRunning(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusStopped)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runProvisionLogs(cmd, nil)
	if err == nil {
		t.Error("expected error for VM not running")
	}
	if !strings.Contains(err.Error(), "must be running") {
		t.Errorf("error = %v, want containing 'must be running'", err)
	}
}
