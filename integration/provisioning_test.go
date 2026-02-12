//go:build integration

package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/cloud-coop/cloudcoop/internal/provisioning"
)

// TestPhase3b_Provisioning waits for VM provisioning to complete and checks status.
func TestPhase3b_Provisioning(t *testing.T) {
	if env.vmInfo == nil {
		t.Fatal("VM not created — Phase 2 must pass first")
	}

	t.Run("wait_for_completion", func(t *testing.T) {
		deadline := time.Now().Add(15 * time.Minute)
		var lastProgress string

		for time.Now().Before(deadline) {
			client := env.connectSSH(t)
			status, err := provisioning.CheckStatus(client)
			_ = client.Close()

			if err != nil {
				t.Logf("Provisioning status check error: %v", err)
				time.Sleep(15 * time.Second)
				continue
			}

			// Log progress changes
			if status.Progress != lastProgress {
				t.Logf("Provisioning: [%s] %s", status.Status, status.Progress)
				lastProgress = status.Progress
			}

			switch status.Status {
			case provisioning.StatusCompleted:
				t.Log("Provisioning completed successfully")
				return
			case provisioning.StatusFailed:
				t.Fatalf("Provisioning failed: %s", status.Error)
			case provisioning.StatusRunning, provisioning.StatusPending:
				// Still in progress, wait and retry
			default:
				t.Logf("Unknown provisioning status: %s", status.Status)
			}

			time.Sleep(15 * time.Second)
		}

		t.Fatal("Provisioning did not complete within 15 minutes")
	})

	t.Run("verify_tools", func(t *testing.T) {
		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		// Verify key tools were installed by the provisioning script
		tools := []struct {
			name string
			cmd  string
		}{
			{"tmux", "tmux -V"},
			{"git", "git --version"},
			{"node", "node --version"},
		}

		for _, tool := range tools {
			t.Run(tool.name, func(t *testing.T) {
				output, err := client.Run(tool.cmd)
				if err != nil {
					t.Errorf("%s not available: %v", tool.name, err)
				} else {
					t.Logf("%s: %s", tool.name, strings.TrimSpace(output))
				}
			})
		}
	})

	t.Run("check_provision_log", func(t *testing.T) {
		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		// Read the last few lines of the provision log
		output, err := client.Run("tail -20 /var/log/cloudcoop-provision.log 2>/dev/null || echo 'no log'")
		if err != nil {
			t.Logf("Read provision log: %v", err)
		} else {
			t.Logf("Provision log (last 20 lines):\n%s", output)
		}
	})
}
