package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// GitRepo is a temporary git repository for integration tests.
type GitRepo struct {
	Dir string
	t   testing.TB
}

// NewGitRepo creates a temporary git repository with an initial commit and
// a dummy origin remote. The repository is cleaned up automatically when
// the test finishes.
func NewGitRepo(t testing.TB) *GitRepo {
	t.Helper()
	dir := t.TempDir()
	r := &GitRepo{Dir: dir, t: t}

	r.git("init")
	r.git("config", "user.email", "test@test.com")
	r.git("config", "user.name", "Test")

	// Create initial commit
	dummy := filepath.Join(dir, "README.md")
	if err := os.WriteFile(dummy, []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.git("add", "README.md")
	r.git("commit", "-m", "initial commit")

	// Set a dummy remote
	r.git("remote", "add", "origin", "git@github.com:acme/test-repo.git")

	return r
}

// WithRemote configures a named remote URL.
func (r *GitRepo) WithRemote(name, url string) *GitRepo {
	r.t.Helper()
	// Remove first in case it already exists
	_, _ = r.gitErr("remote", "remove", name)
	r.git("remote", "add", name, url)
	return r
}

// WithWorktree creates a linked worktree at a new branch.
// Returns the worktree path.
func (r *GitRepo) WithWorktree(branch string) string {
	r.t.Helper()
	wtDir := filepath.Join(r.t.TempDir(), branch)
	r.git("worktree", "add", wtDir, "-b", branch)
	return wtDir
}

// WithBareClone creates a bare clone of this repo and returns a new GitRepo
// pointing to the bare clone.
func (r *GitRepo) WithBareClone() *GitRepo {
	r.t.Helper()
	bareDir := filepath.Join(r.t.TempDir(), "bare.git")
	cmd := exec.Command("git", "clone", "--bare", r.Dir, bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git clone --bare: %v\n%s", err, out)
	}
	return &GitRepo{Dir: bareDir, t: r.t}
}

func (r *GitRepo) git(args ...string) string {
	r.t.Helper()
	out, err := r.gitErr(args...)
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return out
}

func (r *GitRepo) gitErr(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
