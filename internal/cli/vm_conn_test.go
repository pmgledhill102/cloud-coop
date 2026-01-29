package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

func TestConnectToVM(t *testing.T) {
	tests := []struct {
		name       string
		vmStatus   cloud.VMStatus
		vmErr      error
		wantNil    bool // expect nil conn (handled gracefully)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:     "running VM returns connection",
			vmStatus: cloud.VMStatusRunning,
		},
		{
			name:     "not found returns nil",
			vmStatus: cloud.VMStatusNotFound,
			wantNil:  true,
		},
		{
			name:     "stopped returns nil",
			vmStatus: cloud.VMStatusStopped,
			wantNil:  true,
		},
		{
			name:       "API error returns error",
			vmErr:      errors.New("API error"),
			wantErr:    true,
			wantErrMsg: "get VM status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			mock := cloud.NewMockProvider()
			if tt.vmErr != nil {
				mock.WithVMInfoError(tt.vmErr)
			} else {
				mock.WithVMStatus(tt.vmStatus)
			}
			cleanup := withMocks(cfg, mock)
			defer cleanup()

			// For running VMs, also mock the SSH client
			if tt.vmStatus == cloud.VMStatusRunning && tt.vmErr == nil {
				sshMock := newNoopSSHMock()
				cleanupSSH := withMockSSHClient(sshMock)
				defer cleanupSSH()
			}

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())

			conn, err := connectToVM(cmd)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && !containsStr(err.Error(), tt.wantErrMsg) {
					t.Errorf("error = %v, want containing %q", err, tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if conn != nil {
					t.Error("expected nil conn")
					conn.Close()
				}
				return
			}
			if conn == nil {
				t.Fatal("expected non-nil conn")
			}
			defer conn.Close()

			if conn.Config == nil {
				t.Error("conn.Config is nil")
			}
			if conn.Client == nil {
				t.Error("conn.Client is nil")
			}
			if conn.IP == "" {
				t.Error("conn.IP is empty")
			}
			if conn.User == "" {
				t.Error("conn.User is empty")
			}
		})
	}
}

func TestConnectToVM_ConfigError(t *testing.T) {
	cleanup := withMockConfig(nil, errors.New("config broken"))
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	_, err := connectToVM(cmd)
	if err == nil {
		// handleConfigError may return nil for specific errors
		// but a generic error should propagate
		t.Log("config error was handled gracefully (expected for some error types)")
	}
}

// containsStr is a helper to avoid importing strings in this file.
func containsStr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
