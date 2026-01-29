package workspace

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// Sentinel errors returned by workspace functions.
var (
	ErrNotGitRepo       = errors.New("not a git repository")
	ErrNoRemote         = errors.New("no origin remote configured")
	ErrInvalidRemoteURL = errors.New("invalid remote URL")
)

// Worktree represents a single git worktree on the local filesystem.
type Worktree struct {
	Path   string // Absolute filesystem path
	Branch string // Branch name (empty for detached HEAD)
	Commit string // Full SHA-1 commit hash
	Bare   bool   // True if bare worktree
}

// Info aggregates repository context detected from the local filesystem.
type Info struct {
	RemoteURL string     // Raw git remote URL
	Slug      string     // Derived repo slug
	Worktrees []Worktree // Local worktrees
}

// GitRunner abstracts git command execution for testability.
type GitRunner interface {
	Run(args ...string) (string, error)
}

// execGitRunner implements GitRunner using os/exec.
type execGitRunner struct {
	dir string
}

// NewGitRunner returns a GitRunner that executes git commands in dir.
func NewGitRunner(dir string) GitRunner {
	return &execGitRunner{dir: dir}
}

func (r *execGitRunner) Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// SlugFromURL derives a repository slug from a git remote URL.
//
// It extracts the last path segment (repo name only), strips any .git suffix,
// and lowercases the result.
func SlugFromURL(remoteURL string) (string, error) {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return "", ErrInvalidRemoteURL
	}

	var path string
	switch {
	case strings.HasPrefix(remoteURL, "ssh://"),
		strings.HasPrefix(remoteURL, "https://"),
		strings.HasPrefix(remoteURL, "http://"):
		u, err := url.Parse(remoteURL)
		if err != nil {
			return "", fmt.Errorf("%w: %s", ErrInvalidRemoteURL, err)
		}
		path = u.Path

	case strings.Contains(remoteURL, ":") && !strings.Contains(remoteURL, "://"):
		// SCP-style: git@host:path
		idx := strings.Index(remoteURL, ":")
		path = remoteURL[idx+1:]

	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidRemoteURL, remoteURL)
	}

	// Strip trailing slashes, then .git suffix.
	path = strings.TrimRight(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimRight(path, "/")

	// Extract the last path segment.
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		path = path[idx+1:]
	}

	slug := strings.ToLower(path)
	if slug == "" {
		return "", fmt.Errorf("%w: empty slug from %s", ErrInvalidRemoteURL, remoteURL)
	}
	return slug, nil
}

// RemoteURL returns the origin remote URL for the repository.
func RemoteURL(runner GitRunner) (string, error) {
	out, err := runner.Run("remote", "get-url", "origin")
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "not a git repository") {
			return "", ErrNotGitRepo
		}
		if strings.Contains(errStr, "No such remote") {
			return "", ErrNoRemote
		}
		return "", err
	}
	if out == "" {
		return "", ErrNoRemote
	}
	return out, nil
}

// ListWorktrees returns all worktrees for the repository.
func ListWorktrees(runner GitRunner) ([]Worktree, error) {
	out, err := runner.Run("worktree", "list", "--porcelain")
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "not a git repository") {
			return nil, ErrNotGitRepo
		}
		return nil, err
	}
	return parseWorktrees(out), nil
}

// parseWorktrees parses the porcelain output of git worktree list.
//
// Each worktree block is separated by a blank line and contains lines like:
//
//	worktree /path/to/dir
//	HEAD abc123def456
//	branch refs/heads/main
//
// Special lines: "bare" (bare worktree), "detached" (detached HEAD).
func parseWorktrees(output string) []Worktree {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	var worktrees []Worktree
	blocks := splitBlocks(output)

	for _, block := range blocks {
		var wt Worktree
		hasPath := false

		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			switch {
			case strings.HasPrefix(line, "worktree "):
				wt.Path = strings.TrimPrefix(line, "worktree ")
				hasPath = true
			case strings.HasPrefix(line, "HEAD "):
				wt.Commit = strings.TrimPrefix(line, "HEAD ")
			case strings.HasPrefix(line, "branch "):
				ref := strings.TrimPrefix(line, "branch ")
				wt.Branch = strings.TrimPrefix(ref, "refs/heads/")
			case line == "bare":
				wt.Bare = true
			case line == "detached":
				// Branch stays empty for detached HEAD.
			}
		}

		if hasPath {
			worktrees = append(worktrees, wt)
		}
	}

	return worktrees
}

// splitBlocks splits porcelain output into blocks separated by blank lines.
func splitBlocks(output string) []string {
	var blocks []string
	var current strings.Builder

	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			if current.Len() > 0 {
				blocks = append(blocks, current.String())
				current.Reset()
			}
			continue
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		blocks = append(blocks, current.String())
	}

	return blocks
}

// Detect gathers full repository context: remote URL, slug, and worktrees.
func Detect(runner GitRunner) (*Info, error) {
	remote, err := RemoteURL(runner)
	if err != nil {
		return nil, fmt.Errorf("detecting remote URL: %w", err)
	}

	slug, err := SlugFromURL(remote)
	if err != nil {
		return nil, fmt.Errorf("deriving slug: %w", err)
	}

	worktrees, err := ListWorktrees(runner)
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	return &Info{
		RemoteURL: remote,
		Slug:      slug,
		Worktrees: worktrees,
	}, nil
}
