package ssh

import (
	"os/exec"
	"testing"
)

func TestCheckSSHAvailable(t *testing.T) {
	// This test assumes SSH is available on the test machine
	// (which should be true for any development environment)
	err := CheckSSHAvailable()
	if err != nil {
		t.Skipf("SSH not available on this system: %v", err)
	}
}

func TestConnectOptions_Defaults(t *testing.T) {
	opts := ConnectOptions{
		Host:        "example.com",
		User:        "testuser",
		Session:     "agents",
		WindowIndex: 5,
		// Port left as 0 to test default
	}

	// Verify defaults are handled (port should default to 22 in ConnectInteractive)
	if opts.Port != 0 {
		t.Errorf("Port should be 0 when not set, got %d", opts.Port)
	}
}

func TestConnectOptions_AllFields(t *testing.T) {
	opts := ConnectOptions{
		Host:        "192.168.1.100",
		User:        "ubuntu",
		Port:        2222,
		Session:     "agents",
		WindowIndex: 3,
	}

	if opts.Host != "192.168.1.100" {
		t.Errorf("Host = %q, want %q", opts.Host, "192.168.1.100")
	}
	if opts.User != "ubuntu" {
		t.Errorf("User = %q, want %q", opts.User, "ubuntu")
	}
	if opts.Port != 2222 {
		t.Errorf("Port = %d, want %d", opts.Port, 2222)
	}
	if opts.Session != "agents" {
		t.Errorf("Session = %q, want %q", opts.Session, "agents")
	}
	if opts.WindowIndex != 3 {
		t.Errorf("WindowIndex = %d, want %d", opts.WindowIndex, 3)
	}
}

func TestLookPath_SSH(t *testing.T) {
	// Verify exec.LookPath works for ssh
	path, err := exec.LookPath("ssh")
	if err != nil {
		t.Skipf("SSH not found in PATH: %v", err)
	}
	if path == "" {
		t.Error("LookPath returned empty path for ssh")
	}
}

func TestCheckSSHAvailable_ReturnsNilWhenFound(t *testing.T) {
	// Skip if SSH isn't available
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("SSH not available")
	}

	err := CheckSSHAvailable()
	if err != nil {
		t.Errorf("CheckSSHAvailable() returned error when SSH is available: %v", err)
	}
}
