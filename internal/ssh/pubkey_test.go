// cspell:disable -- test file contains fake SSH key material
package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadPublicKeyFrom_Ed25519(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	want := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest user@host"
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPublicKeyFrom(dir)
	if err != nil {
		t.Fatalf("ReadPublicKeyFrom() error = %v", err)
	}
	if got != want {
		t.Errorf("ReadPublicKeyFrom() = %q, want %q", got, want)
	}
}

func TestReadPublicKeyFrom_RSA(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	want := "ssh-rsa AAAAB3NzaC1yc2EAAAATest user@host"
	if err := os.WriteFile(filepath.Join(sshDir, "id_rsa.pub"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPublicKeyFrom(dir)
	if err != nil {
		t.Fatalf("ReadPublicKeyFrom() error = %v", err)
	}
	if got != want {
		t.Errorf("ReadPublicKeyFrom() = %q, want %q", got, want)
	}
}

func TestReadPublicKeyFrom_ECDSA(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	want := "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoTest user@host"
	if err := os.WriteFile(filepath.Join(sshDir, "id_ecdsa.pub"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPublicKeyFrom(dir)
	if err != nil {
		t.Fatalf("ReadPublicKeyFrom() error = %v", err)
	}
	if got != want {
		t.Errorf("ReadPublicKeyFrom() = %q, want %q", got, want)
	}
}

func TestReadPublicKeyFrom_PrefersEd25519(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	ed25519Key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest user@host"
	rsaKey := "ssh-rsa AAAAB3NzaC1yc2EAAAATest user@host"

	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"), []byte(ed25519Key), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_rsa.pub"), []byte(rsaKey), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPublicKeyFrom(dir)
	if err != nil {
		t.Fatalf("ReadPublicKeyFrom() error = %v", err)
	}
	if got != ed25519Key {
		t.Errorf("ReadPublicKeyFrom() = %q, want ed25519 key %q", got, ed25519Key)
	}
}

func TestReadPublicKeyFrom_NoKeys(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := ReadPublicKeyFrom(dir)
	if err == nil {
		t.Error("ReadPublicKeyFrom() expected error when no keys exist")
	}
	if !strings.Contains(err.Error(), "no SSH public key found") {
		t.Errorf("error = %v, want containing 'no SSH public key found'", err)
	}
}

func TestReadPublicKeyFrom_NoSSHDir(t *testing.T) {
	dir := t.TempDir()

	_, err := ReadPublicKeyFrom(dir)
	if err == nil {
		t.Error("ReadPublicKeyFrom() expected error when .ssh dir doesn't exist")
	}
}

func TestReadPublicKeyFrom_EmptyKeyFile(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// Empty ed25519 file, valid RSA file
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"), []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rsaKey := "ssh-rsa AAAAB3NzaC1yc2EAAAATest user@host"
	if err := os.WriteFile(filepath.Join(sshDir, "id_rsa.pub"), []byte(rsaKey), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPublicKeyFrom(dir)
	if err != nil {
		t.Fatalf("ReadPublicKeyFrom() error = %v", err)
	}
	if got != rsaKey {
		t.Errorf("ReadPublicKeyFrom() = %q, want %q (should skip empty file)", got, rsaKey)
	}
}
