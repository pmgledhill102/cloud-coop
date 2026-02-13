package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/ops"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
)

func TestRunStatus(t *testing.T) {
	tests := []struct {
		name       string
		vmStatus   cloud.VMStatus
		vmInfoErr  error
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:     "shows running VM",
			vmStatus: cloud.VMStatusRunning,
			wantErr:  false,
		},
		{
			name:     "shows stopped VM",
			vmStatus: cloud.VMStatusStopped,
			wantErr:  false,
		},
		{
			name:       "handles GetVMInfo error",
			vmInfoErr:  errors.New("API error"),
			wantErr:    true,
			wantErrMsg: "get VM status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			mock := cloud.NewMockProvider()
			if tt.vmInfoErr != nil {
				mock.WithVMInfoError(tt.vmInfoErr)
			} else {
				mock.WithVMStatus(tt.vmStatus)
			}
			cleanup := withMocks(cfg, mock)
			defer cleanup()

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())

			err := runStatus(cmd, []string{})

			if tt.wantErr {
				if err == nil {
					t.Error("runStatus() expected error, got nil")
				} else if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("runStatus() error = %v, want containing %q", err, tt.wantErrMsg)
				}
			} else if err != nil {
				t.Errorf("runStatus() unexpected error: %v", err)
			}
		})
	}
}

func TestRunStatus_ConfigError(t *testing.T) {
	// Test config loading error
	cleanup := withMockConfig(nil, os.ErrNotExist)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	// Should handle gracefully (returns nil after printing message)
	err := runStatus(cmd, []string{})
	if err != nil {
		t.Errorf("runStatus() with config error should handle gracefully, got: %v", err)
	}
}

func TestPrintStatus(t *testing.T) {
	cfg := &config.Config{
		Cloud: config.CloudConfig{
			Provider: "gcp",
			GCP: config.GCPConfig{
				Project:        "test-project",
				Zone:           "us-central1-a",
				ServiceAccount: "cloudcoop-vm@test-project.iam.gserviceaccount.com",
			},
		},
		VM: config.VMConfig{Name: "test-vm"},
	}

	tests := []struct {
		name         string
		vmInfo       *cloud.VMInfo
		wantContains []string
	}{
		{
			name: "running VM with IPs",
			vmInfo: &cloud.VMInfo{
				Name:        "test-vm",
				Status:      cloud.VMStatusRunning,
				ExternalIP:  "203.0.113.1",
				InternalIP:  "10.0.0.1",
				MachineType: "c4a-highcpu-4",
			},
			wantContains: []string{"test-vm", "running", "203.0.113.1"},
		},
		{
			name: "stopped VM",
			vmInfo: &cloud.VMInfo{
				Name:   "test-vm",
				Status: cloud.VMStatusStopped,
			},
			wantContains: []string{"test-vm", "stopped"},
		},
		{
			name: "VM not found",
			vmInfo: &cloud.VMInfo{
				Name:   "test-vm",
				Status: cloud.VMStatusNotFound,
			},
			wantContains: []string{"not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printStatus(cfg, tt.vmInfo)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("printStatus() missing %q in output:\n%s", want, output)
				}
			}
		})
	}
}

func TestCreateProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{
			name:     "unsupported provider returns error",
			provider: "azure",
			wantErr:  true,
		},
		{
			name:     "unknown provider returns error",
			provider: "unknown",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Cloud: config.CloudConfig{
					Provider: tt.provider,
				},
			}

			_, _, err := ops.NewProvider(context.Background(), cfg)
			if tt.wantErr && err == nil {
				t.Error("ops.NewProvider() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ops.NewProvider() unexpected error: %v", err)
			}
		})
	}
}

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
