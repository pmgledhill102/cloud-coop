package cli

import (
	"os"
	"testing"
)

func TestResolveSessionName(t *testing.T) {
	t.Run("returns repo slug inside git repo", func(t *testing.T) {
		// This test runs from the repo root (cloud-coop), so Detect should
		// succeed and return the slug derived from the origin remote URL.
		got := resolveSessionName()
		if got == defaultSessionName {
			t.Errorf("resolveSessionName() = %q, expected repo slug, not the default", got)
		}
		if got == "" {
			t.Error("resolveSessionName() returned empty string")
		}
	})

	t.Run("falls back to default outside git repo", func(t *testing.T) {
		// Change to a temp dir that is not a git repo.
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		tmpDir := t.TempDir()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		got := resolveSessionName()
		if got != defaultSessionName {
			t.Errorf("resolveSessionName() = %q, want %q", got, defaultSessionName)
		}
	})
}
