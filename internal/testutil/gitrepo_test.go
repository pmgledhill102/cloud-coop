package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewGitRepo(t *testing.T) {
	r := NewGitRepo(t)

	// Directory should exist and be a git repo
	if _, err := os.Stat(filepath.Join(r.Dir, ".git")); err != nil {
		t.Fatalf("expected .git dir, got: %v", err)
	}

	// Should have origin remote
	out := r.git("remote", "get-url", "origin")
	if out == "" {
		t.Fatal("expected origin remote URL")
	}
}

func TestWithWorktree(t *testing.T) {
	r := NewGitRepo(t)
	wtPath := r.WithWorktree("feature-branch")

	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}
}

func TestWithBareClone(t *testing.T) {
	r := NewGitRepo(t)
	bare := r.WithBareClone()

	// Bare repo should have a HEAD file but no .git dir
	if _, err := os.Stat(filepath.Join(bare.Dir, "HEAD")); err != nil {
		t.Fatalf("bare repo missing HEAD: %v", err)
	}
}
