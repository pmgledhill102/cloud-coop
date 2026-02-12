//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	sshpkg "github.com/cloud-coop/cloudcoop/internal/ssh"
)

// TestPhase3a_SSH tests SSH connectivity to the VM.
func TestPhase3a_SSH(t *testing.T) {
	if env.provider == nil || env.vmInfo == nil {
		t.Fatal("VM not created — Phase 2 must pass first")
	}

	t.Run("wait_for_ssh", func(t *testing.T) {
		ip, err := sshpkg.ResolveVMIP(env.vmInfo.ExternalIP, env.vmInfo.InternalIP)
		if err != nil {
			t.Fatalf("resolve VM IP: %v", err)
		}

		cfg := sshpkg.SetupClientConfig(ip, env.sshUser, env.cfg.SSH.Port)
		cfg.VM = sshpkg.NewVMIdentity(env.vmInfo.Name, env.vmInfo.CloudcoopCreated)

		t.Logf("Waiting for SSH on %s:%d...", ip, env.cfg.SSH.Port)
		err = sshpkg.WaitForSSH(cfg, 2*time.Minute)
		if err != nil {
			t.Fatalf("WaitForSSH: %v", err)
		}
		t.Log("SSH is ready")
	})

	t.Run("run_command", func(t *testing.T) {
		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		output, err := client.Run("echo hello-cloudcoop")
		if err != nil {
			t.Fatalf("SSH run: %v", err)
		}

		trimmed := strings.TrimSpace(output)
		if trimmed != "hello-cloudcoop" {
			t.Errorf("SSH output = %q, want %q", trimmed, "hello-cloudcoop")
		}
	})

	t.Run("verify_user", func(t *testing.T) {
		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		output, err := client.Run("whoami")
		if err != nil {
			t.Fatalf("SSH run whoami: %v", err)
		}

		user := strings.TrimSpace(output)
		if user != env.sshUser {
			t.Errorf("SSH user = %q, want %q", user, env.sshUser)
		}
	})

	t.Run("ssh_key_idempotent", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// EnsureSSHKeyOnVM should be idempotent — calling it again
		// with the same key should be a no-op
		err := env.provider.EnsureSSHKeyOnVM(ctx, env.vmName, env.sshUser, strings.TrimSpace(env.sshPubKey))
		if err != nil {
			t.Fatalf("EnsureSSHKeyOnVM: %v", err)
		}

		// Verify we can still connect
		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		output, err := client.Run("echo still-connected")
		if err != nil {
			t.Fatalf("SSH after key re-ensure: %v", err)
		}
		if !strings.Contains(output, "still-connected") {
			t.Error("SSH connection broken after EnsureSSHKeyOnVM")
		}
	})
}

// TestPhase5a_SSHAfterRestart verifies SSH reconnects after stop/start cycle.
func TestPhase5a_SSHAfterRestart(t *testing.T) {
	if env.vmInfo == nil || env.vmInfo.Status != cloud.VMStatusRunning {
		t.Skip("VM not running — Phase 5 stop/start must complete first")
	}

	// Update firewall for potential new IP
	t.Run("ensure_firewall", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := env.provider.EnsureFirewallAllowsSSH(ctx, cloud.FirewallConfig{
			SourceIP: "0.0.0.0", // Allow all for test (will be cleaned up)
			Port:     env.cfg.SSH.Port,
			Network:  env.cfg.VM.Network,
		})
		if err != nil {
			t.Logf("Firewall update: %v (continuing)", err)
		}
	})

	t.Run("ssh_reconnect", func(t *testing.T) {
		ip, err := sshpkg.ResolveVMIP(env.vmInfo.ExternalIP, env.vmInfo.InternalIP)
		if err != nil {
			t.Fatalf("resolve VM IP: %v", err)
		}

		cfg := sshpkg.SetupClientConfig(ip, env.sshUser, env.cfg.SSH.Port)
		cfg.VM = sshpkg.NewVMIdentity(env.vmInfo.Name, env.vmInfo.CloudcoopCreated)

		t.Logf("Waiting for SSH after restart on %s...", ip)
		if err := sshpkg.WaitForSSH(cfg, 2*time.Minute); err != nil {
			t.Fatalf("WaitForSSH after restart: %v", err)
		}

		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		output, err := client.Run("echo reconnected")
		if err != nil {
			t.Fatalf("SSH after restart: %v", err)
		}
		if !strings.Contains(output, "reconnected") {
			t.Error("Failed to verify SSH after restart")
		}
		t.Log("SSH reconnected after restart")
	})
}
