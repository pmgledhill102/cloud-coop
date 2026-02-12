//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
)

// TestPhase2_VMCreate creates the test VM with spot instances and max_uptime.
func TestPhase2_VMCreate(t *testing.T) {
	if env.provider == nil {
		t.Fatal("provider not initialized — Phase 0 must pass first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Run("create", func(t *testing.T) {
		machineType := "c4a-highcpu-4"
		if mt := env.cfg.VM.MachineSizes["small"]; mt != "" {
			machineType = mt
		}

		createCfg := cloud.VMCreateConfig{
			Name:               env.vmName,
			MachineType:        machineType,
			DiskSizeGB:         env.cfg.VM.DiskSizeGB,
			Image:              env.cfg.VM.Image,
			Spot:               env.cfg.VM.Spot,
			MaxUptimeMinutes:   env.cfg.VM.MaxUptimeMinutes,
			Network:            env.cfg.VM.Network,
			Subnet:             env.cfg.VM.Subnet,
			SSHPort:            env.cfg.SSH.Port,
			SSHUser:            env.sshUser,
			SSHPublicKey:       strings.TrimSpace(env.sshPubKey),
			ServiceAccount:     env.cfg.Cloud.GCP.ServiceAccount,
			ProvisionScriptURL: env.cfg.Provisioning.ScriptURL,
		}

		t.Logf("Creating VM %s (machine: %s, spot: %v, max_uptime: %d min)...",
			createCfg.Name, createCfg.MachineType, createCfg.Spot, createCfg.MaxUptimeMinutes)

		err := env.provider.CreateVM(ctx, createCfg)
		if err != nil {
			t.Fatalf("CreateVM: %v", err)
		}
		t.Logf("VM %s created successfully", env.vmName)
	})

	t.Run("verify_running", func(t *testing.T) {
		env.waitForStatus(t, cloud.VMStatusRunning, 3*time.Minute)
		t.Logf("VM %s is running", env.vmName)
	})

	t.Run("verify_metadata", func(t *testing.T) {
		env.refreshVMInfo(t)

		if env.vmInfo.ExternalIP == "" {
			t.Error("VM has no external IP")
		} else {
			t.Logf("External IP: %s", env.vmInfo.ExternalIP)
		}

		if env.vmInfo.CloudcoopVersion == "" {
			t.Error("VM missing cloudcoop-version metadata")
		} else {
			t.Logf("cloudcoop version: %s", env.vmInfo.CloudcoopVersion)
		}

		if env.vmInfo.CloudcoopCreated == "" {
			t.Error("VM missing cloudcoop-created metadata")
		}

		if env.vmInfo.CloudcoopConfigHash == "" {
			t.Error("VM missing cloudcoop-config-hash metadata")
		}
	})
}
