//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestPhase0_CredentialValidation verifies we have valid GCP credentials and
// can access the configured project. This must pass before any other tests.
func TestPhase0_CredentialValidation(t *testing.T) {
	env.initProvider(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("project_access", func(t *testing.T) {
		// List instances to verify we have Compute API access
		// Using GetVMInfo with a name we know doesn't exist is a lightweight
		// way to verify credentials and API access
		info, err := env.provider.GetVMInfo(ctx, "cc-inttest-nonexistent-validation")
		if err != nil {
			t.Fatalf("Failed to access GCP Compute API: %v\n"+
				"Verify GOOGLE_APPLICATION_CREDENTIALS is set and the service account "+
				"has roles/compute.admin on project %s", err, env.projectID)
		}
		if info.Status != "not_found" {
			t.Fatalf("Expected not_found status for nonexistent VM, got: %s", info.Status)
		}
		t.Logf("Successfully authenticated to project %s in zone %s", env.projectID, env.zone)
	})

	t.Run("unique_vm_name", func(t *testing.T) {
		// Verify our generated VM name doesn't already exist (collision check)
		info, err := env.provider.GetVMInfo(ctx, env.vmName)
		if err != nil {
			t.Fatalf("Check VM name: %v", err)
		}
		if info.Status != "not_found" {
			t.Fatalf("VM %s already exists (status: %s) — name collision", env.vmName, info.Status)
		}
		t.Logf("VM name %s is available", env.vmName)
	})

	t.Run("environment_summary", func(t *testing.T) {
		t.Logf("Project:     %s", env.projectID)
		t.Logf("Zone:        %s", env.zone)
		t.Logf("VM Name:     %s", env.vmName)
		t.Logf("SSH User:    %s", env.sshUser)
		t.Logf("Spot:        %v", env.cfg.VM.Spot)
		t.Logf("MaxUptime:   %d min", env.cfg.VM.MaxUptimeMinutes)
		t.Logf("Provisioning: %s", env.cfg.Provisioning.ScriptURL)
		fmt.Println() // visual separator in test output
	})
}
