package version

import (
	"strings"
	"testing"
	"time"
)

func TestSet(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := Commit
	origBuildTime := BuildTime
	defer func() {
		Version = origVersion
		Commit = origCommit
		BuildTime = origBuildTime
	}()

	Set("v1.0.0", "abc123", "2025-01-01T00:00:00Z")

	if Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q", Version, "v1.0.0")
	}
	if Commit != "abc123" {
		t.Errorf("Commit = %q, want %q", Commit, "abc123")
	}
	if BuildTime != "2025-01-01T00:00:00Z" {
		t.Errorf("BuildTime = %q, want %q", BuildTime, "2025-01-01T00:00:00Z")
	}
}

func TestString(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := Commit
	origBuildTime := BuildTime
	defer func() {
		Version = origVersion
		Commit = origCommit
		BuildTime = origBuildTime
	}()

	Set("v1.0.0", "abc123", "2025-01-01T00:00:00Z")

	s := String()
	if !strings.Contains(s, "v1.0.0") {
		t.Errorf("String() = %q, should contain version", s)
	}
	if !strings.Contains(s, "abc123") {
		t.Errorf("String() = %q, should contain commit", s)
	}
}

func TestShort(t *testing.T) {
	// Save original values
	origVersion := Version
	defer func() {
		Version = origVersion
	}()

	Version = "v1.0.0"
	if Short() != "v1.0.0" {
		t.Errorf("Short() = %q, want %q", Short(), "v1.0.0")
	}
}

func TestCreatedTimestamp(t *testing.T) {
	ts := CreatedTimestamp()

	// Should be parseable as RFC3339
	_, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Errorf("CreatedTimestamp() = %q, should be valid RFC3339: %v", ts, err)
	}
}

func TestConfigHash(t *testing.T) {
	tests := []struct {
		name   string
		input1 string
		input2 string
		same   bool
	}{
		{
			name:   "same input same hash",
			input1: "config1",
			input2: "config1",
			same:   true,
		},
		{
			name:   "different input different hash",
			input1: "config1",
			input2: "config2",
			same:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := ConfigHash(tt.input1)
			hash2 := ConfigHash(tt.input2)

			// Hash should be 16 hex characters (8 bytes)
			if len(hash1) != 16 {
				t.Errorf("ConfigHash length = %d, want 16", len(hash1))
			}

			if tt.same && hash1 != hash2 {
				t.Errorf("Expected same hash for same input, got %q != %q", hash1, hash2)
			}
			if !tt.same && hash1 == hash2 {
				t.Errorf("Expected different hash for different input, got %q == %q", hash1, hash2)
			}
		})
	}
}
