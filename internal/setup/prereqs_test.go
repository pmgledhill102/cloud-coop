package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckSSHKeyAt_Found(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// Create a fake SSH key
	pubKey := filepath.Join(sshDir, "id_ed25519.pub")
	if err := os.WriteFile(pubKey, []byte("ssh-ed25519 AAAA... user@host"), 0o644); err != nil {
		t.Fatal(err)
	}

	status := CheckSSHKeyAt(dir)
	if !status.OK {
		t.Errorf("expected SSH key found, got: %s", status.Detail)
	}
	if status.Detail != pubKey {
		t.Errorf("detail = %q, want %q", status.Detail, pubKey)
	}
}

func TestCheckSSHKeyAt_RSA(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	pubKey := filepath.Join(sshDir, "id_rsa.pub")
	if err := os.WriteFile(pubKey, []byte("ssh-rsa AAAA... user@host"), 0o644); err != nil {
		t.Fatal(err)
	}

	status := CheckSSHKeyAt(dir)
	if !status.OK {
		t.Errorf("expected SSH key found, got: %s", status.Detail)
	}
}

func TestCheckSSHKeyAt_NotFound(t *testing.T) {
	dir := t.TempDir()

	status := CheckSSHKeyAt(dir)
	if status.OK {
		t.Error("expected no SSH key found")
	}
	if status.HelpMsg == "" {
		t.Error("expected help message for missing SSH key")
	}
}

func TestGenerateSSHKeyAt(t *testing.T) {
	dir := t.TempDir()

	pubPath, err := GenerateSSHKeyAt(dir)
	if err != nil {
		t.Fatalf("GenerateSSHKeyAt() error = %v", err)
	}

	// Verify public key exists and has correct format
	pubData, err := os.ReadFile(pubPath)
	if err != nil {
		t.Fatalf("read public key: %v", err)
	}
	if !strings.HasPrefix(string(pubData), "ssh-ed25519 ") {
		t.Errorf("public key should start with 'ssh-ed25519 ', got: %s", string(pubData)[:20])
	}

	// Verify both keys have 0600 permissions
	for _, name := range []string{"id_ed25519", "id_ed25519.pub"} {
		keyPath := filepath.Join(dir, ".ssh", name)
		info, statErr := os.Stat(keyPath)
		if statErr != nil {
			t.Fatalf("stat %s: %v", name, statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s permissions = %o, want 0600", name, info.Mode().Perm())
		}
	}

	// Verify CheckSSHKeyAt now finds the key
	status := CheckSSHKeyAt(dir)
	if !status.OK {
		t.Errorf("expected SSH key found after generation, got: %s", status.Detail)
	}
}

func TestGenerateSSHKeyAt_SshDirCreated(t *testing.T) {
	dir := t.TempDir()

	// .ssh doesn't exist yet
	sshDir := filepath.Join(dir, ".ssh")
	if _, err := os.Stat(sshDir); !os.IsNotExist(err) {
		t.Fatal("expected .ssh to not exist initially")
	}

	_, err := GenerateSSHKeyAt(dir)
	if err != nil {
		t.Fatalf("GenerateSSHKeyAt() error = %v", err)
	}

	// .ssh should now exist with correct permissions
	info, err := os.Stat(sshDir)
	if err != nil {
		t.Fatalf("stat .ssh dir: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf(".ssh dir permissions = %o, want 0700", info.Mode().Perm())
	}
}
