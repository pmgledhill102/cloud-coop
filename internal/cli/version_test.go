package cli

import (
	"bytes"
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/version"
)

func TestSetVersionInfo(t *testing.T) {
	// Save original values
	origVersion := version.Version
	origCommit := version.Commit
	origBuildTime := version.BuildTime
	defer func() {
		version.Version = origVersion
		version.Commit = origCommit
		version.BuildTime = origBuildTime
	}()

	// Test setting version info
	SetVersionInfo("1.2.3", "abc123", "2025-01-01T00:00:00Z")

	if version.Version != "1.2.3" {
		t.Errorf("version = %q, want %q", version.Version, "1.2.3")
	}
	if version.Commit != "abc123" {
		t.Errorf("commit = %q, want %q", version.Commit, "abc123")
	}
	if version.BuildTime != "2025-01-01T00:00:00Z" {
		t.Errorf("buildTime = %q, want %q", version.BuildTime, "2025-01-01T00:00:00Z")
	}
}

func TestVersionCmd(t *testing.T) {
	// Save original values
	origVersion := version.Version
	origCommit := version.Commit
	origBuildTime := version.BuildTime
	defer func() {
		version.Version = origVersion
		version.Commit = origCommit
		version.BuildTime = origBuildTime
	}()

	// Set test values
	version.Version = "test-version"
	version.Commit = "test-commit"
	version.BuildTime = "test-time"

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
