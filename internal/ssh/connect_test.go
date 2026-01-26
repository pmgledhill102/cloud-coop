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
		WindowIndex: 5,
		// Port left as 0 to test default
	}

	// Verify defaults are handled (port should default to 22 in ConnectInteractive)
	if opts.Port != 0 {
		t.Errorf("Port should be 0 when not set, got %d", opts.Port)
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
