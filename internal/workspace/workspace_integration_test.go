//go:build integration

package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/testutil"
)

func TestMain(m *testing.M) {
	// Skip all integration tests if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		return
	}
	os.Exit(m.Run())
}

func TestExecGitRunner_Run(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	runner := NewGitRunner(repo.Dir)

	// Should capture stdout correctly
	out, err := runner.Run("remote", "get-url", "origin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "github.com") {
		t.Errorf("expected github.com in output, got: %q", out)
	}
}

func TestExecGitRunner_RunError(t *testing.T) {
	// Not a git repo — should fail
	dir := t.TempDir()
	runner := NewGitRunner(dir)

	_, err := runner.Run("remote", "get-url", "origin")
	if err == nil {
		t.Fatal("expected error for non-git dir")
	}
}

func TestRemoteURL_Integration(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	runner := NewGitRunner(repo.Dir)

	url, err := RemoteURL(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "git@github.com:acme/test-repo.git" {
		t.Errorf("RemoteURL() = %q, want %q", url, "git@github.com:acme/test-repo.git")
	}
}

func TestRemoteURL_NoRemote(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	// Remove the origin remote
	cmd := exec.Command("git", "remote", "remove", "origin")
	cmd.Dir = repo.Dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	runner := NewGitRunner(repo.Dir)
	_, err := RemoteURL(runner)
	if err != ErrNoRemote {
		t.Errorf("expected ErrNoRemote, got: %v", err)
	}
}

func TestRemoteURL_NotGitRepo(t *testing.T) {
	runner := NewGitRunner(t.TempDir())
	_, err := RemoteURL(runner)
	if err != ErrNotGitRepo {
		t.Errorf("expected ErrNotGitRepo, got: %v", err)
	}
}

func TestListWorktrees_SingleWorktree(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	runner := NewGitRunner(repo.Dir)

	wts, err := ListWorktrees(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	// Resolve symlinks to handle macOS /var -> /private/var
	wantPath, _ := filepath.EvalSymlinks(repo.Dir)
	gotPath, _ := filepath.EvalSymlinks(wts[0].Path)
	if gotPath != wantPath {
		t.Errorf("worktree path = %q, want %q", gotPath, wantPath)
	}
	if wts[0].Branch != "main" && wts[0].Branch != "master" {
		t.Errorf("worktree branch = %q, want main or master", wts[0].Branch)
	}
	if wts[0].Commit == "" {
		t.Error("expected non-empty commit hash")
	}
}

func TestListWorktrees_MultipleWorktrees(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WithWorktree("feature-a")
	repo.WithWorktree("feature-b")
	runner := NewGitRunner(repo.Dir)

	wts, err := ListWorktrees(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(wts))
	}

	branches := map[string]bool{}
	for _, wt := range wts {
		if wt.Branch != "" {
			branches[wt.Branch] = true
		}
	}
	if !branches["feature-a"] {
		t.Error("missing feature-a worktree")
	}
	if !branches["feature-b"] {
		t.Error("missing feature-b worktree")
	}
}

func TestListWorktrees_BareRepo(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	bare := repo.WithBareClone()
	runner := NewGitRunner(bare.Dir)

	wts, err := ListWorktrees(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	if !wts[0].Bare {
		t.Error("expected bare worktree")
	}
}

func TestDetect_Integration(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WithWorktree("dev")
	runner := NewGitRunner(repo.Dir)

	info, err := Detect(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Slug != "test-repo" {
		t.Errorf("Slug = %q, want %q", info.Slug, "test-repo")
	}
	if info.RemoteURL != "git@github.com:acme/test-repo.git" {
		t.Errorf("RemoteURL = %q", info.RemoteURL)
	}
	if len(info.Worktrees) != 2 {
		t.Errorf("expected 2 worktrees, got %d", len(info.Worktrees))
	}
}

func TestDetect_NotGitRepo(t *testing.T) {
	runner := NewGitRunner(t.TempDir())
	_, err := Detect(runner)
	if err == nil {
		t.Fatal("expected error for non-git dir")
	}
}
