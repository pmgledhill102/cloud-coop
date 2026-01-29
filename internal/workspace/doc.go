// Package workspace detects repository context from the local filesystem.
//
// It derives a repo slug from the git remote URL (used as the tmux session
// name in the multi-agent workflow) and lists local worktrees with their
// branch and commit information.
//
// The slug is the repository name only (last path segment of the remote URL),
// lowercased, matching ADR-0022 and ADR-0023 conventions:
//
//	git@github.com:acme/acme-backend.git → acme-backend
//	https://github.com/acme/frontend.git → frontend
package workspace
