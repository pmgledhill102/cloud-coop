package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

// generateTestKey creates a PEM-encoded ed25519 private key for testing.
func generateTestKey(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	pemBlock, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	return pem.EncodeToMemory(pemBlock)
}

func TestDiscoverAuthMethods_KeyFiles(t *testing.T) {
	// Create a temp home directory with .ssh
	tmpHome := t.TempDir()
	sshDir := filepath.Join(tmpHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("create .ssh dir: %v", err)
	}

	// Unset SSH_AUTH_SOCK so we only test key file discovery
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", tmpHome)

	keyData := generateTestKey(t)

	tests := []struct {
		name      string
		files     []string
		wantCount int
	}{
		{
			name:      "discovers standard ed25519 key",
			files:     []string{"id_ed25519"},
			wantCount: 1,
		},
		{
			name:      "discovers standard rsa key",
			files:     []string{"id_rsa"},
			wantCount: 1,
		},
		{
			name:      "discovers google_compute_engine key",
			files:     []string{"google_compute_engine"},
			wantCount: 1,
		},
		{
			name:      "discovers multiple keys",
			files:     []string{"id_ed25519", "id_ecdsa", "google_compute_engine"},
			wantCount: 3,
		},
		{
			name:      "no keys present returns empty",
			files:     nil,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean out .ssh directory between subtests
			entries, _ := os.ReadDir(sshDir)
			for _, e := range entries {
				os.Remove(filepath.Join(sshDir, e.Name()))
			}

			// Write requested key files
			for _, name := range tt.files {
				path := filepath.Join(sshDir, name)
				if err := os.WriteFile(path, keyData, 0600); err != nil {
					t.Fatalf("write key file %s: %v", name, err)
				}
			}

			methods := discoverAuthMethods()
			if len(methods) != tt.wantCount {
				t.Errorf("discoverAuthMethods() returned %d methods, want %d", len(methods), tt.wantCount)
			}
		})
	}
}

func TestDiscoverAuthMethods_SkipsEmptyAgent(t *testing.T) {
	tmpHome := t.TempDir()
	sshDir := filepath.Join(tmpHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("create .ssh dir: %v", err)
	}

	// Point HOME to temp dir and clear SSH_AUTH_SOCK
	t.Setenv("HOME", tmpHome)
	t.Setenv("SSH_AUTH_SOCK", "")

	// Write a valid key file
	keyData := generateTestKey(t)
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519"), keyData, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	// With no agent, we should get exactly 1 method (the key file)
	methods := discoverAuthMethods()
	if len(methods) != 1 {
		t.Errorf("discoverAuthMethods() with no agent returned %d methods, want 1", len(methods))
	}
}

func TestDiscoverAuthMethods_IgnoresInvalidKeys(t *testing.T) {
	tmpHome := t.TempDir()
	sshDir := filepath.Join(tmpHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("create .ssh dir: %v", err)
	}

	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", tmpHome)

	// Write a file with invalid key data
	path := filepath.Join(sshDir, "id_ed25519")
	if err := os.WriteFile(path, []byte("not a valid key"), 0600); err != nil {
		t.Fatalf("write invalid key: %v", err)
	}

	methods := discoverAuthMethods()
	if len(methods) != 0 {
		t.Errorf("discoverAuthMethods() returned %d methods for invalid key, want 0", len(methods))
	}
}
