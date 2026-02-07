package setup

import (
	"os"
	"path/filepath"
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
