package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

func TestRunStart(t *testing.T) {
	tests := []struct {
		name           string
		vmStatus       cloud.VMStatus
		startError     error
		wantErr        bool
		wantErrContain string
		wantOutput     string
	}{
		{
			name:       "starts stopped VM",
			vmStatus:   cloud.VMStatusStopped,
			wantErr:    false,
			wantOutput: "Starting VM",
		},
		{
			name:       "already running is idempotent",
			vmStatus:   cloud.VMStatusRunning,
			wantErr:    false,
			wantOutput: "already running",
		},
		{
			name:       "already starting is idempotent",
			vmStatus:   cloud.VMStatusStarting,
			wantErr:    false,
			wantOutput: "already starting",
		},
		{
			name:           "stopping returns error",
			vmStatus:       cloud.VMStatusStopping,
			wantErr:        true,
			wantErrContain: "currently stopping",
		},
		{
			name:           "not found returns error",
			vmStatus:       cloud.VMStatusNotFound,
			wantErr:        true,
			wantErrContain: "not found",
		},
		{
			name:           "unknown status returns error",
			vmStatus:       cloud.VMStatusUnknown,
			wantErr:        true,
			wantErrContain: "unexpected state",
		},
		{
			name:           "start API error is returned",
			vmStatus:       cloud.VMStatusStopped,
			startError:     errors.New("API quota exceeded"),
			wantErr:        true,
			wantErrContain: "start VM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			cfg := testConfig()
			mock := cloud.NewMockProvider().WithVMStatus(tt.vmStatus)
			if tt.startError != nil {
				mock.WithStartVMError(tt.startError)
			}
			cleanup := withMocks(cfg, mock)
			defer cleanup()

			// Capture output
			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)
			cmd.SetContext(context.Background())

			// Run command
			err := runStart(cmd, []string{})

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("runStart() expected error, got nil")
					return
				}
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("runStart() error = %v, want containing %q", err, tt.wantErrContain)
				}
			} else {
				if err != nil {
					t.Errorf("runStart() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRunStart_ProviderCalls(t *testing.T) {
	// Setup mocks
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusStopped)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runStart(cmd, []string{})
	if err != nil {
		t.Fatalf("runStart() error: %v", err)
	}

	// Verify provider calls
	calls := mock.GetCalls()
	var hasGetVMInfo, hasStartVM bool
	for _, call := range calls {
		if call.Method == "GetVMInfo" {
			hasGetVMInfo = true
		}
		if call.Method == "StartVM" {
			hasStartVM = true
		}
	}

	if !hasGetVMInfo {
		t.Error("runStart() did not call GetVMInfo")
	}
	if !hasStartVM {
		t.Error("runStart() did not call StartVM")
	}
}
