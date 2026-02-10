package ssh

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFormatHostPort(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{"standard port 22", "192.168.1.1", 22, "192.168.1.1"},
		{"port 0 (default)", "192.168.1.1", 0, "192.168.1.1"},
		{"custom port", "192.168.1.1", 2222, "192.168.1.1:2222"},
		{"hostname with custom port", "example.com", 8022, "example.com:8022"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHostPort(tt.host, tt.port)
			if got != tt.want {
				t.Errorf("formatHostPort(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}

func TestFormatKnownHostsEntry(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{"standard port 22", "192.168.1.1", 22, "192.168.1.1"},
		{"port 0 (default)", "192.168.1.1", 0, "192.168.1.1"},
		{"custom port", "192.168.1.1", 2222, "[192.168.1.1]:2222"},
		{"hostname with custom port", "example.com", 8022, "[example.com]:8022"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatKnownHostsEntry(tt.host, tt.port)
			if got != tt.want {
				t.Errorf("formatKnownHostsEntry(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}

func TestIsHostKeyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "key is unknown error",
			err:  errors.New("ssh: handshake failed: knownhosts: key is unknown"),
			want: true,
		},
		{
			name: "key mismatch error",
			err:  errors.New("ssh: handshake failed: knownhosts: key mismatch"),
			want: true,
		},
		{
			name: "host key verification failed",
			err:  errors.New("host key verification failed for 192.168.1.1: something"),
			want: true,
		},
		{
			name: "unrelated error",
			err:  errors.New("connection refused"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHostKeyError(tt.err)
			if got != tt.want {
				t.Errorf("IsHostKeyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// setTestHome redirects $HOME to a temp dir so pin/known_hosts files are isolated.
func setTestHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	return tmp
}

func TestNewVMIdentity(t *testing.T) {
	tests := []struct {
		name    string
		vmName  string
		created string
		wantNil bool
	}{
		{"both set", "my-vm", "2025-01-15T10:30:00Z", false},
		{"empty name", "", "2025-01-15T10:30:00Z", true},
		{"empty created", "my-vm", "", true},
		{"both empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewVMIdentity(tt.vmName, tt.created)
			if (got == nil) != tt.wantNil {
				t.Errorf("NewVMIdentity(%q, %q) nil=%v, want nil=%v", tt.vmName, tt.created, got == nil, tt.wantNil)
			}
			if got != nil {
				if got.Name != tt.vmName || got.Created != tt.created {
					t.Errorf("fields mismatch: got {%q, %q}", got.Name, got.Created)
				}
			}
		})
	}
}

func TestPinnedKeysRoundTrip(t *testing.T) {
	setTestHome(t)

	store := pinnedKeysStore{
		"vm-a": {Host: "1.2.3.4", Port: 22, Created: "2025-01-01T00:00:00Z"},
		"vm-b": {Host: "5.6.7.8", Port: 2222, Created: "2025-06-15T12:00:00Z"},
	}

	if err := savePinnedKeys(store); err != nil {
		t.Fatalf("savePinnedKeys: %v", err)
	}

	loaded, err := loadPinnedKeys()
	if err != nil {
		t.Fatalf("loadPinnedKeys: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded))
	}
	for name, want := range store {
		got, ok := loaded[name]
		if !ok {
			t.Errorf("missing entry %q", name)
			continue
		}
		if got != want {
			t.Errorf("entry %q: got %+v, want %+v", name, got, want)
		}
	}
}

func TestLoadPinnedKeys_MissingFile(t *testing.T) {
	setTestHome(t)

	store, err := loadPinnedKeys()
	if err != nil {
		t.Fatalf("loadPinnedKeys on missing file: %v", err)
	}
	if len(store) != 0 {
		t.Errorf("expected empty store, got %d entries", len(store))
	}
}

func TestLoadPinnedKeys_CorruptFile(t *testing.T) {
	home := setTestHome(t)

	dir := filepath.Join(home, ".config", "cloudcoop")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pinned_keys.toml"), []byte("{{invalid toml"), 0600); err != nil {
		t.Fatal(err)
	}

	store, err := loadPinnedKeys()
	if err != nil {
		t.Fatalf("loadPinnedKeys on corrupt file: %v", err)
	}
	if len(store) != 0 {
		t.Errorf("expected empty store on corrupt file, got %d entries", len(store))
	}
}

func TestHostKeyExists(t *testing.T) {
	home := setTestHome(t)

	dir := filepath.Join(home, ".config", "cloudcoop")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	khPath := filepath.Join(dir, "known_hosts")

	// Write a known_hosts entry for 1.2.3.4 (standard port)
	content := "1.2.3.4 ssh-ed25519 AAAA-test-key-1\n"
	if err := os.WriteFile(khPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	if !hostKeyExists("1.2.3.4", 22) {
		t.Error("expected hostKeyExists to return true for 1.2.3.4:22")
	}
	if hostKeyExists("5.6.7.8", 22) {
		t.Error("expected hostKeyExists to return false for 5.6.7.8:22")
	}

	// Non-standard port entry
	content += "[10.0.0.1]:2222 ssh-ed25519 AAAA-test-key-2\n"
	if err := os.WriteFile(khPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	if !hostKeyExists("10.0.0.1", 2222) {
		t.Error("expected hostKeyExists to return true for 10.0.0.1:2222")
	}
	if hostKeyExists("10.0.0.1", 22) {
		t.Error("expected hostKeyExists to return false for 10.0.0.1:22")
	}
}

func TestClearPinnedKey(t *testing.T) {
	home := setTestHome(t)

	// Set up known_hosts with an entry
	dir := filepath.Join(home, ".config", "cloudcoop")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	khPath := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(khPath, []byte("1.2.3.4 ssh-ed25519 AAAA-test-key-3\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Save a pin
	store := pinnedKeysStore{
		"my-vm": {Host: "1.2.3.4", Port: 22, Created: "2025-01-01T00:00:00Z"},
	}
	if err := savePinnedKeys(store); err != nil {
		t.Fatal(err)
	}

	// Clear
	if err := ClearPinnedKey("my-vm"); err != nil {
		t.Fatalf("ClearPinnedKey: %v", err)
	}

	// Pin should be gone
	loaded, err := loadPinnedKeys()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded["my-vm"]; ok {
		t.Error("pin for my-vm should have been removed")
	}

	// known_hosts entry should be gone
	if hostKeyExists("1.2.3.4", 22) {
		t.Error("known_hosts entry for 1.2.3.4 should have been removed")
	}
}

func TestFilterKeyscanErrors(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   string
	}{
		{"empty", "", ""},
		{"only comments", "# 1.2.3.4:22 SSH-2.0-OpenSSH_9.6\n# 1.2.3.4:22 banner\n", ""},
		{"error line", "ssh_exchange_identification: Connection refused\n", "ssh_exchange_identification: Connection refused"},
		{"mixed", "# 1.2.3.4:22 SSH-2.0-OpenSSH_9.6\nread_passphrase: can't open /dev/tty\n", "read_passphrase: can't open /dev/tty"},
		{"multiple errors", "error one\nerror two\n", "error one; error two"},
		{"whitespace only", "  \n  \n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterKeyscanErrors(tt.stderr)
			if got != tt.want {
				t.Errorf("filterKeyscanErrors(%q) = %q, want %q", tt.stderr, got, tt.want)
			}
		})
	}
}

func TestClearPinnedKey_Nonexistent(t *testing.T) {
	setTestHome(t)

	// Should not error even with no pin file.
	if err := ClearPinnedKey("nonexistent-vm"); err != nil {
		t.Fatalf("ClearPinnedKey for nonexistent VM: %v", err)
	}
}
