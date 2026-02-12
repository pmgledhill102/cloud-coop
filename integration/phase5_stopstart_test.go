//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

// TestPhase5_StopStart tests stopping and restarting the VM.
func TestPhase5_StopStart(t *testing.T) {
	if env.provider == nil || env.vmInfo == nil {
		t.Fatal("VM not created — earlier phases must pass first")
	}

	t.Run("stop", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		t.Logf("Stopping VM %s...", env.vmName)
		err := env.provider.StopVM(ctx, env.vmName)
		if err != nil {
			t.Fatalf("StopVM: %v", err)
		}

		env.waitForStatus(t, cloud.VMStatusStopped, 3*time.Minute)
		t.Logf("VM %s stopped", env.vmName)
	})

	t.Run("verify_stopped", func(t *testing.T) {
		env.refreshVMInfo(t)
		if env.vmInfo.Status != cloud.VMStatusStopped {
			t.Errorf("Expected stopped, got: %s", env.vmInfo.Status)
		}
	})

	t.Run("start", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		t.Logf("Starting VM %s...", env.vmName)
		err := env.provider.StartVM(ctx, env.vmName)
		if err != nil {
			t.Fatalf("StartVM: %v", err)
		}

		env.waitForStatus(t, cloud.VMStatusRunning, 3*time.Minute)
		t.Logf("VM %s running again", env.vmName)
	})

	t.Run("verify_ip_after_restart", func(t *testing.T) {
		env.refreshVMInfo(t)
		if env.vmInfo.ExternalIP == "" {
			t.Error("VM has no external IP after restart")
		} else {
			t.Logf("External IP after restart: %s", env.vmInfo.ExternalIP)
		}
	})
}
