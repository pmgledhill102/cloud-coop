//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

// TestPhase6_Delete tests VM deletion and verifies cleanup.
func TestPhase6_Delete(t *testing.T) {
	if env.provider == nil {
		t.Fatal("provider not initialized — earlier phases must pass first")
	}

	t.Run("delete", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		t.Logf("Deleting VM %s...", env.vmName)
		err := env.provider.DeleteVM(ctx, env.vmName)
		if err != nil {
			t.Fatalf("DeleteVM: %v", err)
		}
		t.Logf("VM %s deleted", env.vmName)
	})

	t.Run("verify_not_found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		info, err := env.provider.GetVMInfo(ctx, env.vmName)
		if err != nil {
			t.Fatalf("GetVMInfo after delete: %v", err)
		}
		if info.Status != cloud.VMStatusNotFound {
			t.Errorf("Expected not_found after delete, got: %s", info.Status)
		}
	})
}
