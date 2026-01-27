package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
)

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
