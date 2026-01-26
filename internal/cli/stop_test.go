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

func TestRunStop(t *testing.T) {
	tests := []struct {
		name           string
		vmStatus       cloud.VMStatus
		stopError      error
		wantErr        bool
		wantErrContain string
		wantOutput     string
	}{
		{
			name:       "stops running VM",
			vmStatus:   cloud.VMStatusRunning,
			wantErr:    false,
			wantOutput: "Stopping VM",
		},
		{
			name:       "already stopped is idempotent",
			vmStatus:   cloud.VMStatusStopped,
			wantErr:    false,
			wantOutput: "already stopped",
		},
		{
			name:       "already stopping is idempotent",
			vmStatus:   cloud.VMStatusStopping,
			wantErr:    false,
			wantOutput: "already stopping",
		},
		{
			name:           "starting returns error",
			vmStatus:       cloud.VMStatusStarting,
			wantErr:        true,
			wantErrContain: "currently starting",
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
			name:           "stop API error is returned",
			vmStatus:       cloud.VMStatusRunning,
			stopError:      errors.New("permission denied"),
			wantErr:        true,
			wantErrContain: "stop VM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			cfg := testConfig()
			mock := cloud.NewMockProvider().WithVMStatus(tt.vmStatus)
			if tt.stopError != nil {
				mock.WithStopVMError(tt.stopError)
			}
			cleanup := withMocks(cfg, mock)
			defer cleanup()

			// Capture output
			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)
			cmd.SetContext(context.Background())

			// Run command
			err := runStop(cmd, []string{})

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("runStop() expected error, got nil")
					return
				}
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("runStop() error = %v, want containing %q", err, tt.wantErrContain)
				}
			} else {
				if err != nil {
					t.Errorf("runStop() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRunStop_ProviderCalls(t *testing.T) {
	// Setup mocks
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusRunning)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runStop(cmd, []string{})
	if err != nil {
		t.Fatalf("runStop() error: %v", err)
	}

	// Verify provider calls
	calls := mock.GetCalls()
	var hasGetVMInfo, hasStopVM bool
	for _, call := range calls {
		if call.Method == "GetVMInfo" {
			hasGetVMInfo = true
		}
		if call.Method == "StopVM" {
			hasStopVM = true
		}
	}

	if !hasGetVMInfo {
		t.Error("runStop() did not call GetVMInfo")
	}
	if !hasStopVM {
		t.Error("runStop() did not call StopVM")
	}
}
