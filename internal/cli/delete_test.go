package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

func TestRunDelete(t *testing.T) {
	tests := []struct {
		name           string
		vmStatus       cloud.VMStatus
		deleteError    error
		wantErr        bool
		wantErrContain string
	}{
		{
			name:     "stopped VM with force deletes successfully",
			vmStatus: cloud.VMStatusStopped,
			wantErr:  false,
		},
		{
			name:           "running VM returns error",
			vmStatus:       cloud.VMStatusRunning,
			wantErr:        true,
			wantErrContain: "stop it first",
		},
		{
			name:     "not found VM prints message and returns nil",
			vmStatus: cloud.VMStatusNotFound,
			wantErr:  false,
		},
		{
			name:           "unknown status returns error",
			vmStatus:       cloud.VMStatusUnknown,
			wantErr:        true,
			wantErrContain: "cannot delete",
		},
		{
			name:           "delete API failure returns error",
			vmStatus:       cloud.VMStatusStopped,
			deleteError:    errors.New("API quota exceeded"),
			wantErr:        true,
			wantErrContain: "delete VM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			mock := cloud.NewMockProvider().WithVMStatus(tt.vmStatus)
			if tt.deleteError != nil {
				mock.WithDeleteVMError(tt.deleteError)
			}
			cleanup := withMocks(cfg, mock)
			defer cleanup()

			// Force mode to skip stdin confirmation
			origForce := deleteForce
			deleteForce = true
			defer func() { deleteForce = origForce }()

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())

			err := runDelete(cmd, []string{})

			if tt.wantErr {
				if err == nil {
					t.Errorf("runDelete() expected error, got nil")
					return
				}
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("runDelete() error = %v, want containing %q", err, tt.wantErrContain)
				}
			} else {
				if err != nil {
					t.Errorf("runDelete() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRunDelete_ProviderCalls(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusStopped)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	origForce := deleteForce
	deleteForce = true
	defer func() { deleteForce = origForce }()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runDelete(cmd, []string{})
	if err != nil {
		t.Fatalf("runDelete() error: %v", err)
	}

	calls := mock.GetCalls()
	var hasGetVMInfo, hasDeleteVM bool
	for _, call := range calls {
		if call.Method == "GetVMInfo" {
			hasGetVMInfo = true
		}
		if call.Method == "DeleteVM" {
			hasDeleteVM = true
		}
	}

	if !hasGetVMInfo {
		t.Error("runDelete() did not call GetVMInfo")
	}
	if !hasDeleteVM {
		t.Error("runDelete() did not call DeleteVM")
	}
}
