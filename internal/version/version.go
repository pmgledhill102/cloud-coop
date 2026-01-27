// Package version provides version information for cloudcoop.
// It is designed to be set at build time via ldflags and accessed
// throughout the application.
package version

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

var (
	// Version is the semantic version (e.g., "v0.1.0").
	Version = "dev"
	// Commit is the git commit hash.
	Commit = "unknown"
	// BuildTime is the build timestamp.
	BuildTime = "unknown"
)

// Set sets the version information from build-time ldflags.
func Set(v, c, bt string) {
	Version = v
	Commit = c
	BuildTime = bt
}

// String returns a formatted version string.
func String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildTime)
}

// Short returns just the version number.
func Short() string {
	return Version
}

// CreatedTimestamp returns the current time in ISO 8601 format for metadata.
func CreatedTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ConfigHash generates a short hash of the config for change detection.
// The input should be a deterministic string representation of the config.
func ConfigHash(configStr string) string {
	h := sha256.Sum256([]byte(configStr))
	return hex.EncodeToString(h[:8]) // First 8 bytes = 16 hex chars
}
