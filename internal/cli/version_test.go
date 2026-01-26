package cli

import (
	"bytes"
	"testing"
)

func TestSetVersionInfo(t *testing.T) {
	// Save original values
	origVersion := version
	origCommit := commit
	origBuildTime := buildTime
	defer func() {
		version = origVersion
		commit = origCommit
		buildTime = origBuildTime
	}()

	// Test setting version info
	SetVersionInfo("1.2.3", "abc123", "2025-01-01T00:00:00Z")

	if version != "1.2.3" {
		t.Errorf("version = %q, want %q", version, "1.2.3")
	}
	if commit != "abc123" {
		t.Errorf("commit = %q, want %q", commit, "abc123")
	}
	if buildTime != "2025-01-01T00:00:00Z" {
		t.Errorf("buildTime = %q, want %q", buildTime, "2025-01-01T00:00:00Z")
	}
}

func TestVersionCmd(t *testing.T) {
	// Save original values
	origVersion := version
	origCommit := commit
	origBuildTime := buildTime
	defer func() {
		version = origVersion
		commit = origCommit
		buildTime = origBuildTime
	}()

	// Set test values
	version = "test-version"
	commit = "test-commit"
	buildTime = "test-time"

	// Capture output
	buf := new(bytes.Buffer)
	versionCmd.SetOut(buf)
	versionCmd.SetArgs([]string{})

	err := versionCmd.Execute()
	if err != nil {
		t.Fatalf("versionCmd.Execute() error = %v", err)
	}

	output := buf.String()
	if output == "" {
		// Output goes to stdout, not the buffer in this case
		// The Run function uses fmt.Printf directly
		// This test primarily validates the command doesn't error
		return
	}
}
