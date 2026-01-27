package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
)

func TestRunCreate(t *testing.T) {
	tests := []struct {
		name           string
		vmStatus       cloud.VMStatus
		createError    error
		wantErr        bool
		wantErrContain string
	}{
		{
			name:     "creates VM when not found",
			vmStatus: cloud.VMStatusNotFound,
			wantErr:  false,
		},
		{
			name:     "already exists running is idempotent",
			vmStatus: cloud.VMStatusRunning,
			wantErr:  false,
		},
		{
			name:     "already exists stopped is idempotent",
			vmStatus: cloud.VMStatusStopped,
			wantErr:  false,
		},
		{
			name:           "starting returns error",
			vmStatus:       cloud.VMStatusStarting,
			wantErr:        true,
			wantErrContain: "cannot create",
		},
		{
			name:           "stopping returns error",
			vmStatus:       cloud.VMStatusStopping,
			wantErr:        true,
			wantErrContain: "cannot create",
		},
		{
			name:           "create API error is returned",
			vmStatus:       cloud.VMStatusNotFound,
			createError:    errors.New("quota exceeded"),
			wantErr:        true,
			wantErrContain: "create VM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.VM.MachineSizes = map[string]string{
				"small":  "c4a-highcpu-4",
				"medium": "c4a-highcpu-8",
			}
			mock := cloud.NewMockProvider().WithVMStatus(tt.vmStatus)
			if tt.createError != nil {
				mock.WithCreateVMError(tt.createError)
			}
			cleanup := withMocks(cfg, mock)
			defer cleanup()

			// Reset the size flag for each test
			createSize = "small"

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())

			err := runCreate(cmd, []string{})

			if tt.wantErr {
				if err == nil {
					t.Error("runCreate() expected error, got nil")
					return
				}
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("runCreate() error = %v, want containing %q", err, tt.wantErrContain)
				}
			} else {
				if err != nil {
					t.Errorf("runCreate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRunCreate_InvalidSize(t *testing.T) {
	cfg := testConfig()
	cfg.VM.MachineSizes = map[string]string{
		"small": "c4a-highcpu-4",
	}
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusNotFound)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	createSize = "invalid-size"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runCreate(cmd, []string{})
	if err == nil {
		t.Error("runCreate() with invalid size should return error")
	}
	if !strings.Contains(err.Error(), "invalid size") {
		t.Errorf("runCreate() error should mention invalid size, got: %v", err)
	}
}

func TestRunCreate_PassesProvisionScriptURL(t *testing.T) {
	cfg := testConfig()
	cfg.VM.MachineSizes = map[string]string{
		"small": "c4a-highcpu-4",
	}
	cfg.Provisioning = config.ProvisioningConfig{
		ScriptURL: "https://example.com/provision.sh",
	}
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusNotFound)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	createSize = "small"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runCreate(cmd, []string{})
	if err != nil {
		t.Fatalf("runCreate() error: %v", err)
	}

	// Verify CreateVM was called with the provisioning URL
	calls := mock.GetCalls()
	var createCall *cloud.MockCall
	for i, call := range calls {
		if call.Method == "CreateVM" {
			createCall = &calls[i]
			break
		}
	}

	if createCall == nil {
		t.Fatal("CreateVM was not called")
	}

	createCfg, ok := createCall.Args[0].(cloud.VMCreateConfig)
	if !ok {
		t.Fatal("CreateVM arg is not VMCreateConfig")
	}

	if createCfg.ProvisionScriptURL != "https://example.com/provision.sh" {
		t.Errorf("ProvisionScriptURL = %q, want %q", createCfg.ProvisionScriptURL, "https://example.com/provision.sh")
	}
}

func TestRunCreate_PassesServiceAccount(t *testing.T) {
	cfg := testConfig()
	cfg.VM.MachineSizes = map[string]string{
		"small": "c4a-highcpu-4",
	}
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusNotFound)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	createSize = "small"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runCreate(cmd, []string{})
	if err != nil {
		t.Fatalf("runCreate() error: %v", err)
	}

	// Verify CreateVM was called with service account
	calls := mock.GetCalls()
	var createCall *cloud.MockCall
	for i, call := range calls {
		if call.Method == "CreateVM" {
			createCall = &calls[i]
			break
		}
	}

	if createCall == nil {
		t.Fatal("CreateVM was not called")
	}

	createCfg, ok := createCall.Args[0].(cloud.VMCreateConfig)
	if !ok {
		t.Fatal("CreateVM arg is not VMCreateConfig")
	}

	expectedSA := "cloudcoop-vm@test-project.iam.gserviceaccount.com"
	if createCfg.ServiceAccount != expectedSA {
		t.Errorf("ServiceAccount = %q, want %q", createCfg.ServiceAccount, expectedSA)
	}
}

func TestRunCreate_ProviderCalls(t *testing.T) {
	cfg := testConfig()
	cfg.VM.MachineSizes = map[string]string{
		"small": "c4a-highcpu-4",
	}
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusNotFound)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	createSize = "small"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runCreate(cmd, []string{})
	if err != nil {
		t.Fatalf("runCreate() error: %v", err)
	}

	// Verify provider calls
	calls := mock.GetCalls()
	var hasGetVMInfo, hasCreateVM bool
	for _, call := range calls {
		if call.Method == "GetVMInfo" {
			hasGetVMInfo = true
		}
		if call.Method == "CreateVM" {
			hasCreateVM = true
		}
	}

	if !hasGetVMInfo {
		t.Error("runCreate() did not call GetVMInfo")
	}
	if !hasCreateVM {
		t.Error("runCreate() did not call CreateVM")
	}
}
